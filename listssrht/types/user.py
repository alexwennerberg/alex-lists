from srht.database import Base
from srht.oauth import UserMixin
import sqlalchemy as sa

class User(Base, UserMixin):
    # TODO: move sessions into core.sr.ht
    session = sa.Column(sa.String(128))

    copy_self = sa.Column(sa.Boolean, nullable=False, server_default='f')
    """Send the user a copy of their own posts to lists they subscribe to."""

    def to_dict(self, short=False, first_party=False):
        # srht.app.Flask.make_response calls to_dict(first_party=True) when it
        # writes the login cookie, but UserMixin.to_dict only accepts `short`.
        # Upstream never trips over this because only meta.sr.ht sets that
        # cookie; our local login does, so absorb the kwarg here.
        return super().to_dict(short=short)
