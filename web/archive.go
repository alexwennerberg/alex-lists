package main

import (
	"database/sql"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/db"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/model"
	"github.com/google/uuid"
)

// A list as the archive pages need it, with the viewer's access resolved.
type listView struct {
	ID            int
	Owner         string
	Name          string
	Description   *string
	Visibility    string
	RID           string
	MirrorID      *int
	Importing     bool
	DefaultAccess int
	Access        *model.GeneralACL
	Subscribed    bool
	IsOwner       bool
}

func (l *listView) FullName() string { return "~" + l.Owner + "/" + l.Name }

func (l *listView) ResourceID() string { return toRID(l.RID) }

// The address mail is posted to, as listssrht.filters.post_address builds it.
// Mirrors carry their own addresses, which this does not handle yet.
func (l *listView) PostAddress(suffix string) string {
	return l.FullName() + suffix + "@" + conf("lists.sr.ht", "posting-domain")
}

// Resolve the list and what the viewer may do with it. A private list the
// viewer has no access to is a 404 here, as it is in the Python app.
func getList(r *http.Request, owner, name string) (*listView, error) {
	view := listView{Owner: owner, Name: name}
	user := userFor(r)
	err := db.WithReadOnlyTx(r.Context(), func(tx *sql.Tx) error {
		row := tx.QueryRowContext(r.Context(), `
			SELECT l.id, l.description, l.visibility, l.rid, l.mirror_id,
				l.import_in_progress, l.default_access, l.owner_id
			FROM list l JOIN "user" u ON u.id = l.owner_id
			WHERE u.username = $1 AND l.name = $2;
		`, owner, name)
		var ownerID int
		if err := row.Scan(&view.ID, &view.Description, &view.Visibility,
			&view.RID, &view.MirrorID, &view.Importing, &view.DefaultAccess,
			&ownerID); err != nil {
			return err
		}

		if user != nil {
			view.IsOwner = user.ID == ownerID
			row := tx.QueryRowContext(r.Context(), `
				SELECT count(*) FROM subscription
				WHERE list_id = $1 AND user_id = $2;
			`, view.ID, user.ID)
			var count int
			if err := row.Scan(&count); err != nil {
				return err
			}
			view.Subscribed = count > 0
		}

		var err error
		email := ""
		if user != nil {
			email = user.Email
		}
		view.Access, err = model.UserACL(r.Context(), tx, view.ID, email)
		return err
	})
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &view, err
}

type threadEntry struct {
	MessageID     string
	Subject       string
	PatchsetID    *int
	From          string
	SenderName    *string
	Updated       time.Time
	NReplies      int
	NParticipants int
}

type archivePage struct {
	List       *listView
	Threads    []threadEntry
	Search     string
	Page       int
	TotalPages int
}

func handleArchive(w http.ResponseWriter, r *http.Request) {
	owner, ok := strings.CutPrefix(r.PathValue("owner"), "~")
	if !ok {
		notFound(w, r)
		return
	}
	list, err := getList(r, owner, r.PathValue("list"))
	if err != nil {
		serverError(w, r, "archive", err)
		return
	}
	if list == nil {
		notFound(w, r)
		return
	}

	data := archivePage{
		List:   list,
		Search: r.URL.Query().Get("search"),
		Page:   pageParam(r),
	}

	if list.Access.Browse {
		if err := loadThreads(r, &data); err != nil {
			serverError(w, r, "archive", err)
			return
		}
	}

	render(w, r, "archive", &data)
}

// Thread roots, newest activity first. Without search terms only roots are
// listed; with them, every matching message is, which is what the Python
// app's apply_search does.
func loadThreads(r *http.Request, data *archivePage) error {
	where := `e.list_id = $1`
	args := []any{data.List.ID}
	if data.Search == "" {
		where += ` AND e.parent_id IS NULL`
	} else {
		where += ` AND (e.body ILIKE $2 OR e.subject ILIKE $2)`
		args = append(args, "%"+data.Search+"%")
	}

	return db.WithReadOnlyTx(r.Context(), func(tx *sql.Tx) error {
		row := tx.QueryRowContext(r.Context(),
			`SELECT count(*) FROM email e WHERE `+where, args...)
		var total int
		if err := row.Scan(&total); err != nil {
			return err
		}
		data.TotalPages = totalPages(total)

		rows, err := tx.QueryContext(r.Context(), `
			SELECT e.message_id, e.subject, e.patchset_id, e.headers->'From'->>0,
				u.username, e.updated, e.nreplies, e.nparticipants
			FROM email e
			LEFT JOIN "user" u ON u.id = e.sender_id
			WHERE `+where+`
			ORDER BY e.updated DESC
			LIMIT `+strconv.Itoa(perPage)+`
			OFFSET `+strconv.Itoa((data.Page-1)*perPage), args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var entry threadEntry
			var from *string
			if err := rows.Scan(&entry.MessageID, &entry.Subject,
				&entry.PatchsetID, &from, &entry.SenderName, &entry.Updated,
				&entry.NReplies, &entry.NParticipants); err != nil {
				return err
			}
			if from != nil {
				entry.From = displayName(*from)
			}
			data.Threads = append(data.Threads, entry)
		}
		return rows.Err()
	})
}

