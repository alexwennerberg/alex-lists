package main

import "net/http"

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
