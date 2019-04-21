from email.utils import parseaddr
from srht.config import cfg
from srht.database import db
from srht.oauth import AbstractOAuthService, DelegatedScope
from listssrht.types import Email, OAuthToken, Subscription, User
import sqlalchemy as sa

client_id = cfg("lists.sr.ht", "oauth-client-id")
client_secret = cfg("lists.sr.ht", "oauth-client-secret")

class ListsOAuthService(AbstractOAuthService):
    def __init__(self):
        super().__init__(client_id, client_secret, delegated_scopes=[
            DelegatedScope("lists", "mailing lists", True),
            DelegatedScope("email", "emails", False),
            DelegatedScope("subs", "subscriptions", True),
        ], user_class=User, token_class=OAuthToken)

    def lookup_or_register(self, token, token_expires, scopes):
        user = super().lookup_or_register(token, token_expires, scopes)
        if not user.id:
            db.session.flush()
            # Rewire existing subscriptions
            for sub in Subscription.query.filter(
                    Subscription.email == user.email).all():
                sub.email = None
                sub.user_id = user.id
            # Rewire existing emails
            for email in (Email.query
                    .filter(Email.sender_id is None)
                    .filter(sa.cast(Email.headers["From"], sa.String)
                        .ilike("%" + user.email + "%"))):
                name, addr = parseaddr(email.headers["From"])
                if addr == user.email:
                    email.sender_id = user.id
        db.session.commit()
        return user

    def profile_update_hook(self, user, payload):
        if user.email != payload["email"]:
            # Adopt emails sent with this address
            for email in (Email.query
                    .filter(Email.sender_id is None)
                    .filter(sa.cast(Email.headers["From"], sa.String)
                        .ilike("%" + user.email + "%"))):
                name, addr = parseaddr(email.headers["From"])
                if addr == payload["email"]:
                    email.sender_id = user.id
        super().profile_update_hook(user, payload)
