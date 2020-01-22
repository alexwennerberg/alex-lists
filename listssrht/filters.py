from jinja2 import Markup, escape
from jinja2.filters import urlize
from srht.config import cfg

def post_address(ml, suffix=""):
    if ml.mirror_id:
        if suffix == "+subscribe":
            return ml.mirror.list_subscribe
        elif suffix == "+unsubscribe":
            return ml.mirror.list_unsubscribe
        elif suffix == "":
            return ml.mirror.list_post
        else:
            return None

    domain = cfg("lists.sr.ht", "posting-domain")
    return "{}/{}{}@{}".format(
            ml.owner.canonical_name, ml.name, suffix, domain)

def _format_patch(msg, limit=None):
    text = Markup("")
    is_diff = False

    # Predict the starting lines of each file name
    patch = msg.patch()
    old_files = {delta.old_file.path for delta in patch.deltas}
    new_files = {delta.new_file.path for delta in patch.deltas}
    file_lines = {
        f" {p} ": p for p in old_files | new_files
    }
    line_no = 0

    for line in msg.body.replace("\r", "").split("\n"):
        line_no += 1
        if line_no == limit:
            text = text.rstrip()
            text += Markup(
                "\n<span class='text-muted'>[message trimmed]</span>"
            )
            break
        if not is_diff:
            f = next((
                key for key in file_lines.keys() if line.startswith(key)
            ), None)
            if f != None:
                f = file_lines[f]
                text += Markup(" <a href='#{}'>{}</a>".format(
                    escape(msg.message_id) + "+" + escape(f), escape(f)))
                try:
                    stat = line[line.rindex(" ") + 1:]
                    line = line[:line.rindex(" ") + 1]
                    if "+" in stat and "-" in stat:
                        removed = stat[stat.index("-"):]
                        added = stat[:stat.index("-")]
                        stat = Markup(("<span class='text-success'>{}</span>" +
                            "<span class='text-danger'>{}</span>"
                        ).format(escape(added), escape(removed)))
                    elif "-" in stat:
                        stat = Markup(
                                "<span class='text-danger'>{}</span>".format(
                                    escape(stat)))
                    elif "+" in stat:
                        stat = Markup(
                                "<span class='text-success'>{}</span>".format(
                                    escape(stat)))
                    else:
                        stat = escape(stat)
                except ValueError:
                    stat = Markup("")
                text += escape(line[len(f) + 1:])
                text += escape(stat)
                text += Markup("\n")
            else:
                text += escape(line + "\n")
            if line.startswith("diff"):
                is_diff = True
        else:
            if line.strip() == "--":
                text += escape(line + "\n")
            elif line.startswith("+++"):
                path = line[4:].lstrip("b/")
                if f" {path} " in file_lines:
                    text += (
                        Markup("<a href='#{0}' id='{0}' class='text-info'>".format(
                            escape(msg.message_id) + "+" + escape(path)
                        ))
                        + escape(line)
                        + Markup("</a>\n"))
                    continue
            elif line.startswith("---"):
                text += (
                    Markup("<span class='text-info'>")
                    + escape(line)
                    + Markup("</span>\n"))
                continue
            elif line.startswith("+"):
                text += (
                    Markup("<span class='text-success'>")
                    + Markup(
                        ("<a aria-hidden='true' class='lineno text-success' " +
                        "href='#{0}-{1}' id='{0}-{1}'>+</a>").format(
                            escape(msg.message_id), line_no))
                    + escape(line[1:])
                    + Markup("</span>\n"))
            elif line.startswith("-"):
                text += (
                    Markup("<span class='text-danger'>")
                    + Markup(
                        ("<a aria-hidden='true' class='lineno text-danger' " +
                        "href='#{0}-{1}' id='{0}-{1}'>-</a>").format(
                            escape(msg.message_id), line_no))
                    + escape(line[1:])
                    + Markup("</span>\n"))
            elif line.startswith(" "):
                text += (
                    Markup("<a aria-hidden='true' " +
                        "class='lineno' href='#{0}-{1}'" +
                        "id='{0}-{1}'> </a>".format(
                            escape(msg.message_id), line_no))
                    + escape(line[1:] + "\n"))
            else:
                text += escape(line + "\n")
    return text.rstrip()

def format_body(msg, limit=None):
    if msg.patch() is not None:
        return _format_patch(msg, limit)
    text = Markup("")
    line_no = 0
    body = urlize(msg.body, rel="noopener nofollow")
    for line in msg.body.replace("\r", "").split("\n"):
        line_no += 1
        if line_no == limit:
            break
        if line.startswith(">"):
            text += (
                Markup("<span class='text-muted'>")
                    + Markup(urlize(escape(line), rel="noopener nofollow"))
                + Markup("</span>\n"))
        else:
            text += Markup(urlize(escape(line), rel="noopener nofollow")) + "\n"
    return text.rstrip()

def diffstat(patch_email):
    stats = patch_email.patch().stats
    return type("diffstat", tuple(), {
        "added": stats.insertions,
        "removed": stats.deletions,
    })
