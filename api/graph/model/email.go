package model

import (
	"bytes"
	"context"
	"database/sql"
	"log"
	"strconv"
	"time"

	"github.com/emersion/go-message/mail"
	_ "github.com/emersion/go-message/charset"
	sq "github.com/Masterminds/squirrel"

	"git.sr.ht/~sircmpwn/core-go/database"
	"git.sr.ht/~sircmpwn/core-go/model"
)

type Email struct {
	ID        int       `json:"id"`
	Received  time.Time `json:"received"`
	Body      string    `json:"body"`
	Subject   string    `json:"subject"`
	MessageID string    `json:"message_id"`
	InReplyTo *string   `json:"in_reply_to"`

	MailingListID int
	PatchsetID    *int
	ThreadID      *int
	ParentID      *int
	SenderID      *int

	RawEnvelope []byte
	RawHeader   mail.Header

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
			{ "created", "", &email.Received },
			{ "envelope", "", &email.RawEnvelope },
		},
	}
	return email.fields
}

func (email *Email) Populate() {
	reader, err := mail.CreateReader(bytes.NewBuffer(email.RawEnvelope))
	if err != nil {
		return
	}
	email.RawHeader = reader.Header
	if msgId, err := email.RawHeader.MessageID(); err != nil {
		log.Printf("Invalid message ID for email %d: %e", email.ID, err)
	} else {
		email.MessageID = msgId
	}
	if ids, _ := email.RawHeader.MsgIDList("In-Reply-To"); ids != nil {
		if len(ids) != 1 {
			log.Printf("Multiple In-Reply-To IDs for email %d", email.ID)
		}
		indir := ids[0]
		email.InReplyTo = &indir
	}
	reader.Close()
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
		Column(database.WithAlias(email.alias, "envelope")).
		Limit(uint64(cur.Count + 1))

	if rows, err = q.RunWith(runner).QueryContext(ctx); err != nil {
		panic(err)
	}
	defer rows.Close()

	var emails []*Email
	for rows.Next() {
		var (
			email Email
			data  string
		)
		if err := rows.Scan(append(
			database.Scan(ctx, &email),
			&data)...); err != nil {
			panic(err)
		}
		email.Populate()
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
