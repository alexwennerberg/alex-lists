from datetime import datetime, timedelta
from flask import Blueprint, render_template, abort, request, redirect, url_for
from flask import Response, session, send_file
from sqlalchemy import String, select, cast, or_
from sqlalchemy.sql.functions import coalesce
from srht.config import cfg
from srht.database import db
from srht.search import search_by
from srht.flask import paginate_query
from srht.oauth import current_user, loginrequired
from srht.validation import Validation
from listssrht.filters import post_address
from listssrht.types import List, User, Email, Subscription, ListAccess, Access
from listssrht.types import Patchset, PatchsetStatus
from listssrht.process import forward_thread
from listssrht.webhooks import ListWebhook, UserWebhook
from urllib.parse import quote, urlencode
import email
import email.policy
import email.utils
import hashlib
import mailbox
import os

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

def apply_search(query, search):
    if not search:
        return query.filter(Email.parent_id == None)

    def canonicalize(header):
        return "-".join(h[0].upper() + h[1:] for h in header.split("-"))

    def header_filter(name, value):
        header = cast(Email.headers[name], String)
        return header.ilike(f"%{value}%")

    def user_alias(header, value):
        if current_user and value == "me":
            value = current_user.email
        elif "@" not in value:
            # SELECT email.subject, ... FROM "email"  -- from toplevel query
            # WHERE ...
            # AND CAST(email.headers -> 'From' AS VARCHAR)
            #     ILIKE COALESCE((SELECT '%%' || "user".email || '%%' FROM "user"
            #                             WHERE "user".username = 'zupa'),
            #                            '%%zupa%%')
            # AND CAST(email.headers -> 'Cc'   AS VARCHAR)
            #     ILIKE COALESCE((SELECT '%%' || "user".email || '%%' FROM "user"
            #                             WHERE "user".username = 'zeta'),
            #                            '%%zeta%%')
            # ORDER BY email.updated DESC;
            #
            # Try to find a user with the specified name,
            # or default to fuzzy search if no such user exists

            username = value
            if username.startswith("~"):
                username = username[1:]

            header = cast(Email.headers[header], String)
            return header.ilike(coalesce(select(['%' + User.email + '%'])
                    .where(User.username == username).as_scalar(),
                    f"%{username}%"))

        return header_filter(header, value)

    def patchset_status(value):
        status = getattr(PatchsetStatus, value, None)
        if not status:
            raise ValueError(f"Invalid patchset status: '{value}'")

        return Email.patchset.has(Patchset.status == status)

    return search_by(query, search, [Email.body, Email.subject], {
        "is": lambda v: {
            "patch": Email.is_patch,
            "reply": Email.parent_id != None,
            "request-pull": Email.is_request_pull,
            "thread": Email.nreplies > 0,
        }.get(v, False),
        "from": lambda v: user_alias("From", v),
        "to": lambda v: user_alias("To", v),
        "cc": lambda v: user_alias("Cc", v),
        "status": patchset_status,
        "prefix": lambda v: Email.patchset.has(Patchset.prefix == v),
        "sender-timestamp": lambda v: (
            Email.message_date == datetime.utcfromtimestamp(int(v))),
    }, fallback_fn=header_filter)

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
    address = email.utils.parseaddr(mail["From"])[1]
    mailboxhost = address.split("@", 2)
    if len(mailboxhost) < 2:
        return None, None
    domain = mailboxhost[1].lower()
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
    if access.value == 0:
        abort(403)
    threads = (Email.query
            .filter(Email.list_id == ml.id)
        ).order_by(Email.updated.desc())

    search = request.args.get("search")
    search_error = None
    try:
        threads = apply_search(threads, search)
    except ValueError as ex:
        search_error = str(ex)

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
            search=search, search_error=search_error, subscription=subscription,
            parseaddr=email.utils.parseaddr,
            message=message, **pagination)

@archives.route("/<owner_name>/<list_name>/<path:message_id>")
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

    messages = (Email.query
            .filter(Email.thread_id == thread.id)
            .order_by(Email.created)).all()

    def reply_to(msg):
        params = {
            "cc": msg.parsed()['From'],
            "in-reply-to": msg.message_id.strip('<>'),
            "subject": (f"Re: {msg.subject}"
                if not msg.subject.lower().startswith("re:")
                else msg.subject),
        }
        pa = post_address(msg.list)
        if pa.startswith("mailto:"):
            return f"{pa}?{urlencode(params, quote_via=quote)}"
        else:
            return f"mailto:{pa}?{urlencode(params, quote_via=quote)}"

    user_message = session.pop("message", None)
    return render_template("thread.html", view="archives", owner=owner,
            access=access, ListAccess=ListAccess,
            ml=ml, thread=thread, messages=messages,
            parseaddr=email.utils.parseaddr,
            parse_auth_result=parse_auth_result,
            reply_to=reply_to,
            user_message=user_message)

@archives.route("/<owner_name>/<list_name>/<path:message_id>/raw")
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
    b = parsed.as_bytes(unixfrom=True, policy=email.policy.SMTPUTF8) + b'\r\n'
    for reply in msg.replies:
        b += format_mbox(reply)
    return b

@archives.route("/<owner_name>/<list_name>/<path:message_id>/mbox")
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
    try:
        mbox = format_mbox(thread)
    except UnicodeEncodeError:
        return Validation(request).error("Encoding error", status=500)
    return Response(mbox, mimetype='application/mbox')

@archives.route("/<owner_name>/<list_name>/<path:message_id>/remove", methods=["POST"])
@loginrequired
def remove_message(owner_name, list_name, message_id):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.moderate not in access:
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
    # Prevent the before_update hook in srht.database, which'd run even though
    # ml wasn't modified, from setting the updated date
    ml._no_autoupdate = True
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

@archives.route("/<owner_name>/<list_name>/forward/<path:message_id>", methods=["POST"])
@loginrequired
def forward(owner_name, list_name, message_id):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.browse not in access:
        abort(403)
    email = (Email.query
            .filter(Email.message_id == message_id)
            .filter(Email.list_id == ml.id)).one_or_none()
    if not email:
        abort(404)
    forward_thread.delay(ml.id, email.id, current_user.email)
    session["message"] = "This thread has been forwarded to you."
    if "patch" in request.args:
        return redirect(url_for("patches.patchset",
                owner_name=owner_name, list_name=list_name,
                patchset_id=request.args["patch"]))
    return redirect(url_for("archives.thread",
            owner_name=owner_name, list_name=list_name,
            message_id=message_id))

@archives.route("/<owner_name>/<list_name>/export", methods=["POST"])
def export_archive(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.browse not in access:
        abort(403)

    days = request.form.get("days")
    if not days:
        abort(400)
    try:
        days = int(days)
    except:
        abort(400)

    sha = hashlib.sha256()
    sha.update(os.urandom(24))
    digest = sha.hexdigest()
    export_dir = cfg('lists.sr.ht', 'export-directory', default='/tmp')
    path = f"/{export_dir}/{digest}.mbox"

    mbox = mailbox.mbox(path)
    query = (Email.query
            .filter(Email.list_id == ml.id)
            .order_by(Email.created))
    if days > 0:
        query = query.filter(Email.created >
                datetime.utcnow() - timedelta(days=days))
    for message in query.all():
        try:
            mbox.add(message.parsed())
        except:
            continue # plow on forward
    mbox.close()

    f = open(path, "rb")
    os.unlink(path)
    return send_file(f, as_attachment=True,
            attachment_filename=f"{owner.username}-{list_name}.mbox")
