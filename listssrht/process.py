from srht.config import cfg, cfgi
from srht.database import DbSession, db
if not hasattr(db, "session"):
    db = DbSession(cfg("lists.sr.ht", "connection-string"))
    import listssrht.types
    db.init()
from srht.email import start_smtp
from listssrht.types import Email, List, User, Subscription, ListAccess
from listssrht.types import Patchset

import base64
import email
import email.utils
import email.policy
import io
import json
import mailbox
import pygit2
import re
import smtplib
import tempfile
from celery import Celery
from datetime import datetime
from email.mime.text import MIMEText
from email.utils import parseaddr, getaddresses, formatdate, make_msgid
from email.utils import parsedate_to_datetime
from sqlalchemy import or_
from urllib.parse import quote

dispatch = Celery("lists.sr.ht", broker=cfg("lists.sr.ht", "redis"))

smtp_host = cfg("mail", "smtp-host", default=None)
smtp_port = cfgi("mail", "smtp-port", default=None)
smtp_user = cfg("mail", "smtp-user", default=None)
smtp_password = cfg("mail", "smtp-password", default=None)

policy = email.policy.SMTPUTF8.clone(max_line_length=998)


def _prep_mail(dest, mail):
    domain = cfg("lists.sr.ht", "posting-domain")
    list_name = "{}/{}".format(dest.owner.canonical_name, dest.name)
    archive_url = "{}/{}".format(cfg("lists.sr.ht", "origin"), list_name)
    list_unsubscribe = list_name + "+unsubscribe@" + domain
    list_subscribe = list_name + "+subscribe@" + domain
    for overwrite in ["List-Unsubscribe", "List-Subscribe", "List-Archive",
                "List-Post", "List-ID", "Sender"]:
        del mail[overwrite]

    mail["List-Unsubscribe"] = (
            "<mailto:{}?subject=unsubscribe>".format(list_unsubscribe))
    mail["List-Subscribe"] = (
            "<mailto:{}?subject=subscribe>".format(list_subscribe))
    mail["List-Archive"] = "<{}>".format(archive_url)
    mail["Archived-At"] = "<{}/{}>".format(archive_url, quote(mail["Message-ID"]))
    mail["List-Post"] = "<mailto:{}@{}>".format(list_name, domain)
    mail["List-ID"] = "{} <{}.{}>".format(list_name, list_name, domain)
    mail["Sender"] = "{} <{}@{}>".format(list_name, list_name, domain)
    return mail

def _forward(dest, mail):
    mail = _prep_mail(dest, mail)

    # TODO: Encrypt emails
    smtp = start_smtp()
    froms = mail.get_all('From', [])
    tos = mail.get_all('To', [])
    ccs = mail.get_all('Cc', [])
    recipients = set([a[1] for a in getaddresses(froms + tos + ccs)])

    for sub in dest.subscribers:
        to = sub.email
        if sub.user:
            to = sub.user.email
        if to in recipients:
            print(to + " is already copied, skipping")
            continue
        print("Forwarding message to " + to)
        try:
            smtp.send_message(mail, smtp_user, [to])
        except:
            continue
    smtp.quit()

patch_subject = re.compile(r".*\[(?:RFC )?PATCH"
    r"( (?P<prefix>[^\]]+))?\] (?P<subject>.*)")
patch_version = re.compile(r"(v(?P<version>[0-9]+))?"
    r"( ?(?P<index>[0-9]+)/(?P<count>[0-9]+))?$")

