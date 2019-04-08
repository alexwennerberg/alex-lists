from flask import Blueprint, render_template, abort, request, redirect, url_for
from flask import send_file, session
from flask_login import current_user
from srht.config import cfg
from srht.database import db
from srht.flask import paginate_query, loginrequired
from srht.validation import Validation
from listssrht.blueprints.archives import get_list
from listssrht.types import Access, Email, List, ListAccess, User
from listssrht.process import import_mbox
from listssrht.webhooks import ListWebhook
import base64
import email
import hashlib
import mailbox
import os

settings = Blueprint("settings", __name__)

access_help_map = {
    ListAccess.browse:
        "Permission to subscribe and browse the archives",
    ListAccess.reply:
        "Permission to reply to threads submitted by an authorized user.",
    ListAccess.post:
        "Permission to submit new threads."
}

@settings.route("/<owner_name>/<list_name>/settings/info")
@loginrequired
def info_GET(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)
    return render_template("settings-info.html", view="info",
            ml=ml, owner=owner, access_type_list=ListAccess,
            access_help_map=access_help_map)

@settings.route("/<owner_name>/<list_name>/settings/info", methods=["POST"])
@loginrequired
def info_POST(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)

    valid = Validation(request)
    list_desc = valid.optional("list_desc")
    if list_desc == "":
        list_desc = None
    valid.expect(not list_desc or len(list_desc) < 2048,
            "Description must be between 16 and 2048 characters.",
            field="list_desc")

    if not valid.ok:
        return render_template("settings-info.html", list=ml, owner=owner,
                access_type_list=ListAccess, access_help_map=access_help_map,
                view="info", **valid.kwargs)

    ml.description = list_desc
    ListWebhook.deliver(ListWebhook.Events.list_update,
            ml.to_dict(), ListWebhook.Subscription.list_id == ml.id)
    db.session.commit()
    return redirect(url_for("archives.archive",
        owner_name=owner_name, list_name=list_name))

@settings.route("/<owner_name>/<list_name>/settings/access")
@loginrequired
def access_GET(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)
    return render_template("settings-access.html", view="access",
            ml=ml, owner=owner, access_type_list=ListAccess,
            access_help_map=access_help_map)

def _process_access(valid, perm):
    bitfield = ListAccess.none
    for access in ListAccess:
        if access in [ListAccess.none]:
            continue
        if valid.optional("perm_{}_{}".format(
                perm, access.name)) != None:
            bitfield |= access
    return bitfield

@settings.route("/<owner_name>/<list_name>/settings/access", methods=["POST"])
@loginrequired
def access_POST(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)

    valid = Validation(request)

    ml.nonsubscriber_permissions = _process_access(valid, "nonsub")
    ml.subscriber_permissions = _process_access(valid, "sub")
    ml.account_permissions = _process_access(valid, "account")
    if ListAccess.browse in ml.nonsubscriber_permissions:
        ml.subscriber_permissions |= ListAccess.browse
        ml.account_permissions |= ListAccess.browse

    ListWebhook.deliver(ListWebhook.Events.list_update,
            ml.to_dict(), ListWebhook.Subscription.list_id == ml.id)
    db.session.commit()
    return redirect(url_for("settings.access_GET",
        owner_name=owner_name, list_name=list_name))

@settings.route("/<owner_name>/<list_name>/settings/acl", methods=["POST"])
@loginrequired
def acl_POST(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)

    valid = Validation(request)

    username = valid.require("user")
    if not valid.ok:
        return render_template("settings-access.html", view="access",
                ml=ml, owner=owner, access_type_list=ListAccess,
                access_help_map=access_help_map, hide_global=True,
                **valid.kwargs)
    if username.startswith("~"):
        username = username[1:]

    if "@" in username:
        user = User.query.filter(User.email == username).one_or_none()
    else:
        user = User.query.filter(User.username == username).one_or_none()
        valid.expect(user, "User not found", field="user")

    if not valid.ok:
        return render_template("settings-access.html", view="access",
                ml=ml, owner=owner, access_type_list=ListAccess,
                access_help_map=access_help_map, hide_global=True,
                **valid.kwargs)

    # Edit existing ACL entry if present
    if user:
        acl = (Access.query
                .filter(Access.list_id == ml.id)
                .filter(Access.user_id == user.id)
            ).one_or_none()
    else:
        acl = (Access.query
                .filter(Access.list_id == ml.id)
                .filter(Access.email == username)
            ).one_or_none()

    if not acl:
        acl = Access()
        acl.list_id = ml.id
        if user:
            acl.user_id = user.id
        else:
            acl.email = username
    acl.permissions = _process_access(valid, "acl")
    db.session.add(acl)
    db.session.commit()
    return redirect(url_for("settings.access_GET",
        owner_name=owner_name, list_name=list_name))

@settings.route("/<owner_name>/<list_name>/settings/acl/<int:acl_id>/delete",
        methods=["POST"])
@loginrequired
def acl_delete_POST(owner_name, list_name, acl_id):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)
    acl = Access.query.filter(Access.id == acl_id).one_or_none()
    if not acl:
        abort(404)
    if acl.list_id != ml.id:
        abort(403)
    db.session.delete(acl)
    db.session.commit()
    return redirect(url_for("settings.access_GET",
        owner_name=owner_name, list_name=list_name))

@settings.route("/<owner_name>/<list_name>/settings/content")
@loginrequired
def content_GET(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)
    return render_template("settings-content.html",
            view="content", ml=ml, owner=owner)

@settings.route("/<owner_name>/<list_name>/settings/content", methods=["POST"])
@loginrequired
def content_POST(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)

    valid = Validation(request)
    ml.permit_mimetypes = valid.optional("permit_mimetypes")
    ml.reject_mimetypes = valid.optional("reject_mimetypes")

    db.session.commit()
    return redirect(url_for("settings.content_GET",
        owner_name=owner_name, list_name=list_name))

@settings.route("/<owner_name>/<list_name>/settings/import-export")
@loginrequired
def import_export_GET(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)
    return render_template("settings-import-export.html",
            view="import/export", ml=ml, owner=owner)

@settings.route("/<owner_name>/<list_name>/settings/export", methods=["POST"])
@loginrequired
def export_POST(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)

    sha = hashlib.sha256()
    sha.update(f"{owner_name}/{list_name}".encode())
    digest = sha.hexdigest()
    export_dir = cfg('lists.sr.ht', 'export-directory', default='/tmp') 
    path = f"/{export_dir}/{digest}.mbox"

    mbox = mailbox.mbox(path)
    for message in (Email.query
            .filter(Email.list_id == ml.id)
            .order_by(Email.created)).all():
        try:
            mbox.add(message.parsed())
        except:
            continue # plow on forward
    mbox.close()

    f = open(path, "rb")
    os.unlink(path)
    return send_file(f, as_attachment=True,
            attachment_filename=f"{owner.username}-{list_name}.mbox")

@settings.route("/<owner_name>/<list_name>/settings/import", methods=["POST"])
@loginrequired
def import_POST(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)
    if ml.import_in_progress:
        abort(400)

    spool = request.files.get("spool")
    valid = Validation(request)
    valid.expect(spool is not None, "Mail spool is required", field="spool")

    if not valid.ok:
        return render_template("settings-import-export.html",
                view="import/export", ml=ml, owner=owner, **valid.kwargs)

    spool = spool.stream.read()
    spool = base64.b64encode(spool).decode()
    import_mbox.delay(spool, ml.id)

    ml.import_in_progress = True
    db.session.commit()
    return redirect(url_for("archives.archive",
        owner_name=owner_name, list_name=list_name))
