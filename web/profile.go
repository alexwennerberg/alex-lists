package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/db"
	"github.com/yuin/goldmark"
)

const perPage = 15

type mailingList struct {
	Owner        string
	Name         string
	Description  *string
	Visibility   string
	LastActivity *time.Time
}

func (l mailingList) FullName() string { return "~" + l.Owner + "/" + l.Name }

type profile struct {
	User       string
	Lists      []mailingList
	Search     string
	Page       int
	TotalPages int
	IsSelf     bool
}

func handleProfile(w http.ResponseWriter, r *http.Request) {
	username, ok := strings.CutPrefix(r.PathValue("owner"), "~")
	if !ok {
		notFound(w, r)
		return
	}
	owner, err := lookupUser(r.Context(), username)
	if err != nil {
		serverError(w, r, "profile", err)
		return
	}
	if owner == nil {
		notFound(w, r)
		return
	}

	user := userFor(r)
	self := user != nil && user.ID == owner.ID
	data := profile{
		User:   owner.Username,
		Search: r.URL.Query().Get("search"),
		IsSelf: self,
		Page:   pageParam(r),
	}

	// Private lists are only visible to their owner. Search is a plain
	// substring over name and description; srht's key:value terms are not
	// supported here yet.
	where := `l.owner_id = $1`
	if !self {
		where += ` AND l.visibility = 'PUBLIC'`
	}
	if data.Search != "" {
		where += ` AND (l.name ILIKE $2 OR l.description ILIKE $2)`
	}
	args := []any{owner.ID}
	if data.Search != "" {
		args = append(args, "%"+data.Search+"%")
	}

	err = db.WithReadOnlyTx(r.Context(), func(tx *sql.Tx) error {
		row := tx.QueryRowContext(r.Context(),
			`SELECT count(*) FROM list l WHERE `+where, args...)
		var total int
		if err := row.Scan(&total); err != nil {
			return err
		}
		data.TotalPages = totalPages(total)

		rows, err := tx.QueryContext(r.Context(), `
			SELECT $`+strconv.Itoa(len(args)+1)+`::text, l.name, l.description,
				l.visibility, l.last_activity
			FROM list l WHERE `+where+`
			ORDER BY l.last_activity DESC NULLS LAST
			LIMIT `+strconv.Itoa(perPage)+`
			OFFSET `+strconv.Itoa((data.Page-1)*perPage),
			append(args, owner.Username)...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var ml mailingList
			if err := rows.Scan(&ml.Owner, &ml.Name, &ml.Description,
				&ml.Visibility, &ml.LastActivity); err != nil {
				return err
			}
			data.Lists = append(data.Lists, ml)
		}
		return rows.Err()
	})
	if err != nil {
		serverError(w, r, "profile", err)
		return
	}

	render(w, r, "profile-lists", &data)
}

// Deprecated route, kept because things link to it.
func handleListsForUser(w http.ResponseWriter, r *http.Request) {
	username, ok := strings.CutPrefix(r.PathValue("owner"), "~")
	if !ok {
		notFound(w, r)
		return
	}
	http.Redirect(w, r, "/~"+username, http.StatusFound)
}

func pageParam(r *http.Request) int {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		return 1
	}
	return page
}

func totalPages(total int) int {
	pages := total/perPage + 1
	if total%perPage == 0 && total != 0 {
		pages--
	}
	return pages
}

// srht renders descriptions with python-markdown behind a sanitizer, wrapped
// in this <style> and <div>. goldmark agrees with it on ordinary prose; the
// two will drift on anything exotic, which is the first place this rewrite
// stops being byte for byte.
func markdown(text string) template.HTML {
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(text), &buf); err != nil {
		return template.HTML("<p>" + html.EscapeString(text) + "</p>")
	}
	return template.HTML(
		`<style>.highlight { background: inherit; }</style>` +
			`<div class='markdown'>` + buf.String() + `</div>`)
}

// srht's date filter: an exact timestamp in the tooltip, humanised text in
// the page. contrib/golden scrubs both, so what has to match is the shape.
func dateFilter(t *time.Time) template.HTML {
	if t == nil {
		return ""
	}
	return template.HTML(fmt.Sprintf(`<span title="%s">%s</span>`,
		t.Format("2006-01-02 15:04:05 UTC"), humanize(*t)))
}

func humanize(t time.Time) string {
	seconds := int(time.Since(t).Seconds())
	if seconds < 0 {
		seconds = 0
	}
	for _, unit := range []struct {
		name string
		size int
	}{
		{"year", 365 * 24 * 60 * 60},
		{"month", 30 * 24 * 60 * 60},
		{"week", 7 * 24 * 60 * 60},
		{"day", 24 * 60 * 60},
		{"hour", 60 * 60},
		{"minute", 60},
	} {
		if n := seconds / unit.size; n > 0 {
			return plural(n, unit.name)
		}
	}
	return plural(seconds, "second")
}

func plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s ago", unit)
	}
	return fmt.Sprintf("%d %ss ago", n, unit)
}
