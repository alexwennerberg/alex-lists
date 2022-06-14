from flask import Blueprint, abort
from listssrht.blueprints.api import get_user, get_access, get_email
from listssrht.types import List, Email, ListAccess
from sqlalchemy import or_
from srht.api import paginated_response
from srht.oauth import oauth, current_token

emails = Blueprint("api_emails", __name__)

@emails.route("/api/user/<username>/emails")
@emails.route("/api/emails", defaults={"username": None})
@oauth("emails:read")
def user_emails_GET(username):
    user = get_user(username)
    emails = Email.query.filter(Email.sender_id == user.id)
    if current_token.user_id != user.id:
        emails = emails.join(List, List.id == Email.list_id).filter(
            List.default_access > 0)
    return paginated_response(Email.id,
            emails.order_by(Email.created.desc()), short=True)

@emails.route("/api/user/<username>/emails/<email_id>")
@emails.route("/api/emails/<email_id>", defaults={"username": None})
@oauth("emails:read")
def emails_by_id_GET(username, email_id):
    email = get_email(email_id) # Note: username is not used
    if ListAccess.browse not in get_access(email.list, current_token.user):
        abort(403)
    return email.to_dict()

@emails.route("/api/user/<username>/thread/<email_id>")
@emails.route("/api/thread/<email_id>", defaults={"username": None})
@oauth("emails:read")
def thread_by_id_GET(username, email_id):
    email = get_email(email_id) # Note: username is not used
    if email.parent_id is not None:
        abort(404)
    if ListAccess.browse not in get_access(email.list, current_token.user):
        abort(403)
    return [email.to_dict()] + [m.to_dict() for m in email.descendants]
