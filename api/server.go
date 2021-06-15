package main

import (
	"context"

	"git.sr.ht/~sircmpwn/core-go/config"
	"git.sr.ht/~sircmpwn/core-go/email"
	"git.sr.ht/~sircmpwn/core-go/server"
	"github.com/99designs/gqlgen/graphql"

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
	server.NewServer("lists.sr.ht", appConfig).
		WithDefaultMiddleware().
		WithMiddleware(
			loaders.Middleware,
			email.Middleware(mail),
		).
		WithSchema(schema, scopes).
		WithQueues(mail).
		Run()
}
