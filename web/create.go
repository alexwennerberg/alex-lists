package main

import (
	"database/sql"
	"net/http"
	"regexp"
	"time"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/db"
)

// Both create forms are login-gated and, on a fresh GET, static: the fields
// are empty and no validation has run. The POST handlers are not ported yet.
func handleCreateList(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	render(w, r, "create", nil)
}

func handleCreateMirror(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	render(w, r, "create-mirror", nil)
}

// srht's loginrequired sends anonymous visitors to app.login_url, which this
// fork overrides to carry a return_to.
func requireLogin(w http.ResponseWriter, r *http.Request) bool {
	if userFor(r) == nil {
		target := r.URL.Path
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, "/login?return_to="+returnToParam(target),
			http.StatusFound)
		return false
	}
	return true
}

// Creating a list, with the same validation listssrht.types.List does in its
// constructor. The owner is subscribed to their own list, as the API's
// createMailingList mutation used to do.
func handleCreateListPOST(w http.ResponseWriter, r *http.Request) {
	if !requireLogin(w, r) {
		return
	}
	if !csrfOK(r) {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}
	user := userFor(r)

	name := r.PostFormValue("name")
	description := r.PostFormValue("description")
	visibility := r.PostFormValue("visibility")
	invalid := func() {
		render(w, r, "create", nil)
	}
	switch {
	case name == "", !listName.MatchString(name),
		name == ".", name == "..", name == ".git", name == ".hg",
		len(description) >= 2048:
		invalid()
		return
	}
	switch visibility {
	case "PUBLIC", "UNLISTED", "PRIVATE":
	default:
		invalid()
		return
	}

	var taken bool
	err := db.WithTx(r.Context(), nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(r.Context(), `
			SELECT count(*) FROM list
			WHERE owner_id = $1 AND lower(name) = lower($2);
		`, user.ID, name)
		var count int
		if err := row.Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			taken = true
			return nil
		}

		var desc *string
		if description != "" {
			desc = &description
		}
		now := time.Now().UTC()
		row = tx.QueryRowContext(r.Context(), `
			INSERT INTO list (created, updated, name, description, owner_id,
				visibility)
			VALUES ($1, $1, $2, $3, $4, $5)
			RETURNING id;
		`, now, name, desc, user.ID, visibility)
		var listID int
		if err := row.Scan(&listID); err != nil {
			return err
		}
		_, err := tx.ExecContext(r.Context(), `
			INSERT INTO subscription (created, updated, user_id, list_id)
			VALUES ($1, $1, $2, $3);
		`, now, user.ID, listID)
		return err
	})
	if err != nil {
		serverError(w, r, "create list", err)
		return
	}
	if taken {
		invalid()
		return
	}
	http.Redirect(w, r, "/~"+user.Username+"/"+name, http.StatusFound)
}

var listName = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
