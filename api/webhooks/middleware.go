package webhooks

import (
	"context"
	"net/http"

	"git.sr.ht/~sircmpwn/core-go/webhooks"
)

func NewLegacyQueue() *webhooks.LegacyQueue {
	return webhooks.NewLegacyQueue()
}

var legacyWebhooksCtxKey = &contextKey{"legacyWebhooks"}

type contextKey struct {
	name string
}

func LegacyMiddleware(
	queue *webhooks.LegacyQueue,
) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), legacyWebhooksCtxKey, queue)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

func LegacyQueueForContext(ctx context.Context) *webhooks.LegacyQueue {
	q, ok := ctx.Value(legacyWebhooksCtxKey).(*webhooks.LegacyQueue)
	if !ok {
		panic("No legacy webhooks worker for this context")
	}
	return q
}
