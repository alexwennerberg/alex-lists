package main

import (
	"database/sql"
	"html"
	"html/template"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/db"
)

type threadMessage struct {
	ID            int
	MessageID     string
	Subject       string
	SenderName    *string
	FromName      string
	FromAddress   string
	ParentID      *int
	ParentMsgID   string
	MessageDate   *time.Time
	Created       time.Time
	NReplies      int
	NParticipants int
	Body          string
	PatchsetID    *int

	list *listView
}

// mailto: for replying to this message, as archives.reply_to builds it.
func (m *threadMessage) ReplyTo() template.URL {
	subject := m.Subject
	if !strings.HasPrefix(strings.ToLower(subject), "re:") {
		subject = "Re: " + subject
	}
	params := url.Values{}
	params.Set("cc", m.FromHeader())
	params.Set("in-reply-to", m.MessageID)
	params.Set("subject", subject)
	return template.URL("mailto:" + m.list.PostAddress("") + "?" +
		strings.ReplaceAll(params.Encode(), "+", "%20"))
}

func (m *threadMessage) FromHeader() string {
	if m.FromName == "" {
		return m.FromAddress
	}
	return m.FromName + " <" + m.FromAddress + ">"
}

// The sender timestamp, printed as a whole number of seconds. The column has
// no time zone, and datetime.timestamp() reads a naive value as local time,
// so this has to as well or the two apps disagree by the UTC offset.
func (m *threadMessage) SenderTimestamp() string {
	if m.MessageDate == nil {
		return ""
	}
	d := *m.MessageDate
	local := time.Date(d.Year(), d.Month(), d.Day(), d.Hour(), d.Minute(),
		d.Second(), d.Nanosecond(), time.Local)
	return strconv.FormatInt(local.Unix(), 10)
}

// href="#<message-id>", HTML-escaped rather than URL-escaped: Jinja
// interpolates the raw id and escapes it as text, and html/template would
// percent-encode the angle brackets instead.
func (m *threadMessage) Anchor() template.HTMLAttr {
	return template.HTMLAttr(`href="#` + html.EscapeString(m.MessageID) + `"`)
}

func (m *threadMessage) ParentAnchor() template.HTMLAttr {
	return template.HTMLAttr(`href="#` + html.EscapeString(m.ParentMsgID) + `"`)
}

func (m *threadMessage) FormatBody() template.HTML { return formatBody(m.Body) }

type threadPage struct {
	List     *listView
	Thread   *threadMessage
	Messages []*threadMessage
}

func handleThread(w http.ResponseWriter, r *http.Request) {
	list, id, ok := lookupMessage(w, r)
	if !ok {
		return
	}

	var (
		threadID *int
		rootMsg  string
		data     = threadPage{List: list}
	)
	err := db.WithReadOnlyTx(r.Context(), func(tx *sql.Tx) error {
		row := tx.QueryRowContext(r.Context(), `
			SELECT e.thread_id, root.message_id
			FROM email e
			LEFT JOIN email root ON root.id = e.thread_id
			WHERE e.id = $1;
		`, id)
		var rootID *string
		if err := row.Scan(&threadID, &rootID); err != nil {
			return err
		}
		if threadID != nil {
			if rootID != nil {
				rootMsg = *rootID
			}
			return nil
		}

		rows, err := tx.QueryContext(r.Context(), `
			SELECT e.id, e.message_id, e.subject, u.username,
				e.headers->'From'->>0, e.parent_id, parent.message_id,
				e.message_date, e.created, e.nreplies, e.nparticipants,
				e.body, e.patchset_id
			FROM email e
			LEFT JOIN "user" u ON u.id = e.sender_id
			LEFT JOIN email parent ON parent.id = e.parent_id
			WHERE e.id = $1 OR e.thread_id = $1
			ORDER BY e.created, e.id;
		`, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			msg := threadMessage{list: list}
			var from, parentMsgID *string
			if err := rows.Scan(&msg.ID, &msg.MessageID, &msg.Subject,
				&msg.SenderName, &from, &msg.ParentID, &parentMsgID,
				&msg.MessageDate, &msg.Created, &msg.NReplies,
				&msg.NParticipants, &msg.Body, &msg.PatchsetID); err != nil {
				return err
			}
			if from != nil {
				msg.FromName, msg.FromAddress = parseAddress(*from)
			}
			if parentMsgID != nil {
				msg.ParentMsgID = *parentMsgID
			}
			if msg.ID == id {
				data.Thread = &msg
			} else {
				data.Messages = append(data.Messages, &msg)
			}
		}
		return rows.Err()
	})
	if err != nil {
		serverError(w, r, "thread", err)
		return
	}

	// A reply is shown in its thread, anchored at itself.
	if threadID != nil {
		http.Redirect(w, r, "/"+list.FullName()+"/"+
			url.PathEscape(rootMsg)+"#"+
			url.PathEscape(r.PathValue("messageID")),
			http.StatusFound)
		return
	}

	render(w, r, "thread", &data)
}

