from flask import Blueprint, abort
from listssrht.blueprints.api import get_user, get_list
from listssrht.types import User, List, Email, ListAccess
from sqlalchemy import or_
from srht.api import paginated_response
from srht.oauth import oauth, current_token, OAuthScope

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

@lists.route("/api/user/<username>/lists/<list_name>")
@lists.route("/api/lists/<list_name>", defaults={"username": None})
@oauth("lists:read")
def user_lists_by_name_GET(username, list_name):
    user, ml, access = get_list(username, list_name)
    if not ListAccess.browse in access:
        abort(404)
    return ml.to_dict()

@lists.route("/api/user/<username>/lists/<list_name>/emails")
@lists.route("/api/lists/<list_name>/emails", defaults={"username": None})
@oauth("lists:read")
def user_lists_by_name_emails_GET(username, list_name):
    user, ml, access = get_list(username, list_name)
    if not ListAccess.browse in access:
        abort(404)
    emails = Email.query.filter(Email.list_id == ml.id)
    return paginated_response(Email.id, emails, short=True)
