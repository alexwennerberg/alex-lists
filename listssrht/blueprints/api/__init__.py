import pkg_resources
from flask import abort
from listssrht.blueprints.archives import get_list as _get_list
from listssrht.types import Email, User, Subscription, ListAccess
from listssrht.webhooks import UserWebhook
from srht.flask import csrf_bypass
from srht.oauth import current_token, oauth

def get_user(username):
    user = None
    if username == None:
        user = current_token.user
    elif "@" in username:
        user = User.query.filter(User.email == username).one_or_none()
    elif username.startswith("~"):
        user = User.query.filter(User.username == username[1:]).one_or_none()
    if not user:
        abort(404)
    return user

def get_list(owner_name, list_name):
    user = get_user(owner_name)
    owner, ml, access = _get_list(user.canonical_name,
            list_name, current_user=current_token.user)
    if not ml:
        abort(404)
    return owner, ml, access

def get_access(ml, user):
    if user.id == ml.owner_id:
        access = ListAccess.all
    elif Subscription.query .filter(Subscription.user_id == user.id).count():
        access = ml.subscriber_permissions | ml.account_permissions
    else:
        access = ml.account_permissions
    return access

def get_email(email_id):
    """Fetches an email by email ID or message ID"""
    try:
        email_id = int(email_id)
    except ValueError:
        pass
    if isinstance(email_id, int):
        email = Email.query.filter(Email.id == email_id)
    elif isinstance(email_id, str):
        email = Email.query.filter(Email.message_id == email_id)
    else:
        abort(404)
    email = email.one_or_none()
    if not email:
        abort(404)
    return email

def register_api(app):
    from listssrht.blueprints.api.emails import emails
    from listssrht.blueprints.api.lists import lists
    from listssrht.blueprints.api.subscriptions import subs

    app.register_blueprint(emails)
    app.register_blueprint(lists)
    app.register_blueprint(subs)

    csrf_bypass(emails)
    csrf_bypass(lists)
    csrf_bypass(subs)

    @app.route("/api/version")
    def version():
        try:
            dist = pkg_resources.get_distribution("listssrht")
            return { "version": dist.version }
        except:
            return { "version": "unknown" }

    @app.route("/api/user/<username>")
    @app.route("/api/user", defaults={"username": None})
    @oauth(None)
    def user_GET(username):
        if username == None:
            return current_token.user.to_dict()
        return get_user(username).to_dict()

    UserWebhook.api_routes(app, "/api/user")
