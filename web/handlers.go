package main

import (
	"context"
	"database/sql"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/db"
)

type listSummary struct {
	Owner string
	Name  string
}

func (l listSummary) FullName() string { return "~" + l.Owner + "/" + l.Name }

type dashboard struct {
	Subs   []listSummary
	Recent []recentEmail
}

type recentEmail struct {
	Subject   string
	MessageID string
	List      listSummary
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	user := userFor(r)
	if user == nil {
		render(w, r, "index", nil)
		return
	}

	var data dashboard
	err := db.WithReadOnlyTx(r.Context(), func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(r.Context(), `
			SELECT u.username, l.name
			FROM subscription s
			JOIN list l ON l.id = s.list_id
			JOIN "user" u ON u.id = l.owner_id
			WHERE s.user_id = $1
			ORDER BY l.last_activity DESC NULLS LAST
			LIMIT 10;
		`, user.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var sub listSummary
			if err := rows.Scan(&sub.Owner, &sub.Name); err != nil {
				return err
			}
			data.Subs = append(data.Subs, sub)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		rows, err = tx.QueryContext(r.Context(), `
			SELECT e.subject, e.message_id, u.username, l.name
			FROM email e
			JOIN list l ON l.id = e.list_id
			JOIN subscription s ON s.list_id = l.id
			JOIN "user" u ON u.id = l.owner_id
			WHERE s.user_id = $1
			ORDER BY e.created DESC
			LIMIT 10;
		`, user.ID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var email recentEmail
			if err := rows.Scan(&email.Subject, &email.MessageID,
				&email.List.Owner, &email.List.Name); err != nil {
				return err
			}
			data.Recent = append(data.Recent, email)
		}
		return rows.Err()
	})
	if err != nil {
		serverError(w, r, "dashboard", err)
		return
	}

	render(w, r, "dashboard", &data)
}

// The only field on the dashboard: send me a copy of my own emails.
func handleIndexPOST(w http.ResponseWriter, r *http.Request) {
	user := userFor(r)
	if user == nil {
		http.Redirect(w, r, "/login?return_to=/", http.StatusFound)
		return
	}
	if !csrfOK(r) {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}
	copySelf := r.PostFormValue("copy-self") != ""
	err := db.WithTx(r.Context(), nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(r.Context(), `
			UPDATE "user" SET copy_self = $1, updated = $2 WHERE id = $3;
		`, copySelf, time.Now().UTC(), user.ID)
		return err
	})
	if err != nil {
		serverError(w, r, "preferences", err)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

type loginPage struct {
	Users    []string
	ReturnTo string
	Error    string
}

func handleLoginGET(w http.ResponseWriter, r *http.Request) {
	returnTo := r.URL.Query().Get("return_to")
	if userFor(r) != nil {
		http.Redirect(w, r, safeReturnTo(returnTo), http.StatusFound)
		return
	}
	renderLogin(w, r, returnTo, "", http.StatusOK)
}

func renderLogin(w http.ResponseWriter, r *http.Request,
	returnTo, message string, status int) {

	// Offered as one-click buttons so you don't have to remember who exists.
	var users []string
	err := db.WithReadOnlyTx(r.Context(), func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(r.Context(), `
			SELECT username FROM "user" ORDER BY username LIMIT 50;
		`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var username string
			if err := rows.Scan(&username); err != nil {
				return err
			}
			users = append(users, username)
		}
		return rows.Err()
	})
	if err != nil {
		serverError(w, r, "login", err)
		return
	}
	renderStatus(w, r, "login", &loginPage{
		Users:    users,
		ReturnTo: returnTo,
		Error:    message,
	}, status)
}

func handleLoginPOST(w http.ResponseWriter, r *http.Request) {
	if !csrfOK(r) {
		http.Error(w, "invalid CSRF token", http.StatusBadRequest)
		return
	}
	returnTo := r.PostFormValue("return_to")
	username := strings.TrimPrefix(strings.TrimSpace(
		r.PostFormValue("username")), "~")
	if !validUsername(username) {
		renderLogin(w, r, returnTo,
			"Usernames may contain letters, digits, '.', '_' and '-', must "+
				"start with a letter or digit, and must be at most 128 "+
				"characters.", http.StatusBadRequest)
		return
	}

	user, err := getOrCreateUser(r.Context(), username)
	if err != nil {
		serverError(w, r, "login", err)
		return
	}
	setSession(w, user)
	http.Redirect(w, r, safeReturnTo(returnTo), http.StatusFound)
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSession(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// Unknown usernames are created on the spot: this instance has no meta.sr.ht
// to delegate identity to. See listssrht/auth.py, which does the same thing.
func getOrCreateUser(ctx context.Context, username string) (*User, error) {
	user, err := lookupUser(ctx, username)
	if err != nil || user != nil {
		return user, err
	}
	user = &User{Username: username, Email: username + "@localhost"}
	err = db.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO "user" (
				username, created, updated, email, user_type
			) VALUES ($1, $2, $2, $3, 'user')
			RETURNING id, copy_self;
		`, username, time.Now().UTC(), user.Email)
		return row.Scan(&user.ID, &user.CopySelf)
	})
	if err != nil {
		return nil, err
	}
	return user, nil
}

// Even a toy login shouldn't be a usable open redirect.
func safeReturnTo(target string) string {
	if target == "" {
		return "/"
	}
	parsed, err := url.Parse(target)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" {
		return "/"
	}
	if !strings.HasPrefix(parsed.Path, "/") {
		return "/"
	}
	if parsed.RawQuery != "" {
		return parsed.Path + "?" + parsed.RawQuery
	}
	return parsed.Path
}

func notFound(w http.ResponseWriter, r *http.Request) {
	renderStatus(w, r, "not-found", nil, http.StatusNotFound)
}

// Werkzeug's own 403 page, which is what abort(403) produces in the Python
// app; srht does not override it.
func forbidden(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	io.WriteString(w, `<!doctype html>
<html lang=en>
<title>403 Forbidden</title>
<h1>Forbidden</h1>
<p>You don&#39;t have the permission to access the requested resource. It is either read-protected or not readable by the server.</p>
`)
}

func serverError(w http.ResponseWriter, r *http.Request, what string, err error) {
	log.Printf("%s %s: %s", r.Method, r.URL.Path, err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
