from email.utils import parseaddr
from flask import request, url_for
from listssrht.auth import DevAuthService
from listssrht.filters import diffstat, format_body, post_address
from listssrht.types import User, PatchsetStatus, ListAccess, Visibility
from srht.app import Flask
from srht.config import cfg
from srht.database import DbSession
from urllib.parse import quote

db = DbSession(cfg("lists.sr.ht", "connection-string"))
db.init()

access_help_map = {
    ListAccess.browse:
        "Permission to subscribe and browse the archives",
    ListAccess.reply:
        "Permission to reply to threads submitted by an authorized user.",
    ListAccess.post:
        "Permission to submit new threads.",
    ListAccess.moderate:
        "Permission to moderate threads and patches.",
}

class ListsApp(Flask):
    def __init__(self):
        super().__init__("lists.sr.ht", __name__, user_class=User)

        self.url_map.strict_slashes = False

        # FORK: resolve users locally instead of against meta.sr.ht. Must be
        # swapped in before any request runs -- the base class already built
        # an OAuthService pointed at meta in super().__init__.
        self.oauth_service = DevAuthService(self.site, user_class=User)

        from listssrht.blueprints.archives import archives
        from listssrht.blueprints.auth import auth
        from listssrht.blueprints.patches import patches
        from listssrht.blueprints.settings import settings
        from listssrht.blueprints.user import user

        self.register_blueprint(archives)
        self.register_blueprint(auth)
        self.register_blueprint(patches)
        self.register_blueprint(settings)
        self.register_blueprint(user)

        @self.context_processor
        def inject():
            return {
                "ListAccess": ListAccess,
                "access_help_map": access_help_map,
                "Visibility": Visibility,
                "diffstat": diffstat,
                "format_body": format_body,
                "parseaddr": parseaddr,
                "PatchsetStatus": PatchsetStatus,
                "post_address": post_address,
                "quote": quote,
            }

    # FORK: upstream points both of these at meta.sr.ht. Keep them as
    # properties so loginrequired's redirect and the nav links follow along.
    @property
    def login_url(self):
        # full_path tacks on a bare "?" when there's no query string
        return_to = request.full_path.rstrip("?") or request.path
        return url_for("auth.login_GET", return_to=return_to)

    @property
    def logout_url(self):
        return url_for("auth.logout")

app = ListsApp()
