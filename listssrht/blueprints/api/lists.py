from flask import Blueprint, abort, request
from listssrht.blueprints.api import get_user, get_list
from listssrht.types import User, List, Email, ListAccess
from listssrht.webhooks import ListWebhook, UserWebhook
from sqlalchemy import or_
from srht.api import paginated_response
from srht.database import db
from srht.oauth import oauth, current_token
from srht.validation import Validation

lists = Blueprint("api.lists", __name__)

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
    ml = List(user, valid)
    if not valid.ok:
        return valid.response
    db.session.add(ml)
    db.session.flush()
    UserWebhook.deliver(UserWebhook.Events.list_create,
            ml.to_dict(), UserWebhook.Subscription.user_id == ml.owner_id)
    db.session.commit()
    return ml.to_dict(), 201

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
    ml.update(valid)
    ListWebhook.deliver(ListWebhook.Events.list_update,
            ml.to_dict(), ListWebhook.Subscription.list_id == ml.id)
    db.session.commit()
    return ml.to_dict()

@lists.route("/api/user/<username>/lists/<list_name>/posts")
@lists.route("/api/lists/<list_name>/posts", defaults={"username": None})
@oauth("lists:read")
def user_lists_by_name_posts_GET(username, list_name):
    user, ml, access = get_list(username, list_name)
    if not ListAccess.browse in access:
        abort(404)
    emails = Email.query.filter(Email.list_id == ml.id)
    return paginated_response(Email.id, emails, short=True)
