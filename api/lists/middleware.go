package lists

import (
	"context"
	"database/sql"
	"io"
	"log"
	"net/http"
	"time"

	"git.sr.ht/~sircmpwn/core-go/database"
	work "git.sr.ht/~sircmpwn/dowork"
)

type contextKey struct {
	name string
}

var ctxKey = &contextKey{"lists"}

func Middleware(queue *work.Queue) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKey, queue)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// Schedules a mailing list import.
func ImportMailingListSpool(ctx context.Context, listID int, spool io.Reader) {
	queue, ok := ctx.Value(ctxKey).(*work.Queue)
	if !ok {
		panic("No lists worker for this context")
	}
	task := work.NewTask(func(ctx context.Context) error {
		importCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()

		defer func() {
			if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `
				UPDATE list
				SET import_in_progress = false
				WHERE id = $1
			`, listID)
				return err
			}); err != nil {
				panic(err)
			}
		}()

		err := importMailingListSpool(importCtx, listID, spool)
		if err != nil {
			return err
		}
		return nil
	})
	queue.Enqueue(task)
	log.Printf("Enqueued mail spool import for mailing list %d", listID)
}

// Deletes a mailing list. Note that this function makes no attempt to verify
// the authorization of the caller to do so.
//
// It can take a while for the cascades to propagate out, so this belongs in a
// worker rather than blocking the request.
func DeleteMailingList(ctx context.Context, listID int) {
	queue, ok := ctx.Value(ctxKey).(*work.Queue)
	if !ok {
		panic("No lists worker for this context")
	}
	task := work.NewTask(func(ctx context.Context) error {
		if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(
				ctx, `DELETE FROM list WHERE id = $1;`, listID)
			return err
		}); err != nil {
			return err
		}
		log.Printf("Deleted mailing list %d", listID)
		return nil
	})
	queue.Enqueue(task)
	log.Printf("Enqueued mailing list deletion for list %d", listID)
}
