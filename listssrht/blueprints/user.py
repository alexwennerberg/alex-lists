from email.utils import parseaddr
from flask import Blueprint, render_template, request, redirect, url_for, abort
from flask_login import current_user
from srht.config import cfg
from srht.database import db
from srht.flask import loginrequired
from srht.validation import Validation
from sqlalchemy import or_
from listssrht.types import List, User, Email, Subscription
import re

user = Blueprint("user", __name__)

meta_uri = cfg("meta.sr.ht", "origin")

@user.route("/")
def index():
    if not current_user:
        return render_template("index.html")
    recent = (Email.query
            .join(List)
            .join(Subscription)
            .filter(Email.list_id == List.id)
            .filter(Subscription.list_id == List.id)
            .filter(Subscription.user_id == current_user.id)
            .order_by(Email.created.desc())).limit(10).all()
    subs = [sub.list for sub in (Subscription.query
            .join(List)
            .filter(Subscription.user_id == current_user.id)
            .order_by(List.updated.desc())).limit(10).all()]
    return render_template("dashboard.html", recent=recent, subs=subs)

@user.route("/~<username>")
def user_profile(username):
    user = User.query.filter(User.username == username).first()
    if not user:
        abort(404)
    recent = Email.query.filter(Email.sender_id == user.id)
    lists = List.query.filter(List.owner_id == user.id)

    if current_user:
        if current_user.id != user.id:
            lists = lists.filter(or_(
                    List.account_permissions > 0,
                    List.nonsubscriber_permissions > 0
                ))
            recent = recent.join(List).filter(or_(
                List.account_permissions > 0,
                List.nonsubscriber_permissions > 0))
    else:
        lists = lists.filter(List.nonsubscriber_permissions > 0)
        recent = (recent.join(List)
                .filter(List.nonsubscriber_permissions > 0))

    recent = recent.order_by(Email.created.desc()).limit(10).all()
    lists = lists.order_by(List.updated.desc()).limit(10).all()

    return render_template("user.html",
            user=user, recent=recent, lists=lists, parseaddr=parseaddr)

@user.route("/lists/create")
@loginrequired
def create_list_GET():
    return render_template("create.html")

@user.route("/lists/create", methods=["POST"])
def create_list_POST():
    valid = Validation(request)
    ml = List(current_user, valid)
    if not valid.ok:
        return render_template("create.html", **valid.kwargs)
    db.session.add(ml)
    db.session.flush()

    # Auto-subscribe the owner
    sub = Subscription()
    sub.user_id = current_user.id
    sub.list_id = ml.id
    sub.confirmed = True
    db.session.add(sub)
    db.session.commit()

    return redirect(url_for("archives.archive",
            owner_name=current_user.canonical_name,
            list_name=ml.name))
