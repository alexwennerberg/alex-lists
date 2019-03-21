from jinja2 import Markup, escape
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

    def get_path(f):
        # [2:] to remove a/ or b/
        return f.target_file[2:].strip()

    # Predict the starting lines of each file name
    patch = msg.patch()
    file_lines = {
        f" {get_path(f)} ": f
        for f in patch.added_files + patch.modified_files + patch.removed_files
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
                    escape(msg.message_id) + "+" + escape(get_path(f)),
                    escape(get_path(f))))
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
                text += escape(line[len(get_path(f)) + 1:])
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
            if line.startswith("+"):
                text += (
                    Markup("<span class='text-success'>")
                    + Markup(
                        ("<a class='text-success' href='#{0}-{1}' " +
                        "id='{0}-{1}'>+</a>").format(
                            escape(msg.message_id), line_no))
                    + escape(line[1:])
                    + Markup("</span>\n"))
            elif line.startswith("-"):
                text += (
                    Markup("<span class='text-danger'>")
                    + Markup(
                        ("<a class='text-danger' href='#{0}-{1}' " +
                        "id='{0}-{1}'>-</a>").format(
                            escape(msg.message_id), line_no))
                    + escape(line[1:])
                    + Markup("</span>\n"))
            elif line.startswith(" "):
                text += (
                    Markup("<a href='#{0}-{1}' id='{0}-{1}'> </a>".format(
                            escape(msg.message_id), line_no))
                    + escape(line[1:] + "\n"))
            else:
                text += escape(line + "\n")
    return text.rstrip()

def format_body(msg, limit=None):
    if msg.is_patch:
        return _format_patch(msg, limit)
    text = Markup("")
    line_no = 0
    for line in msg.body.replace("\r", "").split("\n"):
        line_no += 1
        if line_no == limit:
            break
        if line.startswith(">"):
            text += (
                Markup("<span class='text-muted'>")
                    + escape(line)
                + Markup("</span>\n"))
        else:
            text += escape(line + "\n")
    return text.rstrip()

def diffstat(patch):
    p = patch.patch()
    return type("diffstat", tuple(), {
        "added": sum(f.added for f in p.added_files + p.modified_files),
        "removed": sum(f.removed for f in p.removed_files + p.modified_files),
    })
