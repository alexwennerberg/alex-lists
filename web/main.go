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
	mux.Handle("GET /static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir(conf("sr.ht", "assets")+"/static"))))

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
