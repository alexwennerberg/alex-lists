from flask import Blueprint, abort, request
from listssrht.blueprints.api import get_list
from listssrht.types import Subscription, ListAccess
from srht.api import paginated_response
from srht.database import db
from srht.oauth import oauth, current_token
from srht.validation import Validation

subs = Blueprint("api.subs", __name__)

@subs.route("/api/subscriptions")
@oauth("subs:read")
def subscriptions_GET():
    user = current_token.user
    subs = Subscription.query.filter(Subscription.user_id == user.id)
    return paginated_response(Subscription.id, subs)

@subs.route("/api/subscriptions", methods=["POST"])
@oauth("subs:write")
def subscriptions_POST():
    user = current_token.user
    valid = Validation(request)
    list_name = valid.require("list")
    valid.expect(not list_name or "/" in list_name,
            "Invalid list name specified - expected owner/list-name",
            field="list")
    if not valid.ok:
        return valid.response
    owner, list_name = list_name.split("/")
    owner, ml, access = get_list(owner, list_name)
    valid.expect(ListAccess.browse in access,
            "You are not authorized to subscribe to this list.",
            field="list", status=403)
    sub = (Subscription.query
            .filter(Subscription.list_id == ml.id)
            .filter(Subscription.user_id == user.id)).one_or_none()
    valid.expect(sub is None,
            "You are already subscribed to this list.",
            field="list")
    if not valid.ok:
        return valid.response
    sub = Subscription()
    sub.user_id = user.id
    sub.list_id = ml.id
    db.session.add(sub)
    db.session.commit()
    return sub.to_dict(), 201

@subs.route("/api/subscriptions/<sub_id>")
@oauth("subs:read")
def subscriptions_by_id_GET(sub_id):
    sub = Subscription.query.get(sub_id)
    if not sub:
        abort(404)
    if sub.user_id != current_token.user_id:
        abort(403)
    return sub.to_dict()

@subs.route("/api/subscriptions/<sub_id>", methods=["DELETE"])
@oauth("subs:write")
def subscriptions_by_id_DELETE(sub_id):
    sub = Subscription.query.get(sub_id)
    if not sub:
        abort(404)
    if sub.user_id != current_token.user_id:
        abort(403)
    db.session.delete(sub)
    db.session.commit()
    return {}, 204
