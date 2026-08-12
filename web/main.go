// The lists.sr.ht web app.
//
// A port of the Python one, route by route. While both exist they run side by
// side against the same database and the same session cookie, and
// contrib/golden diffs what they render.
package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"

	"git.sr.ht/~sircmpwn/core-go/config"
	"git.sr.ht/~sircmpwn/core-go/crypto"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/db"
	_ "github.com/lib/pq"
	"github.com/vaughan0/go-ini"
)

const service = "lists.sr.ht"

var srhtConfig ini.File

// Config values are read often enough (every page render) that a missing one
// should be loud and immediate rather than an empty string in the markup.
func conf(section, key string) string {
	value, ok := srhtConfig.Get(section, key)
	if !ok {
		log.Fatalf("missing [%s]%s in config", section, key)
	}
	return value
}

// For settings that are legitimately absent, like the origins of sibling
// sr.ht services this instance does not run.
func confOpt(section, key string) string {
	value, _ := srhtConfig.Get(section, key)
	return value
}

func main() {
	addr := flag.String("addr", ":5007",
		"address to listen on; the Python app has :5006 while both exist")
	flag.Parse()

	log.Default().SetOutput(os.Stdout)
	log.Default().SetPrefix("web: ")
	log.Default().SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	srhtConfig = config.LoadConfig()
	crypto.InitCrypto(srhtConfig)

	pg, err := sql.Open("postgres", conf(service, "connection-string"))
	if err != nil {
		log.Fatalf("database: %s", err)
	}
	defer pg.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", handleIndex)
	mux.HandleFunc("POST /{$}", handleIndexPOST)
	mux.HandleFunc("GET /login", handleLoginGET)
	mux.HandleFunc("POST /login", handleLoginPOST)
	mux.HandleFunc("GET /logout", handleLogout)
	// net/http patterns cannot express "/~{username}": a wildcard has to be a
	// whole segment. Match the segment and check the ~ in the handler.
	// Anything unmatched gets srht's 404 page rather than net/http's.
	mux.HandleFunc("GET /", handleNotFound)
	mux.HandleFunc("GET /{owner}", handleProfile)
	mux.HandleFunc("GET /lists/create", handleCreateList)
	mux.HandleFunc("GET /lists/create-mirror", handleCreateMirror)
	mux.HandleFunc("GET /lists/{owner}", handleListsForUser)
	mux.HandleFunc("GET /{owner}/{list}", handleArchive)
	// Message ids go in one path segment. The Python app routes them as
	// <path:message_id>, so an id containing a slash reaches it and not this.
	mux.HandleFunc("GET /{owner}/{list}/settings/info",
		settingsHandler("settings-info", "info"))
	mux.HandleFunc("POST /{owner}/{list}/subscribe", handleSubscribe)
	mux.HandleFunc("POST /{owner}/{list}/unsubscribe", handleUnsubscribe)
	mux.HandleFunc("POST /{owner}/{list}/settings/info", handleSettingsInfo)
	mux.HandleFunc("POST /{owner}/{list}/settings/content", handleSettingsContent)
	mux.HandleFunc("POST /{owner}/{list}/settings/access", handleSettingsAccess)
	mux.HandleFunc("POST /{owner}/{list}/settings/acl", handleACLAdd)
	mux.HandleFunc("POST /{owner}/{list}/settings/acl/{acl}/delete", handleACLDelete)
	mux.HandleFunc("GET /{owner}/{list}/settings/access",
		settingsHandler("settings-access", "access"))
	mux.HandleFunc("GET /{owner}/{list}/settings/content",
		settingsHandler("settings-content", "content"))
	mux.HandleFunc("GET /{owner}/{list}/settings/delete",
		settingsHandler("settings-delete", "delete"))
	mux.HandleFunc("GET /{owner}/{list}/settings/import-export",
		settingsHandler("settings-import-export", "export"))
	mux.HandleFunc("GET /{owner}/{list}/{messageID}", handleThread)
	mux.HandleFunc("GET /{owner}/{list}/{messageID}/raw", handleRaw)
	mux.HandleFunc("GET /{owner}/{list}/{messageID}/mbox", handleThreadMbox)
	// Registered by depth rather than as a "/static/" prefix: ServeMux will
	// not accept that alongside "/{owner}/{list}", since neither pattern is
	// more specific than the other. These two are.
	static := http.StripPrefix("/static/",
		http.FileServer(http.Dir(conf("sr.ht", "assets")+"/static")))
	mux.Handle("GET /static/{file}", static)
	mux.Handle("GET /static/{service}/{file}", static)

	handler := withDB(pg, withUser(mux))

	log.Printf("listening on %s", *addr)
	if err := http.ListenAndServe(*addr, handler); err != nil {
		log.Fatalf("listen: %s", err)
	}
}

func withDB(pg *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(db.Context(r.Context(), pg)))
	})
}
