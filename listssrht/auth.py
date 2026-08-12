"""
Local stand-in for meta.sr.ht authentication.

FORK NOTE: upstream lists.sr.ht delegates all identity to meta.sr.ht over
OAuth2 -- there is no local notion of a credential. This fork has no
meta.sr.ht, so this module replaces that with nothing at all: anyone may log
in as anyone, no password, and unknown usernames are created on demand.

This is a placeholder to unblock development. It is not authentication. Any
instance running this is wide open to whoever can reach it -- do not expose it
to a network. Replace this module when real auth goes in; the rest of the app
only touches it through OAuthService.lookup_user and srht.oauth.login_user, so
swapping it out shouldn't ripple.
"""
from datetime import datetime
from srht.database import db
from srht.oauth import OAuthService, UserType
from listssrht.types import User

import re

# Same shape sourcehut allows, so generated ~user/list URLs stay well-formed.
USERNAME_RE = re.compile(r"^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$")


def valid_username(username):
    return bool(username) and bool(USERNAME_RE.match(username))


def get_or_create_user(username):
    """
    Resolve a username to a User, creating one if we've never seen it.

    Upstream this row arrives from meta.sr.ht, either by webhook or by the
    profile fetch in OAuthService.fetch_unknown_user.
    """
    user = User.query.filter(User.username == username).one_or_none()
    if user:
        return user

    now = datetime.utcnow()
    user = User()
    user.username = username
    user.created = now
    user.updated = now
    # email is NOT NULL UNIQUE; there's no real address to put here.
    user.email = f"{username}@localhost"
    user.user_type = UserType.user
    db.session.add(user)
    db.session.commit()
    return user


class DevAuthService(OAuthService):
    """
    OAuthService that resolves users against the local database instead of
    querying meta.sr.ht's GraphQL API.

    lookup_user() (inherited) checks the local user table first and only falls
    through to fetch_unknown_user on a miss, so overriding that one method is
    enough to cut meta out of the session-cookie path.
    """

    def fetch_unknown_user(self, username):
        if not valid_username(username):
            return None
        return get_or_create_user(username)

    def ensure_meta_webhooks(self, user, webhooks):
        pass  # no meta.sr.ht to register webhooks with


def get_profile(user):
    """
    Local replacement for srht.app.get_profile, which fetches avatar/bio/etc
    from meta.sr.ht and aborts 404 when it can't. Everything meta would return
    that we actually store lives on the user row already.
    """
    return {
        "avatar": None,
        "bio": user.bio,
        "location": user.location,
        "pronouns": None,
        "url": user.url,
    }
