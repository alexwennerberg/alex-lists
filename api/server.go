package main

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"git.sr.ht/~sircmpwn/core-go/auth"
	"git.sr.ht/~sircmpwn/core-go/config"
	"git.sr.ht/~sircmpwn/core-go/database"
	"git.sr.ht/~sircmpwn/core-go/server"
	"git.sr.ht/~sircmpwn/core-go/webhooks"
	"github.com/99designs/gqlgen/graphql"
	"github.com/emersion/go-mbox"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"github.com/go-chi/chi"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/loaders"
)

func main() {
	appConfig := config.LoadConfig(":5106")

	gqlConfig := api.Config{Resolvers: &graph.Resolver{}}
	gqlConfig.Directives.Private = server.Private
	gqlConfig.Directives.Internal = server.Internal
	gqlConfig.Directives.Access = func(ctx context.Context, obj interface{},
		next graphql.Resolver, scope model.AccessScope,
		kind model.AccessKind) (interface{}, error) {
		return server.Access(ctx, obj, next, scope.String(), kind.String())
	}
	schema := api.NewExecutableSchema(gqlConfig)

	scopes := make([]string, len(model.AllAccessScope))
	for i, s := range model.AllAccessScope {
		scopes[i] = s.String()
	}

	webhookQueue := webhooks.NewQueue(schema)
	legacyWebhooks := webhooks.NewLegacyQueue()

	gsrv := server.NewServer("lists.sr.ht", appConfig).
		WithDefaultMiddleware().
		WithMiddleware(
			loaders.Middleware,
			webhooks.Middleware(webhookQueue),
			webhooks.LegacyMiddleware(legacyWebhooks),
		).
		WithQueues(webhookQueue.Queue, legacyWebhooks.Queue).
		WithSchema(schema, scopes)

	// Bulk transfer endpoints
	gsrv.Router().Get("/query/email/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid mail ID\r\n"))
			return
		}
		mail, err := loaders.ForContext(r.Context()).EmailsByID.Load(id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Unknown email\r\n"))
			return
		}
		w.Header().Add("Content-Type", "message/rfc822")
		w.Write([]byte(mail.RawEnvelope))
	})

	gsrv.Router().Get("/query/thread/{id}.mbox", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid thread ID\r\n"))
			return
		}

		if err := database.WithTx(r.Context(), &sql.TxOptions{
			Isolation: 0,
			ReadOnly:  true,
		}, func(tx *sql.Tx) error {
			rows, err := tx.QueryContext(r.Context(), `
				SELECT email.envelope, email.created
				FROM email
				JOIN list ON list.id = email.list_id
				LEFT JOIN access ON access.list_id = list.id
				LEFT JOIN subscription sub ON sub.list_id = list.id
				WHERE email.id = $1 OR email.thread_id = $1 AND (
					list.owner_id = $2 OR
					access.permissions & $3 > 0 OR
					list.default_access & $3 > 0)
				ORDER BY email.id
			`, id, auth.ForContext(r.Context()).UserID, model.ACCESS_BROWSE)
			if err != nil {
				return err
			}
			return prepMbox(rows, w)
		}); err != nil {
			panic(err)
		}
	})

	gsrv.Router().Get("/query/patchset/{id}.mbox", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid patchset ID\r\n"))
			return
		}

		if err := database.WithTx(r.Context(), &sql.TxOptions{
			Isolation: 0,
			ReadOnly:  true,
		}, func(tx *sql.Tx) error {
			rows, err := tx.QueryContext(r.Context(), `
				SELECT email.envelope, email.created
				FROM email
				JOIN list ON list.id = email.list_id
				LEFT JOIN access ON access.list_id = list.id
				LEFT JOIN subscription sub ON sub.list_id = list.id
				WHERE email.patchset_id = $1 AND email.is_patch AND (
					list.owner_id = $2 OR
					access.permissions & $3 > 0 OR
					list.default_access & $3 > 0)
				ORDER BY email.patch_index, email.id
			`, id, auth.ForContext(r.Context()).UserID, model.ACCESS_BROWSE)
			if err != nil {
				return err
			}
			return prepMbox(rows, w)
		}); err != nil {
			panic(err)
		}
	})

	gsrv.Router().Get("/query/list/{id}.mbox", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid mailing list ID\r\n"))
			return
		}

		var since time.Time
		if val, ok := r.URL.Query()["since"]; ok {
			days, err := strconv.Atoi(val[0])
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte("Invalid since days\r\n"))
				return
			}
			since = time.Now().UTC().Add(-(time.Hour * 24 * time.Duration(days)))
		}

		if err := database.WithTx(r.Context(), &sql.TxOptions{
			Isolation: 0,
			ReadOnly:  true,
		}, func(tx *sql.Tx) error {
			rows, err := tx.QueryContext(r.Context(), `
				SELECT email.envelope, email.created
				FROM email
				JOIN list ON list.id = email.list_id
				LEFT JOIN access ON access.list_id = list.id
				LEFT JOIN subscription sub ON sub.list_id = list.id
				WHERE email.list_id = $1 AND email.created >= $2 AND (
					list.owner_id = $3 OR
					access.permissions & $4 > 0 OR
					list.default_access & $4 > 0)
				ORDER BY email.created
			`, id, since, auth.ForContext(r.Context()).UserID, model.ACCESS_BROWSE)
			if err != nil {
				return err
			}
			return prepMbox(rows, w)
		}); err != nil {
			panic(err)
		}
	})

	gsrv.Run()
}

func prepMbox(rows *sql.Rows, w http.ResponseWriter) error {
	mbw := mbox.NewWriter(w)
	defer mbw.Close()

	var results bool
	for rows.Next() {
		results = true
		w.Header().Add("Content-Type", "application/mbox")

		var (
			envelope string
			created  time.Time
		)
		if err := rows.Scan(&envelope, &created); err != nil {
			return err
		}

		reader, err := mail.CreateReader(bytes.NewBufferString(envelope))
		if err != nil {
			return err
		}
		from, err := reader.Header.AddressList("From")
		reader.Close()
		if err != nil {
			from = []*mail.Address{&mail.Address{"unknown", "unknown@example.org"}}
		}
		sink, err := mbw.CreateMessage(from[0].Address, created)
		if err != nil {
			return err
		}
		if _, err = sink.Write([]byte(envelope)); err != nil {
			return err
		}
	}

	if !results {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not found\r\n"))
	}

	return nil
}
