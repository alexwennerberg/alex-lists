from srht.config import cfg
from srht.oauth import AbstractOAuthService, DelegatedScope
from listssrht.types import User, OAuthToken

client_id = cfg("lists.sr.ht", "oauth-client-id")
client_secret = cfg("lists.sr.ht", "oauth-client-secret")

class ListsOAuthService(AbstractOAuthService):
    def __init__(self):
        super().__init__(client_id, client_secret, delegated_scopes=[
            DelegatedScope("lists", "mailing lists", True),
            DelegatedScope("email", "emails", True),
            DelegatedScope("subs", "subscriptions", True),
        ], user_class=User, token_class=OAuthToken)
