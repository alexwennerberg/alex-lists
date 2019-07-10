import email
from email import policy
from email.utils import parseaddr
from emailthreads import parse as parse_thread
from flask import Blueprint, render_template, abort, Response, request, redirect
from flask import url_for
from flask_login import current_user
from listssrht.blueprints.archives import get_list, apply_search
from listssrht.filters import post_address
from listssrht.types import List, Email, Patchset, PatchsetStatus, ListAccess
from listssrht.types import Subscription
from sqlalchemy import or_
from srht.database import db
from srht.flask import loginrequired, paginate_query
from srht.validation import Validation
from urllib.parse import quote, urlencode

patches = Blueprint("patches", __name__)

status_to_color = {
    PatchsetStatus.proposed: "text-info",
    PatchsetStatus.needs_revision: "text-warning",
    PatchsetStatus.superseded: "text-muted",
    PatchsetStatus.approved: "text-success",
    PatchsetStatus.rejected: "text-danger",
    PatchsetStatus.applied: "text-muted"
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
    threads, search = apply_search(threads)
    threads, pagination = paginate_query(threads)

    subscription = None
    if current_user:
        subscription = (Subscription.query
                .filter(Subscription.list_id == ml.id)
                .filter(Subscription.user_id == current_user.id)).one_or_none()
    return render_template("archive.html",
            view="patches", owner=owner, ml=ml, threads=threads,
            access=access, ListAccess=ListAccess, search=search,
            subscription=subscription, status_to_color=status_to_color,
            parseaddr=parseaddr, PatchsetStatus=PatchsetStatus,
            **pagination)

def _parse_thread(thread):
    parsed = parse_thread(thread)
    feedback_by_line = {}
    standalone_feedback = []
    for c in parsed.children:
        if c.index is not None and c.index < len(parsed.lines):
            if c.index not in feedback_by_line:
                feedback_by_line[c.index] = [c]
            else:
                feedback_by_line[c.index].append(c)
        else:
            standalone_feedback.append(c)
    parsed.standalone_feedback = standalone_feedback
    parsed.feedback_by_line = feedback_by_line
    return parsed

def gen_cover_letter(patches):
    cover = ""
    authors = {}
    for patch in patches:
        addr = parseaddr(patch.headers["From"])
        authors.setdefault(addr[0], list())
        authors[addr[0]].append(patch)
    # TODO: generate file changes as well
    for author in sorted(authors.keys()):
        patches = authors[author]
        cover += f"{author}: {len(patches)}\n"
        nfiles = 0
        insertions = deletions = 0
        for email in patches:
            cover += f" {email.patch_subject}\n"
            patch = email.patch()
            nfiles += (len(patch.added_files)
                    + len(patch.modified_files)
                    + len(patch.removed_files))
            insertions += sum(f.added for
                    f in patch.added_files + patch.modified_files)
            deletions += sum(f.removed
                    for f in patch.removed_files + patch.modified_files)
    cover += f"\n {nfiles} files changed, {insertions} insertions(+), {deletions} deletions(-)\n"
    return cover

@patches.route("/<owner_name>/<list_name>/patches/<patchset_id>")
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
    thread = thread.thread if thread.thread_id else thread
    patches = (Email.query
            .filter(or_(Email.thread_id == thread.id, Email.id == thread.id))
            .filter(Email.is_patch)
            .order_by(Email.patch_index, Email.created)).all()
    feedback = dict()
    for msg in [thread] + thread.descendants:
        feedback[msg.id] = _parse_thread(
                [m.parsed() for m in [msg] + msg.replies])

    def reply_to(msg):
        params = {
            "cc": msg.parsed()['From'],
            "in-reply-to": msg.message_id,
            "subject": (f"Re: {msg.subject}"
                if not msg.subject.lower().startswith("re:")
                else msg.subject),
        }
        return f"mailto:{post_address(msg.list)}?{urlencode(params, quote_via=quote)}"

    return render_template("patchset.html", view="patches", owner=owner,
            parseaddr=parseaddr, reply_to=reply_to, ml=ml, access=access,
            thread=thread, patchset=patchset, patches=patches,
            feedback=feedback, gen_cover_letter=gen_cover_letter,
            PatchsetStatus=PatchsetStatus, status_to_color=status_to_color,
            max=max)

@patches.route("/<owner_name>/<list_name>/patches/<patchset_id>/update",
        methods=["POST"])
@loginrequired
def patchset_update(owner_name, list_name, patchset_id):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ml.owner_id != current_user.id:
        abort(403)
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
    if ml.owner_id != current_user.id:
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
        patchsets = (Patchset.query
                .filter(Patchset.list_id == ml.id)
                .join(Email, Patchset.cover_letter_id))
        patchsets, _ = apply_search(patchsets, terms=request.form.get("search"))
    else:
        patchsets = (Patchset.query
            .filter(Patchset.id.in_(selection))
            .filter(Patchset.list_id == ml.id))
    status = PatchsetStatus(request.form.get("status"))
    patchsets.update({ Patchset.status: status }, synchronize_session=False)
    db.session.commit()
    return redirect(url_for("patches.patchlist",
        owner_name=owner_name, list_name=list_name))

def format_mbox(msg):
    b = bytes()
    if msg.is_patch:
        parsed = msg.parsed()
        b += parsed.as_bytes(unixfrom=True,
                policy=email.policy.SMTPUTF8) + b'\r\n'
    for reply in msg.replies:
        if not reply.is_patch:
            continue
        b += format_mbox(reply)
    return b

@patches.route("/<owner_name>/<list_name>/patches/<patchset_id>/mbox")
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
    mbox = format_mbox(thread)
    return Response(mbox, mimetype='application/mbox')
