from flask import Blueprint, render_template, abort, request, redirect, url_for
from flask import Response, session
from flask_login import current_user
from sqlalchemy import String, cast, or_
from srht.config import cfg
from srht.database import db
from srht.search import search
from srht.flask import paginate_query, loginrequired
from srht.validation import Validation
from listssrht.filters import post_address
from listssrht.types import List, User, Email, Subscription, ListAccess, Access
from listssrht.webhooks import ListWebhook, UserWebhook
from urllib.parse import quote, urlencode
import email
import email.utils

archives = Blueprint("archives", __name__)

msgauth_server = cfg("lists.sr.ht", "msgauth-server", default=None)

def get_list(owner_name, list_name, current_user=current_user):
    if owner_name and owner_name.startswith('~'):
        owner_name = owner_name[1:]
        owner = User.query.filter(User.username == owner_name).one_or_none()
        if not owner:
            return None, None, None
    else:
        # TODO: orgs
        return None, None, None
    ml = (List.query
            .filter(List.name == list_name)
            .filter(List.owner_id == owner.id)
        )
    if current_user:
        ml = ml.outerjoin(Access, Access.list_id == List.id)
    ml = ml.one_or_none()
    if not ml:
        return None, None, None
    if current_user:
        acl = next((acl for acl in ml.acls
            if acl.user_id == current_user.id), None)
        if current_user.id == ml.owner_id:
            access = ListAccess.all
        elif acl:
            access = acl.permissions
        elif (Subscription.query
                .filter(Subscription.user_id == current_user.id)).count():
            access = ml.subscriber_permissions | ml.account_permissions
        else:
            access = ml.account_permissions
    else:
        access = ml.nonsubscriber_permissions
    return owner, ml, access

def apply_search(query):
    terms = request.args.get("search")
    if not terms:
        return query.filter(Email.parent_id == None), None
    def canonicalize(header):
        return "-".join(h[0].upper() + h[1:] for h in header.split("-"))
    def me_alias(header, q, v):
        return (q.filter(cast(Email.headers[header], String).ilike(
                "%" + current_user.email + "%"))
            if current_user and v == "me" else
            q.filter(Email.headers[header].ilike("%" + v + "%")))
    return search(query, terms, [Email.body, Email.subject], {
        "is": lambda q, v: q.filter({
            "patch": Email.is_patch,
            "request-pull": Email.is_request_pull,
        }.get(v, False)),
        "from": lambda q, v: me_alias("From", q, v),
        "to": lambda q, v: me_alias("To", q, v),
        "cc": lambda q, v: me_alias("Cc", q, v),
        None: lambda q, p, v: query.filter(cast(
            Email.headers[canonicalize(p)], String).ilike("%" + v + "%")),
    }), terms

def _dkim_explain(status, domain):
    return {
        "pass": f"Valid DKIM signature for {domain}",
        "fail": f"Invalid DKIM signature for {domain}. The message may have" +
            f" been tampered with, or the mail server at {domain} is" +
            " misconfigured.",
        "policy": "This email has a DKIM signature, but is for some reason" +
            " unsuitable for the policy of the recipient.",
        "neutral": "This email has a DKIM signature, but it has syntax errors" +
            " or other problems rendering it meaningless. This is generally" +
            f" a configuration error with the mail server at {domain}.",
        "temperror": "A temporary error occured while validating this DKIM" +
            " signature.",
        "permerror": "A permanent error occured while validating this DKIM" +
            " signature, such as a missing or invalid header. This is" +
            f" generally a configuration error with the mail server at {domain}."
    }.get(status)

def parse_auth_result(mail, method):
    domain = email.utils.parseaddr(mail["From"])[1].split("@", 2)[1].lower()
    if msgauth_server is None:
        return None, None
    fields = mail.get_all("Authentication-Results", failobj=[])
    for field in fields:
        parts = field.lower().replace(';', ' ').split()
        host = parts.pop(0)
        if host != msgauth_server:
            continue
        if parts[0].isalnum():
            version = parts.pop(0)
            if version != "1":
                continue
        [meth, result] = parts.pop(0).split('=', 2)
        if meth != method.lower():
            continue
        if not "header.d=" + domain in parts:
            continue
        return result, _dkim_explain(result, domain)
    return None, _dkim_explain("none", domain)