def _import_patch(thread, mail, envelope):
    match = patch_subject.match(mail.subject)
    if not match:
        # TODO: figure out a better way of dealing with patches that have weird
        # subjects
        return
    prefix = match.group("prefix")
    subject = match.group("subject")
    version = index = count = 1
    match = patch_version.search(prefix) if prefix else None
    if match:
        version = int(match.group("version")) if match.group("version") else 1
        index = int(match.group("index")) if match.group("index") else 1
        count = int(match.group("count")) if match.group("count") else 1
        prefix = patch_version.sub("", prefix).strip()
    mail.patch_index = index
    mail.patch_count = count
    mail.patch_version = version
    mail.patch_prefix = prefix
    mail.patch_subject = subject
    # TODO: generate diffstat?
    print(f"Received patch {index}/{count}: {subject}")
    if not all(any(m for m in thread
            if m.patch_index == i) for i in range(1, count+1)):
        return
    if any(m.patchset_id for m in thread):
        return # TODO: is this a new revision? complicated
    print("Complete patchset received")

    def is_cover(m):
        match = patch_subject.match(m.subject)
        prefix = match.group("prefix") if match else None
        match = patch_version.search(prefix) if prefix else None
        if not match:
            return False
        index = (int(match.group("index").strip())
            if match.group("index") else 1)
        return index == 0
    cover_letter = next((m for m in thread if is_cover(m)), None)

    patchset = Patchset()
    patchset.cover_letter_id = cover_letter.id if cover_letter else None
    patchset.submitter = envelope["From"]
    patchset.message_id = envelope["Message-ID"].strip()
    patchset.reply_to = envelope["Reply-To"]

    subject = cover_letter.subject if cover_letter else thread[0].subject
    match = patch_subject.match(subject)
    patchset.subject = match.group("subject") if match else subject
    patchset.prefix = prefix

    patchset.list_id = mail.list_id
    patchset.version = version
    db.session.add(patchset)
    db.session.flush()
    for m in thread:
        m.patchset_id = patchset.id
    db.session.commit()

    from listssrht.webhooks import ListWebhook
    ListWebhook.deliver(ListWebhook.Events.patchset_received,
            patchset.to_dict(),
            ListWebhook.Subscription.list_id == mail.list_id)
    db.session.commit()
    # TODO: identify patchset that this supersedes, if appropriate
    return patchset

def _archive(dest, envelope):
    mail = Email()
    # TODO: Use message date within a tolerance from now
    mail.created = datetime.utcnow()
    mail.updated = datetime.utcnow()
    mail.subject = envelope["Subject"]
    mail.message_id = envelope["Message-ID"].strip()
    mail.headers = {
        key: value for key, value in envelope.items()
    }
    mail.envelope = envelope.as_string(unixfrom=True)
    mail.list_id = dest.id
    for part in envelope.walk():
        if part.is_multipart():
            continue
        content_type = part.get_content_type()
        [charset] = part.get_charsets("utf-8")
        # TODO: Verify signed emails
        if content_type == 'text/plain':
            # TODO: should we consider multiple text parts?
            mail.body = part.get_payload(decode=True).decode(charset)
            break

    # force lazy parse of patch (if it exists) after msg body is set
    mail.patch()

    mail.is_request_pull = False # TODO: Detect git request-pull
    reply_to = envelope["In-Reply-To"]
    if reply_to:
        reply_to = reply_to.strip()
        if "(" in reply_to:
            # Strip out obsolete In-Reply-To syntax used by e.g. gnus
            reply_to = reply_to.split("(")[0].rstrip()

    db.session.add(mail)
    db.session.flush() # obtain an ID for this email

    # Set parent of this email
    parent = Email.query.filter(Email.message_id == reply_to,
            Email.list_id == dest.id).one_or_none()
    if parent is not None:
        mail.parent_id = parent.id
        mail.parent = parent
        mail.in_reply_to = reply_to

    thread = mail
    while thread.parent_id:
        thread = thread.parent

    if thread.id != mail.id:
        mail.thread_id = thread.id

    # Reparent emails that arrived out-of-order
    children = Email.query.filter(Email.in_reply_to == mail.message_id,
            Email.list_id == dest.id).all()
    ex_threads = set()
    for child in children:
        child.parent_id = mail.id
        if child.thread_id != thread.id:
            ex_threads.update({ child.thread_id })
        child.thread_id = thread.id
    (Email.__table__.update().where(Email.thread_id in ex_threads)
            .values(thread_id=thread.id))

    db.session.flush()
    # Update thread nreplies & nparticipants
    thread.nreplies = 0
    thread_members = [thread]
    participants = set()
    for current in Email.query.filter(Email.thread_id == thread.id):
        thread_members.append(current)
        tenvelope = email.message_from_string(current.envelope, policy=policy)
        participants.update({ a for a in tenvelope["From"].split(",") })
        thread.nreplies += 1
    thread.nparticipants = len(participants)

    if not mail in thread_members:
        thread.append(mail)
    if mail.is_patch:
        _import_patch(thread_members, mail, envelope)

    # TODO: Enumerate CC's and create SQL relationships for them
    # TODO: Some users will have many email addresses
    sender = parseaddr(envelope["From"])
    sender = User.query.filter(User.email == sender[1]).one_or_none()
    if sender:
        mail.sender_id = sender.id
    print("Archived {} with ID {}".format(mail.subject, mail.id))
    return mail

