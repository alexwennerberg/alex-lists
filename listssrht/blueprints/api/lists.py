from flask import current_app, Blueprint, abort, request
from listssrht.blueprints.api import get_user, get_list
from listssrht.blueprints.archives import apply_search
from listssrht.types import List, Email, ListAccess, Subscription
from listssrht.webhooks import ListWebhook, UserWebhook
from sqlalchemy import or_
from srht.api import paginated_response
from srht.database import db
from srht.graphql import exec_gql
from srht.oauth import oauth, current_token
from srht.validation import Validation

lists = Blueprint("api_lists", __name__)

@lists.route("/api/user/<username>/lists")
@lists.route("/api/lists", defaults={"username": None})
@oauth("lists:read")
def user_lists_GET(username):
    user = get_user(username)
    lists = List.query.filter(List.owner_id == user.id)
    if current_token.user_id != user.id:
        lists = lists.filter(or_(
            List.account_permissions > 0,
            List.nonsubscriber_permissions > 0))
    return paginated_response(List.id, lists)

@lists.route("/api/lists", methods=["POST"])
@oauth("lists:write")
def user_lists_POST():
    user = current_token.user
    valid = Validation(request)
    name = valid.require("name", friendly_name="Name")
    description = valid.optional("description")
    if not valid.ok:
        return valid.response

    resp = exec_gql(current_app.site, """
        mutation CreateMailingList($name: String!, $description: String) {
            createMailingList(name: $name, description: $description) {
                id
                name
                owner {
                    canonical_name: canonicalName
                    ... on User {
                        name: username
                    }
                }
                created
                updated
                description
                nonsubscriber {
                    browse
                    reply
                    post
                }
                subscriber {
                    browse
                    reply
                    post
                }
                identified {
                    browse
                    reply
                    post
                }
            }
        }
    """, user=user, valid=valid, name=name, description=description)

    if not valid.ok:
        return valid.response

    resp = resp["createMailingList"]

    permList = lambda acl: [key for key in [
        "browse", "reply", "post"
    ] if acl[key]]

    resp["permissions"] = {
        "nonsubscriber": permList(resp["nonsubscriber"]),
        "subscriber": permList(resp["subscriber"]),
        "account": permList(resp["identified"]),
    }
    del resp["nonsubscriber"]
    del resp["subscriber"]
    del resp["identified"]

    return resp, 201

@lists.route("/api/user/<username>/lists/<list_name>")
@lists.route("/api/lists/<list_name>", defaults={"username": None})
@oauth("lists:read")
def user_lists_by_name_GET(username, list_name):
    user, ml, access = get_list(username, list_name)
    if not ListAccess.browse in access:
        abort(404)
    return ml.to_dict()

def _webhook_filters(query, username, list_name):
    owner, ml, access = get_list(username, list_name)
    return query.filter(ListWebhook.Subscription.list_id == ml.id)

def _webhook_create(sub, valid, username, list_name):
    owner, ml, access = get_list(username, list_name)
    if not ml:
        abort(404)
    valid.expect(ListAccess.browse in access,
            "You are not authorized to subscribe to this list.",
            field="list", status=403)
    sub.list_id = ml.id
    return sub

ListWebhook.api_routes(lists, "/api/user/<username>/lists/<list_name>",
        filters=_webhook_filters, create=_webhook_create)

@lists.route("/api/lists/<list_name>", methods=["PUT"])
@oauth("lists:write")
def user_lists_by_name_PUT(list_name):
    user, ml, access = get_list(None, list_name)
    if ml.owner_id != user.id:
        abort(403)

    valid = Validation(request)
    rewrite = lambda value: None if value == "" else value
    input = {
        key: rewrite(valid.source[key]) for key in [
            "description"
        ] if valid.source.get(key) is not None
    }

    resp = exec_gql(current_app.site, """
        mutation UpdateMailingList($id: Int!, $input: MailingListInput!) {
            updateMailingList(id: $id, input: $input) {
                id
                name
                owner {
                    canonical_name: canonicalName
                    ... on User {
                        name: username
                    }
                }
                created
                updated
                description
                nonsubscriber {
                    browse
                    reply
                    post
                }
                subscriber {
                    browse
                    reply
                    post
                }
                identified {
                    browse
                    reply
                    post
                }
            }
        }
    """, user=user, valid=valid, id=ml.id, input=input)

    if not valid.ok:
        return valid.response

    resp = resp["updateMailingList"]

    permList = lambda acl: [key for key in [
        "browse", "reply", "post"
    ] if acl[key]]

    resp["permissions"] = {
        "nonsubscriber": permList(resp["nonsubscriber"]),
        "subscriber": permList(resp["subscriber"]),
        "account": permList(resp["identified"]),
    }
    del resp["nonsubscriber"]
    del resp["subscriber"]
    del resp["identified"]

    return resp

@lists.route("/api/lists/<list_name>", methods=["DELETE"])
@oauth("lists:write")
def user_lists_by_name_DELETE(list_name):
    user, ml, access = get_list(None, list_name)
    if ml.owner_id != user.id:
        abort(403)
    exec_gql(current_app.site, """
        mutation DeleteMailingList($id: Int!) {
            deleteMailingList(id: $id) { id }
        }
    """, user=user, id=ml.id)
    return {}, 204

@lists.route("/api/user/<username>/lists/<list_name>/posts")
@lists.route("/api/lists/<list_name>/posts", defaults={"username": None})
@oauth("lists:read")
def user_lists_by_name_posts_GET(username, list_name):
    user, ml, access = get_list(username, list_name)
    if not ListAccess.browse in access:
        abort(404)

    search = request.args.get("search")
    emails = Email.query.filter(Email.list_id == ml.id)
    try:
        emails = apply_search(emails, search)
    except ValueError as ex:
        return {"errors": [{"reason": str(ex)}]}, 400

    return paginated_response(Email.id, emails, short=True)
