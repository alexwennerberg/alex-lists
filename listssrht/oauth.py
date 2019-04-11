from srht.config import cfg
from srht.database import db
from srht.oauth import AbstractOAuthService, DelegatedScope
from listssrht.types import User, OAuthToken, Subscription

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
        db.session.flush()
        # Rewire existing subscriptions
        for sub in Subscription.query.filter(
                Subscription.email == user.email).all():
            sub.email = None
            sub.user_id = user.id
        db.session.commit()
        return user