def _webhooks(dest, mail):
    from listssrht.webhooks import UserWebhook, ListWebhook
    ListWebhook.deliver(ListWebhook.Events.post_received, mail.to_dict(),
            ListWebhook.Subscription.list_id == dest.id)
    if mail.sender:
        UserWebhook.deliver(UserWebhook.Events.email_received,
                mail.to_dict(),
                UserWebhook.Subscription.user_id == mail.sender.id)

def _subscribe(dest, mail):
    sender = parseaddr(mail["From"])
    user = User.query.filter(User.email == sender[1]).one_or_none()
    if user:
        perms = dest.account_permissions
        sub = Subscription.query.filter(
            Subscription.list_id == dest.id,
            Subscription.user_id == user.id).one_or_none()
    else:
        perms = dest.nonsubscriber_permissions
        sub = Subscription.query.filter(
            Subscription.list_id == dest.id,
            Subscription.email == sender[1]).one_or_none()

    list_addr = dest.owner.canonical_name + "/" + dest.name
    message = None

    # TODO: User-specific/email-specific overrides
    if ListAccess.browse not in perms:
        reply = MIMEText("""Hi {}!

We got your request to subscribe to {}, but unfortunately subscriptions to this
list are restricted. Your request has been disregarded.{}
        """.format(sender[0] or sender[1], list_addr, ("""

However, you are permitted to post mail to this list at this address:

{}@{}""".format(list_addr, cfg("lists.sr.ht", "posting-domain"))
        if ListAccess.post in perms else "")))
    elif sub is None:
        reply = MIMEText("""Hi {}!

Your subscription to {} is confirmed! To unsubscribe in the future, send an
email to this address:

{}+unsubscribe@{}

Feel free to reply to this email if you have any questions.""".format(
                sender[0] or sender[1], list_addr, list_addr,
                cfg("lists.sr.ht", "posting-domain")))
        sub = Subscription()
        sub.user_id = user.id if user else None
        sub.list_id = dest.id
        sub.email = sender[1] if not user else None
        db.session.add(sub)
    else:
        reply = MIMEText("""Hi {}!

We got an email asking to subscribe you to the {} mailing list. However, it
looks like you're already subscribed. To unsubscribe, send an email to:

{}+unsubscribe@{}

Feel free to reply to this email if you have any questions.""".format(
                sender[0] or sender[1], list_addr, list_addr,
                cfg("lists.sr.ht", "posting-domain")))
    reply["To"] = mail["From"]
    reply["From"] = "mailer@" + cfg("lists.sr.ht", "posting-domain")
    reply["In-Reply-To"] = mail["Message-ID"]
    reply["Auto-Submitted"] = "auto-replied"
    reply["Subject"] = "Re: " + (
            mail.get("Subject") or "Your subscription request")
    reply["Reply-To"] = "{} <{}>".format(
            cfg("sr.ht", "owner-name"), cfg("sr.ht", "owner-email"))
    reply["Date"] = formatdate()
    reply["Message-ID"] = make_msgid()
    print(reply.as_string(unixfrom=True))
    smtp = start_smtp()
    smtp.send_message(reply, smtp_user, [sender[1]])
    smtp.quit()
    db.session.commit()

def _unsubscribe(dest, mail):
    sender = parseaddr(mail["From"])
    user = User.query.filter(User.email == sender[1]).one_or_none()
    if user:
        sub = Subscription.query.filter(
            Subscription.list_id == dest.id,
            Subscription.user_id == user.id).one_or_none()
    else:
        sub = Subscription.query.filter(
            Subscription.list_id == dest.id,
            Subscription.email == sender[1]).one_or_none()
    list_addr = dest.owner.canonical_name + "/" + dest.name
    message = None
    if sub is None:
        reply = MIMEText("""Hi {}!

We got your request to unsubscribe from {}, but we did not find a subscription
from your email. If you continue to receive undesirable emails from this list,
please reply to this email for support.""".format(
                sender[0] or sender[1], list_addr))
    else:
        db.session.delete(sub)
        reply = MIMEText("""Hi {}!

You have been successfully unsubscribed from the {} mailing list. If you wish to
re-subscribe, send an email to:

{}+subscribe@{}

Feel free to reply to this email if you have any questions.""".format(
                sender[0] or sender[1], list_addr, list_addr,
                cfg("lists.sr.ht", "posting-domain")))
    reply["To"] = mail["From"]
    reply["From"] = "mailer@" + cfg("lists.sr.ht", "posting-domain")
    reply["In-Reply-To"] = mail["Message-ID"]
    reply["Auto-Submitted"] = "auto-replied"
    reply["Subject"] = "Re: " + (
            mail.get("Subject") or "Your subscription request")
    reply["Reply-To"] = "{} <{}>".format(
            cfg("sr.ht", "owner-name"), cfg("sr.ht", "owner-email"))
    reply["Date"] = formatdate()
    reply["Message-ID"] = make_msgid()
    print(reply.as_string(unixfrom=True))
    smtp = start_smtp()
    smtp.send_message(reply, smtp_user, [sender[1]])
    smtp.quit()
    db.session.commit()

