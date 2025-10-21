package lists

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"text/template"
	"time"

	"git.sr.ht/~sircmpwn/core-go/auth"
	"git.sr.ht/~sircmpwn/core-go/client"
	"git.sr.ht/~sircmpwn/core-go/config"
	"git.sr.ht/~sircmpwn/core-go/database"
	work "git.sr.ht/~sircmpwn/dowork"
	"github.com/emersion/go-message/mail"
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

func sendEmail(ctx context.Context, address, message string) error {
	return client.Do(ctx, "", "meta.sr.ht", client.GraphQLQuery{
		Query: `
		mutation sendEmail($address: String!, $message: String!) {
			sendEmail(address: $address, message: $message)
		}`,
		Variables: map[string]interface{}{
			"address": address,
			"message": message,
		},
	}, struct{}{})
}

type importReport struct {
	userID   int
	username string
	email    string
	listID   int
	listName string
	result   ImportResult
	err      error
}

func sendImportReport(ctx context.Context, report importReport) {
	conf := config.ForContext(ctx)
	siteName, ok := conf.Get("sr.ht", "site-name")
	if !ok {
		panic(fmt.Errorf("Expected [sr.ht]site-name in config"))
	}
	ownerName, ok := conf.Get("sr.ht", "owner-name")
	if !ok {
		panic(fmt.Errorf("Expected [sr.ht]owner-name in config"))
	}

	address := mail.Address{
		Name:    report.username,
		Address: report.email,
	}
	type TemplateContext struct {
		OwnerName string
		SiteName  string
		Username  string
		ListName  string
		Result    ImportResult
		Fatal     error
	}
	tctx := TemplateContext{
		OwnerName: ownerName,
		SiteName:  siteName,
		Username:  report.username,
		ListName:  report.listName,
		Result:    report.result,
		Fatal:     report.err,
	}

	tmpl := template.Must(template.New("import-failed").Parse(`Subject: Mailing list import report

Hello ~{{.Username}}!

{{ if .Fatal -}}
The import for your mailing list "{{.ListName}}" failed:

{{.Fatal}}

The transaction was rolled back and no messages were imported.
{{- else -}}
The import for your mailing list "{{.ListName}}" failed for some messages.

- {{.Result.Imported}} messages were imported
- {{.Result.Duplicate}} messages were dropped as duplicates
- {{ len .Result.Dropped }} messages were dropped due to errors

{{ range $i, $err := .Result.Dropped -}}
{{ if eq $i 50 -}}
...
{{- break -}}
{{- end -}}
{{ . }}
{{ end -}}
{{- end }}
--
{{.OwnerName}}
{{.SiteName}}`))

	var message strings.Builder
	err := tmpl.Execute(&message, tctx)
	if err != nil {
		panic(err)
	}

	sendEmail(ctx, address.String(), message.String())
}

// Schedules a mailing list import.
func ImportMailingListSpool(ctx context.Context, listID int, listName string, spool io.Reader) {
	queue, ok := ctx.Value(ctxKey).(*work.Queue)
	if !ok {
		panic(fmt.Errorf("No lists worker for this context"))
	}

	// Capture this here, the task has a context without authentication
	user := auth.ForContext(ctx)

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

		return database.WithTx(ctx, nil, func(tx *sql.Tx) error {
			result, err := NewArchiver(importCtx, tx, listID).
				ImportSpool(spool)
			if len(result.Dropped) > 0 || err != nil {
				report := importReport{
					userID:   user.UserID,
					username: user.Username,
					email:    user.Email,
					listID:   listID,
					listName: listName,
					result:   result,
					err:      err,
				}
				sendImportReport(ctx, report)
			}
			return err
		})
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
		panic(fmt.Errorf("No lists worker for this context"))
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
