package main

import (
	"database/sql"
	"net/http"
	"strings"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/db"
)

// Everything under /settings is owner-only: anonymous visitors are sent to
// the login page, and anyone else gets a 403.
type settingsPage struct {
	List        *listView
	Description string
	PermitMime  string
	RejectMime  string
}

func (s *settingsPage) PageNumber() int     { return 0 }
func (s *settingsPage) PageCount() int      { return 1 }
func (s *settingsPage) SearchTerms() string { return "" }

func settingsFor(w http.ResponseWriter, r *http.Request) (*settingsPage, bool) {
	if !requireLogin(w, r) {
		return nil, false
	}
	owner, ok := strings.CutPrefix(r.PathValue("owner"), "~")
	if !ok {
		notFound(w, r)
		return nil, false
	}
	list, err := getList(r, owner, r.PathValue("list"))
	if err != nil {
		serverError(w, r, "settings", err)
		return nil, false
	}
	if list == nil {
		notFound(w, r)
		return nil, false
	}
	if !list.IsOwner {
		forbidden(w, r)
		return nil, false
	}

	page := settingsPage{List: list}
	if list.Description != nil {
		page.Description = *list.Description
	}
	err = db.WithReadOnlyTx(r.Context(), func(tx *sql.Tx) error {
		row := tx.QueryRowContext(r.Context(), `
			SELECT permit_mimetypes, reject_mimetypes FROM list WHERE id = $1;
		`, list.ID)
		return row.Scan(&page.PermitMime, &page.RejectMime)
	})
	if err != nil {
		serverError(w, r, "settings", err)
		return nil, false
	}
	return &page, true
}

func settingsHandler(template, subview string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, ok := settingsFor(w, r)
		if !ok {
			return
		}
		renderSubview(w, r, template, subview, data)
	}
}

// The permission bits the access page draws a checkbox for. Iterating an
// IntFlag yields only the single-bit members, so none, normal and all never
// reach the template, and the macro's guard against them never fires.
type permission struct {
	Name  string
	Value int
	Help  string
	Skip  bool
}

var permissions = []permission{
	{Name: "browse", Value: 1,
		Help: "Permission to subscribe and browse the archives"},
	{Name: "reply", Value: 2,
		Help: "Permission to reply to threads submitted by an authorized user."},
	{Name: "post", Value: 4, Help: "Permission to submit new threads."},
	{Name: "moderate", Value: 8,
		Help: "Permission to moderate threads and patches."},
}

// A checkbox's state for one permission bit within a bitfield.
type permCheckbox struct {
	Perm    permission
	Field   string
	Checked bool
}

func (s *settingsPage) DefaultPerms() []permCheckbox {
	return permsFor(s.List.DefaultAccess, "default")
}

// The "access grants" form on a new ACL entry starts with everything ticked,
// as ListAccess.all does in the Python template.
func (s *settingsPage) ACLPerms() []permCheckbox {
	return permsFor(1|2|4|8, "acl")
}

func permsFor(bits int, field string) []permCheckbox {
	out := make([]permCheckbox, 0, len(permissions))
	for _, perm := range permissions {
		out = append(out, permCheckbox{
			Perm:    perm,
			Field:   field,
			Checked: perm.Value != 0 && bits&perm.Value == perm.Value,
		})
	}
	return out
}

// No ACL rows are rendered yet; the table needs the access table joined to
// users, which comes with the ACL forms.
func (s *settingsPage) ACLs() []struct{} { return nil }

// The page data a settings form needs when it has to be re-rendered after a
// failed submission.
func settingsData(list *listView) *settingsPage {
	page := settingsPage{List: list}
	if list.Description != nil {
		page.Description = *list.Description
	}
	return &page
}
