import bleach
import email
from email import policy
from email.utils import parseaddr
from markupsafe import Markup
from flask import Blueprint, render_template, abort, Response, request, redirect
from flask import url_for, session
from listssrht.blueprints.archives import get_list, apply_search
from listssrht.filters import post_address
from listssrht.types import List, Email, Patchset, PatchsetStatus, ListAccess
from listssrht.types import Subscription, PatchsetTool, ToolIcon
from sqlalchemy import or_
from srht.app import paginate_query, get_projects
from srht.database import db
from srht.markdown import markdown
from srht.oauth import current_user, loginrequired
from srht.validation import Validation
from urllib.parse import quote, urlencode

patches = Blueprint("patches", __name__)

status_to_color = {
    PatchsetStatus.proposed: "text-info",
    PatchsetStatus.needs_revision: "text-warning",
    PatchsetStatus.superseded: "text-muted",
    PatchsetStatus.approved: "text-success",
    PatchsetStatus.rejected: "text-danger",
    PatchsetStatus.applied: "text-muted",
}

tool_icon_to_class = {
    ToolIcon.pending: "text-muted",
    ToolIcon.waiting: "text-info icon-spin",
    ToolIcon.success: "text-success",
    ToolIcon.failed: "text-danger",
    ToolIcon.cancelled: "text-warning",
}

tool_icon_to_icon = {
    ToolIcon.pending: "minus",
    ToolIcon.waiting: "circle-notch",
    ToolIcon.success: "check",
    ToolIcon.failed: "times",
    ToolIcon.cancelled: "times",
}

# Patch statuses that only moderators can transition patches to
MODERATOR_ONLY_STATUS = [ "applied", "approved", "rejected" ]

def project_nav(mailing_list):
    projects = get_projects(mailing_list.owner, mailing_list.rid)
    return {
        "projects": projects,
        "owner": mailing_list.owner,
    }

@patches.context_processor
def inject():
    return {
        "status_to_color": status_to_color,
        "tool_icon_to_class": tool_icon_to_class,
        "tool_icon_to_icon": tool_icon_to_icon,
        "MODERATOR_ONLY_STATUS": MODERATOR_ONLY_STATUS,
    }