// email.utils.parseaddr: display name and address.
func parseAddress(header string) (string, string) {
	addr, err := mail.ParseAddress(header)
	if err != nil {
		return "", header
	}
	return addr.Name, addr.Address
}

// Plain text bodies, as listssrht.filters._format_plain renders them:
// quoted lines muted, everything escaped, wrapped in a pre.
//
// The patch and format=flowed renderers are not ported yet, so a patch body
// shows here as plain text rather than as a coloured diff.
func formatBody(body string) template.HTML {
	var out strings.Builder
	out.WriteString("<pre class='message-body'>")
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r", ""), "\n") {
		if strings.HasPrefix(line, ">") {
			out.WriteString("<span class='text-muted'>")
			out.WriteString(urlize(line))
			out.WriteString("</span>\n")
		} else {
			out.WriteString(urlize(line))
			out.WriteString("\n")
		}
	}
	return template.HTML(strings.TrimRight(out.String(), " \t\n") + "</pre>")
}

// A much simpler jinja2 urlize: bare http(s) URLs become links, everything
// else is escaped. Jinja also linkifies www. hosts and email addresses.
func urlize(line string) string {
	var out strings.Builder
	words := splitKeepSpace(line)
	for _, word := range words {
		trimmed := strings.TrimRight(word, ".,:;!?)")
		suffix := word[len(trimmed):]
		if strings.HasPrefix(trimmed, "http://") ||
			strings.HasPrefix(trimmed, "https://") {
			escaped := html.EscapeString(trimmed)
			out.WriteString(`<a href="` + escaped +
				`" rel="noopener nofollow">` + escaped + `</a>`)
			out.WriteString(html.EscapeString(suffix))
			continue
		}
		out.WriteString(html.EscapeString(word))
	}
	return out.String()
}

// Split into words while keeping the whitespace between them, so a line
// round-trips unchanged when nothing is linkified.
func splitKeepSpace(line string) []string {
	var out []string
	start := 0
	inSpace := false
	for i, r := range line {
		space := r == ' ' || r == '\t'
		if i > 0 && space != inSpace {
			out = append(out, line[start:i])
			start = i
		}
		inSpace = space
	}
	if start < len(line) {
		out = append(out, line[start:])
	}
	return out
}

func (m *threadMessage) CreatedPtr() *time.Time { return &m.Created }

// The last message in the thread is what "reply to thread" answers.
func (t *threadPage) ReplyTarget() *threadMessage {
	if len(t.Messages) > 0 {
		return t.Messages[len(t.Messages)-1]
	}
	return t.Thread
}

func (t *threadPage) PageNumber() int     { return 0 }
func (t *threadPage) PageCount() int      { return 1 }
func (t *threadPage) SearchTerms() string { return "" }

// What the display_message macro needs: the message, the page around it, and
// whether it is the thread root.
type messageContext struct {
	Page     *page
	Msg      *threadMessage
	IsThread bool
}

// Replies repeat the thread's subject with an Re: prefix; the macro only
// prints a heading when the subject actually differs.
func (m messageContext) StrippedDiffers() bool {
	thread, ok := m.Page.Content.(*threadPage)
	if !ok {
		return false
	}
	stripped := m.Msg.Subject
	if len(stripped) >= 4 && strings.EqualFold(stripped[:4], "re: ") {
		stripped = stripped[4:]
	}
	return stripped != thread.Thread.Subject
}
