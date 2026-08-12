package main

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/db"
)

// Subscribing is idempotent: submitting twice is not an error, it just does
// nothing the second time.
func handleSubscribe(w http.ResponseWriter, r *http.Request) {
	list, user, ok := listForWrite(w, r, needBrowse)
	if !ok {
		return
	}
	err := db.WithTx(r.Context(), nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `
			INSERT INTO subscription (created, updated, user_id, list_id)
			VALUES ($1, $1, $2, $3)
			ON CONFLICT (list_id, user_id) DO NOTHING;
		`, time.Now().UTC(), user.ID, list.ID)
		return err
	})
	if err != nil {
		serverError(w, r, "subscribe", err)
		return
	}
	http.Redirect(w, r, "/"+list.FullName(), http.StatusFound)
}

// Unsubscribing does not check browse access: someone whose access was
// revoked still needs to be able to get off the list.
func handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	list, user, ok := listForWrite(w, r, needNothing)
	if !ok {
		return
	}
	err := db.WithTx(r.Context(), nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `
			DELETE FROM subscription WHERE list_id = $1 AND user_id = $2;
		`, list.ID, user.ID)
		return err
	})
	if err != nil {
		serverError(w, r, "unsubscribe", err)
		return
	}
	http.Redirect(w, r, "/"+list.FullName(), http.StatusFound)
}

func handleSettingsInfo(w http.ResponseWriter, r *http.Request) {
	list, _, ok := listForWrite(w, r, needOwner)
	if !ok {
		return
	}
	visibility := r.PostFormValue("visibility")
	switch visibility {
	case "PUBLIC", "UNLISTED", "PRIVATE":
	default:
		// The Python app re-renders the form with an error; nothing sends an
		// invalid visibility but a hand-rolled request.
		renderSubview(w, r, "settings-info", "info", settingsData(list))
		return
	}
	description := r.PostFormValue("description")
	if len(description) >= 2048 {
		renderSubview(w, r, "settings-info", "info", settingsData(list))
		return
	}

	err := db.WithTx(r.Context(), nil, func(tx *sql.Tx) error {
		var desc *string
		if description != "" {
			desc = &description
		}
		_, err := tx.ExecContext(r.Context(), `
			UPDATE list SET visibility = $1, description = $2, updated = $3
			WHERE id = $4;
		`, visibility, desc, time.Now().UTC(), list.ID)
		return err
	})
	if err != nil {
		serverError(w, r, "settings info", err)
		return
	}
	http.Redirect(w, r, "/"+list.FullName()+"/settings/info", http.StatusFound)
}

func handleSettingsContent(w http.ResponseWriter, r *http.Request) {
	list, _, ok := listForWrite(w, r, needOwner)
	if !ok {
		return
	}
	permit := r.PostFormValue("permitMime")
	reject := r.PostFormValue("rejectMime")
	if permit == "" || reject == "" {
		renderSubview(w, r, "settings-content", "content", settingsData(list))
		return
	}
	err := db.WithTx(r.Context(), nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `
			UPDATE list SET permit_mimetypes = $1, reject_mimetypes = $2,
				updated = $3
			WHERE id = $4;
		`, permit, reject, time.Now().UTC(), list.ID)
		return err
	})
	if err != nil {
		serverError(w, r, "settings content", err)
		return
	}
	http.Redirect(w, r, "/"+list.FullName()+"/settings/content",
		http.StatusFound)
}

func handleSettingsAccess(w http.ResponseWriter, r *http.Request) {
	list, _, ok := listForWrite(w, r, needOwner)
	if !ok {
		return
	}
	err := db.WithTx(r.Context(), nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `
			UPDATE list SET default_access = $1, updated = $2 WHERE id = $3;
		`, submittedPerms(r, "default"), time.Now().UTC(), list.ID)
		return err
	})
	if err != nil {
		serverError(w, r, "settings access", err)
		return
	}
	http.Redirect(w, r, "/"+list.FullName()+"/settings/access",
		http.StatusFound)
}