def _configure_mirror(ml, mirror, mail):
    print("Message from mirror upstream mail server: ",
            mail.as_string(unixfrom=True))

    sender = parseaddr(mail["From"])
    mirror.mailer_sender = sender[1]
    reply_to = mail["Reply-To"]
    if not reply_to:
        mirror.configured = True
        db.session.commit()
        return

    if mirror.configure_attempts > 2:
        # Contact support
        return
    mirror.configure_attempts += 1

    list_name = "{}/{}".format(ml.owner.canonical_name, ml.name)

    posting_domain = cfg("lists.sr.ht", "posting-domain")
    reply = MIMEText(f"Confirming subscription request for {posting_domain}"
        f"on behalf of {ml.owner.canonical_name}\n\n"
        "If this email is unexpected, feel free to ignore it, or send "
        "questions to:\n\n"
        f"{cfg('sr.ht', 'owner-name')} <{cfg('sr.ht', 'owner-email')}>")
    reply["To"] = reply_to
    reply["From"] = f"{posting_domain} mirror <{list_name}@{posting_domain}>"
    reply["In-Reply-To"] = mail["Message-ID"]
    reply["Auto-Submitted"] = "auto-replied"
    reply["Subject"] = "Re: " + (mail.get("Subject") or "subscribe")
    reply["Date"] = formatdate()
    reply["Message-ID"] = make_msgid()
    reply["X-Mirroring-To"] = posting_domain

    smtp = start_smtp()
    smtp.send_message(reply, smtp_user, [sender[1]])
    smtp.quit()
    db.session.commit()

def _mirror(ml, mail):
    # TODO: disallow mail from any mail server other than the one being mirrored
    # TODO TODO: deal with the mirror's mail server changing addresses
    mirror = ml.mirror
    sender = parseaddr(mail["From"])
    if not mirror.configured or sender[1] == mirror.mailer_sender:
        return _configure_mirror(ml, mirror, mail)

    list_subscribe = mail["List-Subscribe"]
    list_unsubscribe = mail["List-Unsubscribe"]
    list_post = mail["List-Post"]

    updated = False
    if list_subscribe:
        if list_subscribe.startswith("<") and list_subscribe.endswith(">"):
            list_subscribe = list_subscribe[1:-1]
        mirror.list_subscribe = list_subscribe
    if list_unsubscribe:
        if list_unsubscribe.startswith("<") and list_unsubscribe.endswith(">"):
            list_unsubscribe = list_unsubscribe[1:-1]
        mirror.list_unsubscribe = list_unsubscribe
    if list_post:
        if list_post.startswith("<") and list_post.endswith(">"):
            list_post = list_post[1:-1]
        mirror.list_post = list_post

    return _archive(ml, mail)

@dispatch.task
def dispatch_message(address, list_id, mail):
    address = address[:address.rfind("@")]
    command = "post"
    if "+" in address:
        command = address[address.rfind("+") + 1:].lower()
        address = address[:address.rfind("+")]
    dest = List.query.filter(List.id == list_id).one_or_none()
    mail = email.message_from_string(mail, policy=policy)

    autosub = mail.get("auto-submitted")
    if autosub == "auto-generated" or autosub == "auto-replied":
        return # disregard automatic emails like OOO replies

    try:
        if command == "post":
            msgid = mail.get("Message-ID").strip()
            if not msgid or Email.query.filter(
                    Email.message_id == msgid,
                    Email.list_id == dest.id).count():
                print("Dropping email due to duplicate message ID")
                return
            dest.updated = datetime.utcnow()

            list_id = mail.get("List-ID")
            if dest.mirror_id or list_id:
                archived = _mirror(dest, mail)
                _webhooks(dest, archived)
                db.session.commit()
            else:
                archived = _archive(dest, mail)
                _webhooks(dest, archived)
                db.session.commit()
                _forward(dest, mail)
        elif command == "subscribe":
            _subscribe(dest, mail)
        elif command == "unsubscribe":
            _unsubscribe(dest, mail)
    except:
        db.session.rollback()
        raise

