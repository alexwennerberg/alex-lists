from flask import Blueprint, render_template, abort, request, redirect, url_for
from flask import current_app, session
from srht.config import cfg
from srht.database import db
from srht.flask import paginate_query
from srht.graphql import exec_gql, GraphQLOperation, GraphQLUpload
from srht.oauth import current_user, loginrequired
from srht.validation import Validation
from listssrht.blueprints.archives import get_list
from listssrht.types import Access, Email, List, ListAccess, User
from listssrht.webhooks import ListWebhook
import base64
import email

settings = Blueprint("settings", __name__)

access_help_map = {
    ListAccess.browse:
        "Permission to subscribe and browse the archives",
    ListAccess.reply:
        "Permission to reply to threads submitted by an authorized user.",
    ListAccess.post:
        "Permission to submit new threads.",
    ListAccess.moderate:
        "Permission to moderate threads and patches.",
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
    rewrite = lambda value: None if value == "" else value
    input = {
        key: rewrite(valid.source[key]) for key in [
            "description", "visibility"
        ] if valid.source.get(key) is not None
    }

    exec_gql(current_app.site, """
        mutation UpdateMailingList($id: Int!, $input: MailingListInput!) {
            updateMailingList(id: $id, input: $input) {
                id
            }
        }
    """, valid=valid, id=ml.id, input=input)

    if not valid.ok:
        return render_template("settings-info.html", ml=ml, owner=owner,
                access_type_list=ListAccess, access_help_map=access_help_map,
                view="info", **valid.kwargs)

    return redirect(url_for("settings.info_GET",
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
    access = _process_access(valid, "default")
    input = {
        perm: ((access & ListAccess[perm].value) != 0) for perm in [
            "browse", "reply", "post", "moderate",
        ]
    }

    exec_gql(current_app.site, """
        mutation UpdateMailingListACL($id: Int!, $input: ACLInput!) {
            updateMailingListACL(listID: $id, input: $input) {
                id
            }
        }
    """, valid=valid, id=ml.id, input=input)

    if not valid.ok:
        return render_template("settings-access.html", view="access",
                ml=ml, owner=owner, access_type_list=ListAccess,
                access_help_map=access_help_map, **valid.kwargs)

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
        # TODO: Figure out if we can associate emails with users for users we
        # haven't seen yet
        user = User.query.filter(User.email == username).one_or_none()
    else:
        user = current_app.oauth_service.lookup_user(username)
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
    if ListAccess.browse in ml.default_access:
        acl.permissions |= ListAccess.browse
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
    rewrite = lambda value: [] if value == "" else value.split(",")
    input = {
        key: rewrite(valid.source[key]) for key in [
            "permitMime", "rejectMime"
        ] if valid.source.get(key) is not None
    }

    exec_gql(current_app.site, """
        mutation UpdateMailingList($id: Int!, $input: MailingListInput!) {
            updateMailingList(id: $id, input: $input) {
                id
            }
        }
    """, valid=valid, id=ml.id, input=input)

    if not valid.ok:
        return render_template("settings-content.html",
                view="content", ml=ml, owner=owner,
                **valid.kwargs)

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

    op = GraphQLOperation("""
        mutation ImportMailingListSpool($listID: Int!, $spool: Upload!) {
            importMailingListSpool(listID: $listID, spool: $spool)
        }
    """)

    spool = GraphQLUpload(
        spool.filename,
        spool.stream,
        "application/octet-stream",
    )
    op.var("listID", ml.id)
    op.var("spool", spool)
    op.execute("lists.sr.ht", valid=valid)

    if not valid.ok:
        return render_template("settings-import-export.html",
                view="import/export", ml=ml, owner=owner, **valid.kwargs)

    return redirect(url_for("archives.archive",
        owner_name=owner_name, list_name=list_name))

@settings.route("/<owner_name>/<list_name>/settings/delete")
@loginrequired
def delete_GET(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)
    return render_template("settings-delete.html",
            view="delete", ml=ml, owner=owner)

@settings.route("/<owner_name>/<list_name>/settings/delete", methods=["POST"])
@loginrequired
def delete_POST(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)
    exec_gql("lists.sr.ht", """
        mutation DeleteMailingList($id: Int!) {
            deleteMailingList(id: $id) { id }
        }
    """, user=owner, id=ml.id)
    session["notice"] = f"{ml.name} is being deleted. This may take a few minutes."
    return redirect(url_for("user.index"))
