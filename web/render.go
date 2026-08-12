package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strings"
)

//go:embed templates/*.html icons/*.svg
var assets embed.FS

// The templates are ports of the Jinja ones, whitespace included, so that
// contrib/golden compares the two implementations byte for byte. Where a
// blank line looks pointless it is almost certainly where a {% if %} used to
// be; leave it.
var pages = map[string]*template.Template{}

func init() {
	for _, name := range []string{"index", "dashboard", "login"} {
		page := template.New("layout.html").Funcs(funcs)
		_, err := page.ParseFS(assets,
			"templates/layout.html",
			"templates/nav.html",
			"templates/"+name+".html")
		if err != nil {
			log.Fatalf("parse %s: %s", name, err)
		}
		pages[name] = page
	}
}

var funcs = template.FuncMap{
	"icon": icon,
	"cfg":  func(section, key string) string { return conf(section, key) },
}

// Inline SVG, matching srht.app.icons.icon. The Font Awesome license comment
// upstream appends on first use inside a request never actually fires there
// (flask.g is falsy at that point), so it is not reproduced.
func icon(name string) template.HTML {
	svg, err := assets.ReadFile("icons/" + name + ".svg")
	if err != nil {
		log.Printf("no such icon: %s", name)
		return ""
	}
	return template.HTML(fmt.Sprintf(
		`<span class="icon icon-%s " aria-hidden="true">%s</span>`,
		name, svg))
}

// The values every page needs: what srht injects as template globals.
type page struct {
	Domain        string
	SiteName      string
	Site          string
	SiteShort     string
	Environment   string
	StaticURL     string
	AllowNewLists bool
	User          *User
	LoginURL      string
	LogoutURL     string
	CSRF          template.HTML
	Content       any
}

func newPage(w http.ResponseWriter, r *http.Request, content any) *page {
	returnTo := r.URL.Path
	if r.URL.RawQuery != "" {
		returnTo += "?" + r.URL.RawQuery
	}
	return &page{
		Domain:        strings.TrimPrefix(conf("lists.sr.ht", "origin"), "http://"),
		SiteName:      conf("sr.ht", "site-name"),
		Site:          service,
		SiteShort:     strings.Split(service, ".")[0],
		Environment:   strings.ToUpper(conf("sr.ht", "environment")),
		StaticURL:     "/static/" + service + "/main.css",
		AllowNewLists: conf("lists.sr.ht", "allow-new-lists") == "yes",
		User:          userFor(r),
		LoginURL:      "/login?return_to=" + returnTo,
		LogoutURL:     "/logout",
		CSRF:          csrfField(csrfToken(w, r)),
		Content:       content,
	}
}

func render(w http.ResponseWriter, r *http.Request, name string, content any) {
	renderStatus(w, r, name, content, http.StatusOK)
}

func renderStatus(w http.ResponseWriter, r *http.Request, name string,
	content any, status int) {

	// newPage issues the CSRF cookie, so it has to run before anything is
	// written; and buffering means a template that fails halfway does not
	// leave half a page on the wire behind a 200.
	data := newPage(w, r, content)
	var buf strings.Builder
	if err := pages[name].Execute(&buf, data); err != nil {
		log.Printf("render %s: %s", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	io.WriteString(w, buf.String())
}
