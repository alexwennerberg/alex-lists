from email.mime.text import MIMEText
from email.utils import parseaddr, formatdate, make_msgid
from flask import Blueprint, render_template, request, redirect, url_for, abort
from flask_login import current_user
from srht.config import cfg, cfgi
from srht.database import db
from srht.flask import loginrequired
from srht.validation import Validation
from sqlalchemy import or_
from listssrht.types import List, User, Email, Subscription, Mirror
from listssrht.webhooks import UserWebhook
import re
import smtplib

user = Blueprint("user", __name__)

meta_uri = cfg("meta.sr.ht", "origin")

smtp_host = cfg("mail", "smtp-host", default=None)
smtp_port = cfgi("mail", "smtp-port", default=None)
smtp_user = cfg("mail", "smtp-user", default=None)
smtp_password = cfg("mail", "smtp-password", default=None)

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
    UserWebhook.deliver(UserWebhook.Events.list_create,
            ml.to_dict(), UserWebhook.Subscription.user_id == ml.owner_id)

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

@user.route("/lists/create-mirror")
@loginrequired
def create_mirror_GET():
    return render_template("create-mirror.html")

def mirror_subscribe(ml, mirror):
    posting_domain = cfg("lists.sr.ht", "posting-domain")
    list_name = "u.{}.{}".format(ml.owner.username, ml.name)

    smtp = smtplib.SMTP(smtp_host, smtp_port)
    smtp.ehlo()
    smtp.starttls()
    smtp.login(smtp_user, smtp_password)

    mail = MIMEText(f"Subscription request for {posting_domain} on behalf of "
        f"{ml.owner.canonical_name}\n\n"
        "If this email is unexpected, feel free to ignore it, or send "
        "questions to:\n\n"
        f"{cfg('sr.ht', 'owner-name')} <{cfg('sr.ht', 'owner-email')}>")
    mail["X-Mirroring-To"] = posting_domain
    mail["Subject"] = "subscribe"
    mail["To"] = mirror.list_subscribe
    mail["From"] = f"{posting_domain} mirror <{list_name}@{posting_domain}>"
    mail["Date"] = formatdate()
    mail["Message-ID"] = make_msgid()
    smtp.sendmail(smtp_user, [mirror.list_subscribe], mail.as_string(
        unixfrom=True, maxheaderlen=998))
    smtp.quit()

@user.route("/lists/create-mirror", methods=["POST"])
@loginrequired
def create_mirror_POST():
    valid = Validation(request)
    ml = List(current_user, valid)
    address = valid.require("address", friendly_name="Subscription address")
    valid.expect(not address or "@" in address,
            "A valid email address is required", field="address")
    weird_ok = valid.optional("weird-email-okay")
    valid.expect(not address or weird_ok == "yes" or "subscribe" in address,
            "This address does not look like a subscription address. Double "
            "check it and click 'Create' if you're certain.", field="address")
    if not valid.ok:
        return render_template("create-mirror.html", **valid.kwargs)

    posting_domain = cfg("lists.sr.ht", "posting-domain")

    user, domain = address.split("@")
    valid.expect(domain != posting_domain,
            "You can't mirror a list from {{cfg('sr.ht', 'site-name')}}!",
            field="address")
    if not valid.ok:
        return render_template("create-mirror.html", **valid.kwargs)

    mirror = Mirror()
    mirror.list_subscribe = address
    db.session.add(mirror)
    db.session.flush()
    ml.mirror_id = mirror.id

    mirror_subscribe(ml, mirror)

    UserWebhook.deliver(UserWebhook.Events.list_create,
            ml.to_dict(), UserWebhook.Subscription.user_id == ml.owner_id)

    db.session.commit()
    return redirect(url_for("archives.archive",
            owner_name=current_user.canonical_name,
            list_name=ml.name))
