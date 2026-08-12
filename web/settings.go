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
