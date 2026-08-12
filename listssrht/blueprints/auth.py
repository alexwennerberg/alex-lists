"""
Login/logout routes for the local dev auth stub. See listssrht/auth.py for
what this replaces and why it is not safe.
"""
from flask import Blueprint, redirect, render_template, request, url_for
from srht.oauth import current_user, login_user, logout_user
from listssrht.auth import get_or_create_user, valid_username
from listssrht.types import User
from urllib.parse import urlparse

auth = Blueprint("auth", __name__)


def _safe_return_to(target):
    """
    Confine post-login redirects to this site. Even a toy login shouldn't be
    a usable open redirect.
    """
    if not target:
        return url_for("user.index")
    parsed = urlparse(target)
    if parsed.scheme or parsed.netloc:
        return url_for("user.index")
    if not parsed.path.startswith("/"):
        return url_for("user.index")
    return parsed.path + (f"?{parsed.query}" if parsed.query else "")


def _render_login(error=None, status=200):
    # Offered as one-click buttons so you don't have to remember who exists.
    users = User.query.order_by(User.username).limit(50).all()
    return render_template("login.html",
            users=users,
            return_to=request.values.get("return_to"),
            error=error), status


@auth.route("/login")
def login_GET():
    if current_user:
        return redirect(_safe_return_to(request.args.get("return_to")))
    return _render_login()


@auth.route("/login", methods=["POST"])
def login_POST():
    username = (request.form.get("username") or "").strip().lstrip("~")
    if not valid_username(username):
        return _render_login(
                error="Usernames may contain letters, digits, '.', '_' and "
                      "'-', must start with a letter or digit, and must be "
                      "at most 128 characters.",
                status=400)

    user = get_or_create_user(username)
    # set_cookie=True writes the encrypted sr.ht.unified-login.v1 cookie that
    # srht.app.Flask.get_session_cookie reads back on subsequent requests.
    login_user(user, set_cookie=True)
    return redirect(_safe_return_to(request.form.get("return_to")))


@auth.route("/logout")
def logout():
    logout_user()
    return redirect(url_for("user.index"))
