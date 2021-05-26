from flask import Blueprint, abort, request
from listssrht.blueprints.api import get_user, get_list
from listssrht.types import Email, List, ListAccess
from listssrht.types import Patchset, PatchsetStatus, PatchsetTool, ToolIcon
from srht.api import paginated_response
from srht.database import db
from srht.oauth import oauth, current_token
from srht.validation import Validation

patches = Blueprint("api_patches", __name__)

@patches.route("/api/user/<username>/lists/<list_name>/patchsets")
@patches.route("/api/lists/<list_name>/patchsets", defaults={"username": None})
@oauth("patches:read")
def list_patchsets_GET(username, list_name):
    user, ml, access = get_list(username, list_name)
    if not ListAccess.browse in access:
        abort(404)
    patches = Patchset.query.filter(Patchset.list_id == ml.id)
    return paginated_response(Patchset.id, patches)

@patches.route("/api/user/<username>/lists/<list_name>/patchsets/<patchset_id>")
@patches.route("/api/lists/<list_name>/patchsets/<patchset_id>",
        defaults={"username": None})
@oauth("patches:read")
def list_patchsets_by_id_GET(username, list_name, patchset_id):
    user, ml, access = get_list(username, list_name)
    if not ListAccess.browse in access:
        abort(404)
    patchset = (Patchset.query
            .filter(Patchset.list_id == ml.id)
            .filter(Patchset.id == patchset_id)).one_or_none()
    if not patchset:
        abort(404)
    return patchset.to_dict()

@patches.route("/api/user/<username>/lists/<list_name>/patchsets/<patchset_id>/patches")
@patches.route("/api/lists/<list_name>/patchsets/<patchset_id>/patches",
        defaults={"username": None})
@oauth("patches:read")
def list_patchsets_by_id_patches_GET(username, list_name, patchset_id):
    user, ml, access = get_list(username, list_name)
    if not ListAccess.browse in access:
        abort(404)
    patchset = (Patchset.query
            .filter(Patchset.list_id == ml.id)
            .filter(Patchset.id == patchset_id)).one_or_none()
    if not patchset:
        abort(404)
    patches = (Email.query
            .filter(Email.patchset_id == patchset.id)
            .filter(Email.is_patch))
    return paginated_response(Email.id, patches, order_by=(
        Email.patch_version, Email.patch_index, Email.created
    ))

@patches.route("/api/user/<username>/lists/<list_name>/patchsets/<patchset_id>",
        methods=["PUT"])
@patches.route("/api/lists/<list_name>/patchsets/<patchset_id>",
        defaults={"username": None}, methods=["PUT"])
@oauth("patches:write")
def list_patchsets_by_id_PUT(username, list_name, patchset_id):
    user, ml, access = get_list(username, list_name)
    if ml.owner_id != current_token.user_id:
        abort(401)
    patchset = (Patchset.query
            .filter(Patchset.list_id == ml.id)
            .filter(Patchset.id == patchset_id)).one_or_none()
    if not patchset:
        abort(404)
    # TODO: other stuff, like superseding another patch
    valid = Validation(request)
    patchset.status = valid.optional("status", cls=PatchsetStatus,
            default=patchset.status)
    if not valid.ok:
        return valid.response
    db.session.commit()
    return patchset.to_dict()

@patches.route("/api/user/<username>/lists/<list_name>/patchsets/<patchset_id>/tools", methods=["PUT"])
@patches.route("/api/lists/<list_name>/patchsets/<patchset_id>/tools",
        defaults={"username": None}, methods=["PUT"])
@oauth("patches:write")
def list_patchsets_by_id_tools_PUT(username, list_name, patchset_id):
    user, ml, access = get_list(username, list_name)
    if ml.owner_id != current_token.user_id:
        abort(401)
    patchset = (Patchset.query
            .filter(Patchset.list_id == ml.id)
            .filter(Patchset.id == patchset_id)).one_or_none()
    if not patchset:
        abort(404)
    valid = Validation(request)
    icon = valid.require("icon", cls=ToolIcon)
    details = valid.require("details")
    key = valid.require("key")
    if not valid.ok:
        return valid.response
    tool = (PatchsetTool.query
            .filter(PatchsetTool.patchset_id == patchset.id)
            .filter(PatchsetTool.key == key)).one_or_none()
    if not tool:
        tool = PatchsetTool()
        tool.patchset_id = patchset.id
        tool.key = key

    tool.icon = icon
    tool.details = details
    db.session.add(tool)
    db.session.commit()
    return {}