@archives.route("/<owner_name>/<list_name>")
def archive(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.browse not in access:
        abort(403)
    threads = (Email.query
            .filter(Email.list_id == ml.id)
        ).order_by(Email.updated.desc())
    threads, search = apply_search(threads)
    threads, pagination = paginate_query(threads)

    subscription = None
    if current_user:
        subscription = (Subscription.query
                .filter(Subscription.list_id == ml.id)
                .filter(Subscription.user_id == current_user.id)).one_or_none()

    message = session.pop("message", None)
    return render_template("archive.html",
            view="archives", owner=owner, ml=ml, threads=threads,
            access=access, ListAccess=ListAccess,
            search=search, subscription=subscription,
            message=message, **pagination)

@archives.route("/<owner_name>/<list_name>/<message_id>")
def thread(owner_name, list_name, message_id):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.browse not in access:
        abort(403)
    thread = (Email.query
            .filter(Email.message_id == message_id)
            .filter(Email.list_id == ml.id)
        ).one_or_none()
    if not thread:
        abort(404)
    if thread.thread_id != None:
        return redirect(url_for("archives.thread",
            owner_name=owner_name,
            list_name=list_name,
            message_id=thread.thread.message_id) + "#" + thread.message_id)
    patches = [mail for mail in thread.descendants if mail.is_patch]
    if thread.is_patch:
        patches.append(thread)
    patches = sorted(patches, key=lambda p: p.created)

    def reply_to(msg):
        params = {
            "cc": msg.parsed()['From'],
            "in-reply-to": msg.message_id,
            "subject": (f"Re: {msg.subject}"
                if not msg.subject.lower().startswith("re:")
                else msg.subject),
        }
        return f"mailto:{post_address(msg.list)}?{urlencode(params, quote_via=quote)}"

    return render_template("thread.html", view="archives", owner=owner,
            ml=ml, thread=thread, patches=patches,
            parseaddr=email.utils.parseaddr,
            parse_auth_result=parse_auth_result, reply_to=reply_to)

@archives.route("/<owner_name>/<list_name>/<message_id>/raw")
def raw(owner_name, list_name, message_id):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.browse not in access:
        abort(403)
    message = (Email.query
            .filter(Email.message_id == message_id)
            .filter(Email.list_id == ml.id)
        ).one_or_none()
    if not message:
        abort(404)
    return Response(message.envelope, mimetype='text/plain')

def format_mbox(msg):
    parsed = msg.parsed()
    b = parsed.as_bytes(unixfrom=True) + b'\r\n'
    for reply in msg.replies:
        b += format_mbox(reply)
    return b

@archives.route("/<owner_name>/<list_name>/<message_id>/mbox")
def mbox(owner_name, list_name, message_id):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.browse not in access:
        abort(403)
    thread = (Email.query
            .filter(Email.message_id == message_id)
            .filter(Email.list_id == ml.id)
        ).one_or_none()
    if not thread or thread.thread_id != None:
        abort(404)
    mbox = format_mbox(thread)
    return Response(mbox, mimetype='application/mbox')

@archives.route("/<owner_name>/<list_name>/<message_id>/remove", methods=["POST"])
@loginrequired
def remove_message(owner_name, list_name, message_id):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(401)
    message = (Email.query
            .filter(Email.message_id == message_id)
            .filter(Email.list_id == ml.id)
        ).one_or_none()
    if not message:
        abort(404)
    redir = url_for("archives.archive",
            owner_name=owner_name, list_name=list_name)
    if message.thread != None:
        redir = url_for("archives.thread",
            owner_name=owner_name, list_name=list_name,
            message_id=message.thread.message_id)
    db.session.delete(message)
    db.session.commit()
    return redirect(redir)

@archives.route("/<owner_name>/<list_name>/subscribe", methods=["POST"])
@loginrequired
def subscribe(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.browse not in access:
        abort(403)
    sub = (Subscription.query
        .filter(Subscription.list_id == ml.id)
        .filter(Subscription.user_id == current_user.id)).one_or_none()
    if sub:
        return redirect(url_for("archives.archive",
            owner_name=owner_name, list_name=list_name))
    sub = Subscription()
    sub.user_id = current_user.id
    sub.user = current_user
    sub.list_id = ml.id
    sub.list = ml
    db.session.add(sub)
    UserWebhook.deliver(UserWebhook.Events.subscription_create,
            sub.to_dict(), UserWebhook.Subscription.user_id == sub.user_id)
    db.session.commit()
    return redirect(url_for("archives.archive",
        owner_name=owner_name, list_name=list_name))

@archives.route("/<owner_name>/<list_name>/unsubscribe", methods=["POST"])
@loginrequired
def unsubscribe(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    sub = (Subscription.query
        .filter(Subscription.list_id == ml.id)
        .filter(Subscription.user_id == current_user.id)).one_or_none()
    if sub:
        db.session.delete(sub)
        UserWebhook.deliver(UserWebhook.Events.subscription_remove,
                { "id": sub.id },
                UserWebhook.Subscription.user_id == sub.user_id)
        db.session.commit()
    return redirect(url_for("archives.archive",
        owner_name=owner_name, list_name=list_name))
