package main

import (
	"context"
	"net/http"
	"strconv"

	"git.sr.ht/~sircmpwn/core-go/config"
	"git.sr.ht/~sircmpwn/core-go/email"
	"git.sr.ht/~sircmpwn/core-go/server"
	"github.com/99designs/gqlgen/graphql"
	"github.com/go-chi/chi"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/loaders"
)

func main() {
	appConfig := config.LoadConfig(":5106")

	gqlConfig := api.Config{Resolvers: &graph.Resolver{}}
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

	mail := email.NewQueue()
	gsrv := server.NewServer("lists.sr.ht", appConfig).
		WithDefaultMiddleware().
		WithMiddleware(
			loaders.Middleware,
			email.Middleware(mail),
		).
		WithSchema(schema, scopes).
		WithQueues(mail)

	// Bulk transfer endpoints
	gsrv.Router().Get("/query/email/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(chi.URLParam(r, "id"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Invalid mail ID"))
			return
		}
		mail, err := loaders.ForContext(r.Context()).EmailsByID.Load(id)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("Unknown email"))
			return
		}
		w.Header().Add("Content-Type", "message/rfc822")
		w.Write([]byte(mail.RawEnvelope))
	})

	gsrv.Router().Get("/query/thread/{id}.mbox", func(w http.ResponseWriter, r *http.Request) {
		// TODO
		w.WriteHeader(200)
		w.Write([]byte("200 OK"))
	})

	gsrv.Router().Get("/query/patchset/{id}.mbox", func(w http.ResponseWriter, r *http.Request) {
		// TODO
		w.WriteHeader(200)
		w.Write([]byte("200 OK"))
	})

	gsrv.Router().Get("/query/list/{id}.mbox", func(w http.ResponseWriter, r *http.Request) {
		// TODO
		w.WriteHeader(200)
		w.Write([]byte("200 OK"))
	})

	gsrv.Run()
}