// Grant, or regrant, one user or address access to a list.
func handleACLAdd(w http.ResponseWriter, r *http.Request) {
	list, _, ok := listForWrite(w, r, needOwner)
	if !ok {
		return
	}
	subject := strings.TrimPrefix(r.PostFormValue("user"), "~")
	if subject == "" {
		renderSubview(w, r, "settings-access", "access", settingsData(list))
		return
	}

	perms := submittedPerms(r, "acl")
	// Browsing is implied when the list grants it by default: an ACL entry
	// must not take away what everyone already has.
	if list.DefaultAccess&1 == 1 {
		perms |= 1
	}

	err := db.WithTx(r.Context(), nil, func(tx *sql.Tx) error {
		var userID *int
		var email *string
		if strings.Contains(subject, "@") {
			row := tx.QueryRowContext(r.Context(),
				`SELECT id FROM "user" WHERE email = $1;`, subject)
			var id int
			switch err := row.Scan(&id); err {
			case nil:
				userID = &id
			case sql.ErrNoRows:
				email = &subject
			default:
				return err
			}
		} else {
			row := tx.QueryRowContext(r.Context(),
				`SELECT id FROM "user" WHERE username = $1;`, subject)
			var id int
			if err := row.Scan(&id); err != nil {
				return err // includes "no such user", handled below
			}
			userID = &id
		}

		now := time.Now().UTC()
		if userID != nil {
			_, err := tx.ExecContext(r.Context(), `
				INSERT INTO access (created, updated, list_id, user_id, permissions)
				VALUES ($1, $1, $2, $3, $4)
				ON CONFLICT (list_id, user_id)
				DO UPDATE SET permissions = $4, updated = $1;
			`, now, list.ID, *userID, perms)
			return err
		}
		_, err := tx.ExecContext(r.Context(), `
			INSERT INTO access (created, updated, list_id, email, permissions)
			VALUES ($1, $1, $2, $3, $4);
		`, now, list.ID, *email, perms)
		return err
	})
	if err == sql.ErrNoRows {
		renderSubview(w, r, "settings-access", "access", settingsData(list))
		return
	}
	if err != nil {
		serverError(w, r, "acl", err)
		return
	}
	http.Redirect(w, r, "/"+list.FullName()+"/settings/access",
		http.StatusFound)
}

func handleACLDelete(w http.ResponseWriter, r *http.Request) {
	list, _, ok := listForWrite(w, r, needOwner)
	if !ok {
		return
	}
	aclID, err := strconv.Atoi(r.PathValue("acl"))
	if err != nil {
		notFound(w, r)
		return
	}

	var listID int
	err = db.WithTx(r.Context(), nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(r.Context(),
			`SELECT list_id FROM access WHERE id = $1;`, aclID)
		if err := row.Scan(&listID); err != nil {
			return err
		}
		if listID != list.ID {
			return nil // reported as a 403 below
		}
		_, err := tx.ExecContext(r.Context(),
			`DELETE FROM access WHERE id = $1;`, aclID)
		return err
	})
	switch {
	case err == sql.ErrNoRows:
		notFound(w, r)
		return
	case err != nil:
		serverError(w, r, "acl delete", err)
		return
	case listID != list.ID:
		forbidden(w, r)
		return
	}
	http.Redirect(w, r, "/"+list.FullName()+"/settings/access",
		http.StatusFound)
}

// The permission bits ticked in a perm_<field>_<name> checkbox group.
func submittedPerms(r *http.Request, field string) int {
	perms := 0
	for _, perm := range permissions {
		if r.PostFormValue("perm_"+field+"_"+perm.Name) != "" {
			perms |= perm.Value
		}
	}
	return perms
}

type accessRequirement int

const (
	needNothing accessRequirement = iota
	needBrowse
	needOwner
)

// The preamble every write handler shares: a logged-in user, a list, and the
// access the route requires. Writes the error response itself.
func listForWrite(w http.ResponseWriter, r *http.Request,
	required accessRequirement) (*listView, *User, bool) {

	if !requireLogin(w, r) {
		return nil, nil, false
	}
	if !csrfOK(r) {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return nil, nil, false
	}
	owner, ok := strings.CutPrefix(r.PathValue("owner"), "~")
	if !ok {
		notFound(w, r)
		return nil, nil, false
	}
	list, err := getList(r, owner, r.PathValue("list"))
	if err != nil {
		serverError(w, r, "list", err)
		return nil, nil, false
	}
	if list == nil {
		notFound(w, r)
		return nil, nil, false
	}
	switch required {
	case needBrowse:
		if !list.Access.Browse {
			forbidden(w, r)
			return nil, nil, false
		}
	case needOwner:
		if !list.IsOwner {
			forbidden(w, r)
			return nil, nil, false
		}
	}
	return list, userFor(r), true
}