// email.utils.parseaddr(header)[0]: the display name, or "" when the header
// is a bare address.
func displayName(header string) string {
	addr, err := mail.ParseAddress(header)
	if err != nil {
		return ""
	}
	return addr.Name
}

// The headers column is JSON; a few call sites want a header out of it
// without reparsing the raw message.
func headerFrom(headers []byte, name string) string {
	var parsed map[string]any
	if err := json.Unmarshal(headers, &parsed); err != nil {
		return ""
	}
	value, _ := parsed[name].(string)
	return value
}

// srht's to_rid: crockford-ish base32 of the UUID's bytes, which is how
// resource IDs are shown and how hub.sr.ht links to them.
func toRID(id string) string {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return id
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).
		EncodeToString(parsed[:])
	return strings.Map(func(r rune) rune {
		const std = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
		const rid = "0123456789abcdefghjkmnpqrstvwxyz"
		if i := strings.IndexRune(std, r); i >= 0 {
			return rune(rid[i])
		}
		return r
	}, encoded)
}

func (a *archivePage) PageNumber() int     { return a.Page }
func (a *archivePage) PageCount() int      { return a.TotalPages }
func (a *archivePage) SearchTerms() string { return a.Search }

// The raw message, exactly as it was archived.
func handleRaw(w http.ResponseWriter, r *http.Request) {
	list, message, ok := lookupMessage(w, r)
	if !ok {
		return
	}
	var raw []byte
	err := db.WithReadOnlyTx(r.Context(), func(tx *sql.Tx) error {
		row := tx.QueryRowContext(r.Context(),
			`SELECT raw_message FROM email WHERE id = $1;`, message)
		return row.Scan(&raw)
	})
	if err != nil {
		serverError(w, r, "raw", err)
		return
	}
	_ = list
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(raw)
}

// A thread as an mbox spool: every message in it, depth first, each preceded
// by a From_ line. The Python app re-serialises the parsed message; that
// round-trips to the archived bytes, so those are used directly.
func handleThreadMbox(w http.ResponseWriter, r *http.Request) {
	list, message, ok := lookupMessage(w, r)
	if !ok {
		return
	}

	var (
		root     int
		threadID *int
		spool    []byte
	)
	err := db.WithReadOnlyTx(r.Context(), func(tx *sql.Tx) error {
		row := tx.QueryRowContext(r.Context(),
			`SELECT thread_id FROM email WHERE id = $1;`, message)
		if err := row.Scan(&threadID); err != nil {
			return err
		}
		if threadID != nil {
			return nil // only a thread root has an mbox
		}
		root = message

		// Children by parent, so the spool can be walked in reply order the
		// way format_mbox recurses.
		rows, err := tx.QueryContext(r.Context(), `
			SELECT id, parent_id, raw_message FROM email
			WHERE list_id = $1 AND (id = $2 OR thread_id = $2)
			ORDER BY created, id;
		`, list.ID, root)
		if err != nil {
			return err
		}
		defer rows.Close()

		raw := map[int][]byte{}
		children := map[int][]int{}
		for rows.Next() {
			var id int
			var parent *int
			var body []byte
			if err := rows.Scan(&id, &parent, &body); err != nil {
				return err
			}
			raw[id] = body
			if parent != nil {
				children[*parent] = append(children[*parent], id)
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}

		var walk func(id int)
		stamp := time.Now().Format("Mon Jan _2 15:04:05 2006")
		walk = func(id int) {
			spool = append(spool, []byte("From nobody "+stamp+"\r\n")...)
			spool = append(spool, raw[id]...)
			spool = append(spool, '\r', '\n')
			for _, child := range children[id] {
				walk(child)
			}
		}
		walk(root)
		return nil
	})
	if err != nil {
		serverError(w, r, "mbox", err)
		return
	}
	if threadID != nil {
		notFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/mbox")
	w.Write(spool)
}

// Shared preamble of the per-message routes: resolve the list, check browse
// access, and find the message. Writes the error response itself.
func lookupMessage(w http.ResponseWriter, r *http.Request) (*listView, int, bool) {
	owner, ok := strings.CutPrefix(r.PathValue("owner"), "~")
	if !ok {
		notFound(w, r)
		return nil, 0, false
	}
	list, err := getList(r, owner, r.PathValue("list"))
	if err != nil {
		serverError(w, r, "message", err)
		return nil, 0, false
	}
	if list == nil {
		notFound(w, r)
		return nil, 0, false
	}
	if !list.Access.Browse {
		forbidden(w, r)
		return nil, 0, false
	}

	var id int
	err = db.WithReadOnlyTx(r.Context(), func(tx *sql.Tx) error {
		row := tx.QueryRowContext(r.Context(), `
			SELECT id FROM email WHERE list_id = $1 AND message_id = $2;
		`, list.ID, r.PathValue("messageID"))
		return row.Scan(&id)
	})
	switch {
	case err == sql.ErrNoRows:
		notFound(w, r)
		return nil, 0, false
	case err != nil:
		serverError(w, r, "message", err)
		return nil, 0, false
	}
	return list, id, true
}
