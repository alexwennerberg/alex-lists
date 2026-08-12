package main

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
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
	// Each page gets its own set: pages redefine "body"/"content", and the
	// last definition parsed wins, so they must not share one namespace.
	for name, extends := range map[string][]string{
		"index":         nil,
		"dashboard":     nil,
		"login":         nil,
		"not-found":     nil,
		"profile-lists": {"templates/profile.html"},
		"thread":        {"templates/list.html"},
		// list-full.html overrides the nav block that list.html inherits,
		// so it has to be parsed after it.
		"archive": {"templates/list.html", "templates/list-full.html"},
	} {
		files := []string{"templates/layout.html", "templates/nav.html",
			"templates/pagination.html", "templates/navlink.html"}
		files = append(files, extends...)
		page := template.New("layout.html").Funcs(funcs)
		_, err := page.ParseFS(assets, append(files,
			"templates/"+name+".html")...)
		if err != nil {
			log.Fatalf("parse %s: %s", name, err)
		}
		pages[name] = page
	}
}

type navlinkArgs struct {
	Path   string
	Title  string
	Active bool
}

var funcs = template.FuncMap{
	"icon":     icon,
	"cfg":      func(section, key string) string { return conf(section, key) },
	"markdown": markdown,
	"date":     dateFilter,
	"lower":    strings.ToLower,
	"add":      func(a, b int) int { return a + b },
	"sub":      func(a, b int) int { return a - b },
	// The archive tab is labelled "archive" on a thread page and "archives"
	// on the index; likewise patch/patches.
	"tabname": func(view, singular, plural string) string {
		if view == singular {
			return singular
		}
		return plural
	},
	// html/template escapes + to &#43; inside an href; a mailto: is a URL,
	// so hand it one.
	// html/template escapes + to &#43; in an attribute value, even a URL one,
	// so build the whole attribute rather than just its value.
	"mailto": func(address string) template.HTMLAttr {
		return template.HTMLAttr(`href="mailto:` + address + `"`)
	},
	"pathescape": url.PathEscape,
	// srht widens the archive and patches pages but not a single thread.
	"fluid": isFluid,
	"cls": func(view string) string {
		if isFluid(view) {
			return "container-fluid"
		}
		return "container"
	},
	"message": func(p *page, msg *threadMessage, isThread bool) messageContext {
		return messageContext{p, msg, isThread}
	},
	// Pagination state lives on the page content, which differs per page;
	// these report zero for pages that have none, matching an undefined
	// variable in Jinja.
	"pageNum":   pageNumber,
	"pageCount": pageCount,
	"navlink": func(path, title string, active bool) navlinkArgs {
		return navlinkArgs{path, title, active}
	},
	// A whole pagination href, typed as a URL so that html/template leaves
	// its & and = alone and only HTML-escapes them, as Jinja does.
	"pageLink": func(c any, delta int) template.URL {
		link := "?page=" + strconv.Itoa(pageNumber(c)+delta)
		if search := searchOf(c); search != "" {
			link += "&search=" + url.QueryEscape(search)
		}
		return template.URL(link)
	},
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

	// Sibling services, for the profile tabs. Empty when not configured,
	// which in this fork is all of them but lists.sr.ht itself.
	HubOrigin   string
	GitOrigin   string
	HgOrigin    string
	ListsOrigin string
	TodoOrigin  string
	User        *User
	LoginURL    string
	LogoutURL   string
	CSRF        template.HTML
	View        string
	Content     any
}

// What srht passes as view=, which drives the active tab and a few layout
// choices shared with the patches pages.
var views = map[string]string{
	"archive":       "archives",
	"thread":        "archive",
	"profile-lists": "lists",
}

func newPage(w http.ResponseWriter, r *http.Request, name string,
	content any) *page {
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
		HubOrigin:     confOpt("hub.sr.ht", "origin"),
		GitOrigin:     confOpt("git.sr.ht", "origin"),
		HgOrigin:      confOpt("hg.sr.ht", "origin"),
		ListsOrigin:   confOpt("lists.sr.ht", "origin"),
		TodoOrigin:    confOpt("todo.sr.ht", "origin"),
		User:          userFor(r),
		LoginURL:      "/login?return_to=" + returnToParam(returnTo),
		LogoutURL:     "/logout",
		CSRF:          csrfField(csrfToken(w, r)),
		View:          views[name],
		Content:       content,
	}
}

// Werkzeug's url_for escapes the return_to value but leaves the path and
// query separators legible, so "/l?a=b" becomes "/l?a%3Db".
func returnToParam(target string) string {
	escaped := url.QueryEscape(target)
	// Werkzeug's safe set is wider than this; these are the characters the
	// fixtures actually exercise.
	for _, pair := range [][2]string{{"%2F", "/"}, {"%3F", "?"}, {"%40", "@"}} {
		escaped = strings.ReplaceAll(escaped, pair[0], pair[1])
	}
	return escaped
}

func render(w http.ResponseWriter, r *http.Request, name string, content any) {
	renderStatus(w, r, name, content, http.StatusOK)
}

func renderStatus(w http.ResponseWriter, r *http.Request, name string,
	content any, status int) {

	// newPage issues the CSRF cookie, so it has to run before anything is
	// written; and buffering means a template that fails halfway does not
	// leave half a page on the wire behind a 200.
	data := newPage(w, r, name, content)
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

// Implemented by page content that paginates.
type paginated interface {
	PageNumber() int
	PageCount() int
	SearchTerms() string
}

func pageNumber(c any) int {
	if p, ok := c.(paginated); ok {
		return p.PageNumber()
	}
	return 0
}

func pageCount(c any) int {
	if p, ok := c.(paginated); ok {
		return p.PageCount()
	}
	return 1
}

func searchOf(c any) string {
	if p, ok := c.(paginated); ok {
		return p.SearchTerms()
	}
	return ""
}

func isFluid(view string) bool {
	return view == "archives" || view == "patches"
}