@patches.route("/<owner_name>/<list_name>/patches")
def patchlist(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.browse not in access:
        abort(403)
    threads = (Email.query
            .filter(Email.list_id == ml.id)
            .filter(Email.patchset_id != None)
            .filter(Email.parent_id == None)
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
    return render_template("archive.html", view="patches",
            ml=ml, threads=threads, access=access, search=search,
            search_error=search_error, subscription=subscription,
            **pagination, **project_nav(ml))

def gen_cover_letter(patches):
    cover = ""
    authors = {}
    for patch in patches:
        # git send-email passes the author information in:
        #  - The mail body's first line if it starts with "From: "
        #  - Otherwise, in the From: header
        # (see --from in https://git-scm.com/docs/git-format-patch)
        msg = patch.parsed()
        author = parseaddr(msg["From"])[0]
        if not msg.is_multipart():
            payload = patch.parsed().get_payload(decode=True)
            charset = msg.get_content_charset('utf-8')
            first_line = payload.splitlines()[0].decode(charset)
            if first_line.startswith("From: "):
                author = parseaddr(first_line)[0]
        authors.setdefault(author, list())
        authors[author].append(patch)
    # TODO: generate file changes as well
    nfiles = 0
    insertions = deletions = 0
    for author in sorted(authors.keys()):
        patches = authors[author]
        cover += f"{author}: {len(patches)}\n"
        for email in patches:
            cover += f" {email.patch_subject}\n"
            if email.patch():
                stats = email.patch().stats
                nfiles += stats.files_changed
                insertions += stats.insertions
                deletions += stats.deletions
    cover += f"\n {nfiles} files changed, {insertions} insertions(+), {deletions} deletions(-)\n"
    return cover

@patches.route("/<owner_name>/<list_name>/patches/<int:patchset_id>")
def patchset(owner_name, list_name, patchset_id):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.browse not in access:
        abort(403)
    patchset = (Patchset.query
            .filter(Patchset.id == patchset_id)
            .filter(Patchset.list_id == ml.id)).one_or_none()
    if not patchset:
        abort(404)
    thread = Email.query.filter(Email.patchset_id == patchset_id).first()
    assert thread, f"Patchset {patchset_id} found without any corresponding emails"
    thread = thread.thread if thread.thread_id else thread
    patches = (Email.query
            .filter(or_(Email.thread_id == thread.id, Email.id == thread.id))
            .filter(Email.is_patch)
            .order_by(Email.patch_index, Email.created)).all()
    # FORK: the thread's non-patch replies used to be rendered here as inline
    # quote attribution, sourced from query { patchset { thread { blocks } } }
    # and go-emailthreads in the API. Nothing parses threads on this side any
    # more, so the patchset page shows the patches only; the discussion lives
    # in the archive view linked from the sidebar.

    def reply_to(msg):
        params = {
            "cc": msg.parsed()['From'],
            "in-reply-to": msg.message_id,
            "subject": (f"Re: {msg.subject}"
                if not msg.subject.lower().startswith("re:")
                else msg.subject),
        }
        pa = post_address(msg.list)
        if pa.startswith("mailto:"):
            return f"{pa}?{urlencode(params, quote_via=quote)}"
        else:
            return f"mailto:{pa}?{urlencode(params, quote_via=quote)}"

    tools = (PatchsetTool.query
            .filter(PatchsetTool.patchset_id == patchset.id)
            .order_by(PatchsetTool.id)).all()

    tool_details = lambda d: Markup(bleach.sanitizer.Cleaner(
            tags=["code", "a", "strong", "em"],
            attributes={"a": ["href", "target", "rel"]},
            strip=True).clean(markdown(d, with_styles=False)))

    user_message = session.pop("message", None)
    return render_template("patchset.html", view="patch",
            parseaddr=parseaddr, reply_to=reply_to, ml=ml, access=access,
            thread=thread, patchset=patchset, patches=patches,
            gen_cover_letter=gen_cover_letter, max=max,
            user_message=user_message, tools=tools, tool_details=tool_details,
            **project_nav(ml))

@patches.route("/<owner_name>/<list_name>/patches/<int:patchset_id>/update",
        methods=["POST"])
@loginrequired
def patchset_update(owner_name, list_name, patchset_id):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    patchset = (Patchset.query
            .filter(Patchset.id == patchset_id)
            .filter(Patchset.list_id == ml.id)).one_or_none()
    if not patchset:
        abort(404)
    valid = Validation(request)
    status = valid.require("status", cls=PatchsetStatus)
    if not valid.ok:
        # not possible without end-user fuckery, so no pretty error for you
        abort(400)
    if ListAccess.moderate not in access:
        if not patchset.submitter or current_user.email not in patchset.submitter:
            abort(403)
    patchset.status = status
    db.session.commit()
    return redirect(url_for("patches.patchset", owner_name=owner_name,
        list_name=list_name, patchset_id=patchset_id))

@patches.route("/<owner_name>/<list_name>/patches/bulk-update", methods=["POST"])
@loginrequired
def patchset_bulk_update(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.moderate not in access:
        abort(403)
    select_all = False
    selection = []
    for item in request.form:
        if item == "select-all":
            select_all = True
            break
        if item.startswith("select-"):
            selection.append(int(item.split("-")[1]))
    if select_all:
        patchsets = Patchset.query.filter(Patchset.list_id == ml.id)
        patchsets = apply_search(patchsets, request.form.get("search"))
    else:
        patchsets = (Patchset.query
            .filter(Patchset.id.in_(selection))
            .filter(Patchset.list_id == ml.id))
    status = PatchsetStatus(request.form.get("status"))
    patchsets.update({ Patchset.status: status }, synchronize_session=False)
    db.session.commit()
    redirect_url_args = {
        "owner_name": owner_name,
        "list_name": list_name,
    }
    if request.form.get("search"):
        redirect_url_args["search"] = request.form.get("search")
    if request.form.get("page"):
        redirect_url_args["page"] = request.form.get("page")
    return redirect(url_for("patches.patchlist", **redirect_url_args))

def format_mbox(msgs):
    b = bytes()
    policy = email.policy.SMTPUTF8.clone(max_line_length=998)
    for msg in msgs:
        parsed = msg.parsed()
        b += parsed.as_bytes(unixfrom=True, policy=policy) + b'\r\n'
    return b

@patches.route("/<owner_name>/<list_name>/patches/<int:patchset_id>/mbox")
def mbox(owner_name, list_name, patchset_id):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.browse not in access:
        abort(403)
    patchset = (Patchset.query
            .filter(Patchset.id == patchset_id)
            .filter(Patchset.list_id == ml.id)).one_or_none()
    if not patchset:
        abort(404)
    thread = Email.query.filter(Email.patchset_id == patchset_id).first()
    thread = thread.thread if thread.thread_id else thread
    patches = (Email.query
            .filter(or_(Email.thread_id == thread.id, Email.id == thread.id))
            .filter(Email.is_patch)
            .order_by(Email.patch_index, Email.created)).all()
    try:
        mbox = format_mbox(patches)
    except UnicodeEncodeError:
        return Validation(request).error("Encoding error", status=500)
    return Response(mbox, mimetype='application/mbox')
