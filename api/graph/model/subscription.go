package model

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"

	"git.sr.ht/~sircmpwn/core-go/database"
	"git.sr.ht/~sircmpwn/core-go/model"
)

type MailingListSubscription struct {
	ID      int       `json:"id"`
	Created time.Time `json:"created"`

	UserID int
	ListID int

	alias  string
	fields *database.ModelFields
}

func (sub MailingListSubscription) IsSubscription() {
}

func (sub *MailingListSubscription) As(alias string) *MailingListSubscription {
	sub.alias = alias
	return sub
}

func (sub *MailingListSubscription) Alias() string {
	return sub.alias
}

func (sub *MailingListSubscription) Table() string {
	return "subscription"
}

func (sub *MailingListSubscription) Fields() *database.ModelFields {
	if sub.fields != nil {
		return sub.fields
	}
	sub.fields = &database.ModelFields{
		Fields: []*database.FieldMap{
			// Always fetch everything
			{"id", "", &sub.ID},
			{"created", "", &sub.Created},
			{"user_id", "", &sub.UserID},
			{"list_id", "", &sub.ListID},
		},
	}
	return sub.fields
}

func (sub *MailingListSubscription) QueryWithCursor(ctx context.Context,
	runner sq.BaseRunner, q sq.SelectBuilder,
	cur *model.Cursor) ([]Subscription, *model.Cursor) {
	var (
		err  error
		rows *sql.Rows
	)

	if cur.Next != "" {
		ts, _ := strconv.ParseInt(cur.Next, 10, 64)
		created := time.Unix(ts, 0)
		q = q.Where(database.WithAlias(sub.alias, "created")+"<= ?", created)
	}
	q = q.Limit(uint64(cur.Count + 1))

	if rows, err = q.RunWith(runner).QueryContext(ctx); err != nil {
		panic(err)
	}
	defer rows.Close()

	var (
		subs        []Subscription
		lastCreated time.Time
	)
	for rows.Next() {
		var sub MailingListSubscription
		if err := rows.Scan(database.Scan(ctx, &sub)...); err != nil {
			panic(err)
		}
		subs = append(subs, &sub)
		lastCreated = sub.Created
	}

	if len(subs) > cur.Count {
		cur = &model.Cursor{
			Count:  cur.Count,
			Next:   strconv.FormatInt(lastCreated.Unix(), 10),
			Search: cur.Search,
		}
		subs = subs[:cur.Count]
	} else {
		cur = nil
	}

	return subs, cur
}