@dispatch.task
def send_error_for(mail, error):
    # Instead of letting postfix send an unfriendly bounce message, for some
    # errors we send our own bounce message which is a little easier to
    # understand.
    mail = email.message_from_string(mail, policy=email.policy.SMTPUTF8)
    autosub = mail.get("auto-submitted")
    if autosub == "auto-generated" or autosub == "auto-replied":
        return # disregard automatic emails like OOO replies
    reply = MIMEText(error)
    posting_domain = cfg("lists.sr.ht", "posting-domain")
    reply["To"] = mail["From"]
    reply["From"] = "mailer@" + posting_domain
    reply["In-Reply-To"] = mail["Message-ID"]
    reply["Subject"] = "Re: " + (
            mail.get("Subject") or "Your recent email to " + posting_domain)
    reply["Reply-To"] = "{} <{}>".format(
            cfg("sr.ht", "owner-name"), cfg("sr.ht", "owner-email"))
    reply["Date"] = formatdate()
    reply["Message-ID"] = make_msgid()
    reply["Auto-Submitted"] = "auto-replied"
    print(reply.as_string(unixfrom=True))
    smtp = start_smtp()
    sender = parseaddr(mail["From"])
    autosub = mail.get("auto-submitted")
    smtp.send_message(reply, smtp_user, [sender[1]])
    smtp.quit()

@dispatch.task
def import_mbox(spool, list_id):
    ml = List.query.filter(List.id == list_id).one_or_none()
    if not ml:
        print(f"Warning: unable to import mbox for unknown list {list_id}")
        return
    with tempfile.NamedTemporaryFile() as f:
        f.write(base64.b64decode(spool.encode()))
        f.flush()
        try:
            factory = lambda f: email.message_from_bytes(f.read(), policy=policy)
            mbox = mailbox.mbox(f.name, factory=factory)
        except:
            print("Error opening this file. Is it in mbox format?")
            # TODO: tell the user?
            return

        db.session.skip_autoupdate = True  # we want to use dates from the mbox
        for msg in mbox.values():
            try:
                msg_id = msg.get("Message-ID")
                if not msg_id:
                    continue
                msg_id = msg_id.strip()
                existing = (Email.query
                        .filter(Email.message_id == msg_id)
                        .filter(Email.list_id == ml.id)).count()
                if existing != 0:
                    continue # Drop messages with a duplicate message ID
                mail = _archive(ml, msg)
                date = msg.get("Date")
                if not date:
                    continue
                date = parsedate_to_datetime(date)
                if not date:
                    continue
                date = date.replace(tzinfo=None)
                mail.created = date
                mail.updated = date
                db.session.commit()
            except:
                print(f"Skipping email {msg_id} due to exception")
                db.session.rollback()
                continue # plow on forward
    ml.import_in_progress = False
    db.session.commit()

@dispatch.task
def forward_thread(list_id, thread_id, recipient):
    thread = (Email.query
            .filter(or_(Email.thread_id == thread_id, Email.id == thread_id))
            .filter(Email.list_id == list_id)
            .order_by(Email.id)).all()
    if not thread:
        return
    dest = thread[0].list

    smtp = start_smtp()
    for message in thread:
        mail = email.message_from_string(message.envelope, policy=policy)
        mail = _prep_mail(dest, mail)
        try:
            smtp.send_message(mail, smtp_user, [recipient])
        except:
            continue

    smtp.quit()

@dispatch.task
def delete_list(list_id):
    from listssrht.webhooks import ListWebhook
    ml = List.query.filter(List.id == list_id).one_or_none()
    ListWebhook.deliver(ListWebhook.Events.list_delete,
            ml.to_dict(), ListWebhook.Subscription.list_id == ml.id)
    db.engine.execute(f"DELETE FROM list WHERE id = {ml.id};")
    db.session.commit()
