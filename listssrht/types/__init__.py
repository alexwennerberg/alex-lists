from listssrht.types.email import Email
from listssrht.types.list import List
from listssrht.types.listaccess import ListAccess
from listssrht.types.subscription import Subscription
from listssrht.types.user import User

from srht.database import Base
from srht.oauth import ExternalOAuthTokenMixin

class OAuthToken(Base, ExternalOAuthTokenMixin):
    pass
