package model

import (
	"context"
	"database/sql"
	"time"
	"strconv"

	sq "github.com/Masterminds/squirrel"

	"git.sr.ht/~sircmpwn/core-go/database"
	"git.sr.ht/~sircmpwn/core-go/model"
)

type Email struct {
	ID        int       `json:"id"`
	Sender    Entity    `json:"sender"`
	Received  time.Time `json:"received"`
	Date      time.Time `json:"date"`
	Envelope  Mail      `json:"envelope"`
	Body      string    `json:"body"`
	Headers   Mail      `json:"headers"`
	Subject   string    `json:"subject"`
	MessageID string    `json:"message_id"`
	// TODO: Populate
	// - Date
	// - Envelope
	// - Headers

	MailingListID int
	PatchsetID    *int
	ThreadID      *int
	ParentID      *int
	SenderID      *int

	alias  string
	fields *database.ModelFields
}

func (email *Email) Patch() *Patch {
	panic("TODO")
}

func (email *Email) As(alias string) *Email {
	email.alias = alias
	return email
}

func (email *Email) Alias() string {
	return email.alias
}

func (email *Email) Table() string {
	return "email"
}

func (email *Email) Fields() *database.ModelFields {
	if email.fields != nil {
		return email.fields
	}
	email.fields = &database.ModelFields{
		Fields: []*database.FieldMap{
			{ "id", "id", &email.ID },
			{ "created", "received", &email.Received },
			{ "message_date", "date", &email.Date },
			{ "body", "body", &email.Body },
			{ "subject", "subject", &email.Subject },
			{ "message_id", "message_id", &email.MessageID },

			// Always fetch:
			{ "id", "", &email.ID },
			{ "list_id", "", &email.MailingListID },
			{ "patchset_id", "", &email.PatchsetID },
			{ "thread_id", "", &email.ThreadID },
			{ "parent_id", "", &email.ParentID },
			{ "sender_id", "", &email.SenderID },
		},
	}
	return email.fields
}

func (email *Email) QueryWithCursor(ctx context.Context,
	runner sq.BaseRunner, q sq.SelectBuilder,
	cur *model.Cursor) ([]*Email, *model.Cursor) {
	var (
		err  error
		rows *sql.Rows
	)

	if cur.Next != "" {
		ts, _ := strconv.ParseInt(cur.Next, 10, 64)
		updated := time.Unix(ts, 0)
		q = q.Where(database.WithAlias(email.alias, "created") + "<= ?", updated)
	}
	q = q.
		OrderBy(database.WithAlias(email.alias, "created") + " DESC").
		Limit(uint64(cur.Count + 1))

	if rows, err = q.RunWith(runner).QueryContext(ctx); err != nil {
		panic(err)
	}
	defer rows.Close()

	var emails []*Email
	for rows.Next() {
		var email Email
		if err := rows.Scan(database.Scan(ctx, &email)...); err != nil {
			panic(err)
		}
		emails = append(emails, &email)
	}

	if len(emails) > cur.Count {
		cur = &model.Cursor{
			Count:  cur.Count,
			Next:   strconv.FormatInt(emails[len(emails)-1].Received.Unix(), 10),
			Search: cur.Search,
		}
		emails = emails[:cur.Count]
	} else {
		cur = nil
	}

	return emails, cur
}
