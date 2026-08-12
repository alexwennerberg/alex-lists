package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"git.sr.ht/~sircmpwn/core-go/crypto"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/db"
)

// The cookie srht writes and reads: fernet(JSON) under [sr.ht]network-key,
// looked up by the "name" field. Reproduced exactly, so a session survives
// moving between this and the Python app while both are running.
const sessionCookie = "sr.ht.unified-login.v1"

const csrfCookie = "lists.sr.ht.csrf"

type User struct {
	ID       int
	Username string
	Email    string
	CopySelf bool
}

type ctxKey struct{ name string }

var userKey = &ctxKey{"user"}

func userFor(r *http.Request) *User {
	user, _ := r.Context().Value(userKey).(*User)
	return user
}

// Resolve the session cookie once per request rather than in every handler.
func withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := userFromCookie(r)
		if user != nil {
			r = r.WithContext(context.WithValue(r.Context(), userKey, user))
		}
		next.ServeHTTP(w, r)
	})
}

func userFromCookie(r *http.Request) *User {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || cookie.Value == "" {
		return nil
	}
	payload := crypto.DecryptWithoutExpiration([]byte(cookie.Value))
	if payload == nil {
		return nil // tampered with, or a stale key
	}
	var info struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(payload, &info); err != nil {
		return nil
	}
	user, err := lookupUser(r.Context(), info.Name)
	if err != nil {
		log.Printf("session lookup %q: %s", info.Name, err)
		return nil
	}
	return user
}

func lookupUser(ctx context.Context, username string) (*User, error) {
	var user User
	err := db.WithReadOnlyTx(ctx, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT id, username, email, copy_self
			FROM "user" WHERE username = $1;
		`, username)
		return row.Scan(&user.ID, &user.Username, &user.Email, &user.CopySelf)
	})
	switch {
	case err == sql.ErrNoRows:
		return nil, nil
	case err != nil:
		return nil, err
	}
	return &user, nil
}

func setSession(w http.ResponseWriter, user *User) {
	payload, err := json.Marshal(map[string]any{
		"name":           user.Username,
		"canonical_name": "~" + user.Username,
		"email":          user.Email,
	})
	if err != nil {
		panic(err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    string(crypto.Encrypt(payload)),
		Path:     "/",
		Domain:   conf("sr.ht", "global-domain"),
		HttpOnly: true,
		MaxAge:   60 * 60 * 24 * 365,
	})
}

func clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		Domain:   conf("sr.ht", "global-domain"),
		HttpOnly: true,
		MaxAge:   0,
	})
}

// --- CSRF -------------------------------------------------------------------

// Token in a cookie, same token in a hidden field, compared on POST. srht
// keeps it in the Flask session instead, but the markup has to match its
// csrf_token() byte for byte or every form in the fixtures differs.
func csrfToken(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(csrfCookie); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	buf := make([]byte, 64)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	token := hex.EncodeToString(buf)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
	})
	return token
}

func csrfField(token string) template.HTML {
	return template.HTML(fmt.Sprintf(`<input
        type='hidden'
        name='_csrf_token'
        value='%s' />`, token))
}

func csrfOK(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookie)
	if err != nil || cookie.Value == "" {
		return false
	}
	sent := r.PostFormValue("_csrf_token")
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(sent)) == 1
}

func validUsername(username string) bool {
	username = strings.TrimPrefix(username, "~")
	if username == "" || len(username) > 128 {
		return false
	}
	for i, c := range username {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case i > 0 && (c == '.' || c == '_' || c == '-'):
		default:
			return false
		}
	}
	return true
}
