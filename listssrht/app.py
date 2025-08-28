from listssrht.filters import diffstat, format_body, post_address
from listssrht.types import User
from srht.config import cfg
from srht.database import DbSession
from srht.flask import SrhtFlask
from urllib.parse import quote

db = DbSession(cfg("lists.sr.ht", "connection-string"))
db.init()

class ListsApp(SrhtFlask):
    def __init__(self):
        super().__init__("lists.sr.ht", __name__, user_class=User)

        self.url_map.strict_slashes = False

        from listssrht.blueprints.archives import archives
        from listssrht.blueprints.patches import patches
        from listssrht.blueprints.settings import settings
        from listssrht.blueprints.user import user
        from srht.graphql import gql_blueprint

        self.register_blueprint(archives)
        self.register_blueprint(patches)
        self.register_blueprint(settings)
        self.register_blueprint(user)
        self.register_blueprint(gql_blueprint)

        @self.context_processor
        def inject():
            from listssrht.types import ListAccess
            return {
                "diffstat": diffstat,
                "format_body": format_body,
                "post_address": post_address,
                "quote": quote,
                "ListAccess": ListAccess,
            }

app = ListsApp()
