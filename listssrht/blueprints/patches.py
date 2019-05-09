import email
from email.utils import parseaddr
from emailthreads import parse as parse_thread
from flask import Blueprint, render_template, abort, Response
from listssrht.blueprints.archives import get_list
from listssrht.filters import post_address
from listssrht.types import List, User, Email, Patchset, ListAccess
from sqlalchemy import or_
from urllib.parse import quote, urlencode

patches = Blueprint("patches", __name__)

@patches.route("/<owner_name>/<list_name>/patches")
def patchlist(owner_name, list_name):
    owner, ml, access = get_list(owner_name, list_name)
    if not ml:
        abort(404)
    if ListAccess.browse not in access:
        abort(403)
    # TODO

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
            parseaddr=parseaddr, reply_to=reply_to, ml=ml,
            thread=thread, patchset=patchset, patches=patches,
            feedback=feedback, gen_cover_letter=gen_cover_letter)

def format_mbox(msg):
    b = bytes()
    if msg.is_patch:
        parsed = msg.parsed()
        b += parsed.as_bytes(unixfrom=True) + b'\r\n'
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
