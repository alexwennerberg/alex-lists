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

type GeneralACL struct {
	Browse   bool `json:"browse"`
	Reply    bool `json:"reply"`
	Post     bool `json:"post"`
	Moderate bool `json:"moderate"`
}

func (GeneralACL) IsACL() {}

type MailingListACL struct {
	ID      int       `json:"id"`
	Created time.Time `json:"created"`

	UserID        *int
	Email         *string
	MailingListID int
	RawAccess     uint

	alias  string
	fields *database.ModelFields
}

func (MailingListACL) IsACL() {}

func (acl *MailingListACL) As(alias string) *MailingListACL {
	acl.alias = alias
	return acl
}

func (acl *MailingListACL) Alias() string {
	return acl.alias
}

func (acl *MailingListACL) Table() string {
	return "access"
}

func (acl *MailingListACL) Browse() bool {
	return acl.RawAccess&ACCESS_BROWSE != 0
}

func (acl *MailingListACL) Reply() bool {
	return acl.RawAccess&ACCESS_REPLY != 0
}

func (acl *MailingListACL) Post() bool {
	return acl.RawAccess&ACCESS_POST != 0
}

func (acl *MailingListACL) Moderate() bool {
	return acl.RawAccess&ACCESS_MODERATE != 0
}

func (acl *MailingListACL) Fields() *database.ModelFields {
	if acl.fields != nil {
		return acl.fields
	}
	acl.fields = &database.ModelFields{
		Fields: []*database.FieldMap{
			{"created", "created", &acl.Created},

			// Always fetch:
			{"id", "", &acl.ID},
			{"permissions", "", &acl.RawAccess},
			{"list_id", "", &acl.MailingListID},
			{"user_id", "", &acl.UserID},
			{"email", "", &acl.Email},
		},
	}
	return acl.fields
}

func (acl *MailingListACL) QueryWithCursor(ctx context.Context,
	runner sq.BaseRunner, q sq.SelectBuilder,
	cur *model.Cursor) ([]ACL, *model.Cursor) {
	var (
		err  error
		rows *sql.Rows
	)

	if cur.Next != "" {
		ts, _ := strconv.ParseInt(cur.Next, 10, 64)
		created := time.Unix(ts, 0)
		q = q.Where(database.WithAlias(acl.alias, "created")+"<= ?", created)
	}
	q = q.
		OrderBy(database.WithAlias(acl.alias, `created`) + " DESC").
		Limit(uint64(cur.Count + 1))

	if rows, err = q.RunWith(runner).QueryContext(ctx); err != nil {
		panic(err)
	}
	defer rows.Close()

	var (
		acls   []ACL
		latest time.Time
	)
	for rows.Next() {
		var acl MailingListACL
		if err := rows.Scan(database.Scan(ctx, &acl)...); err != nil {
			panic(err)
		}
		latest = acl.Created
		acls = append(acls, &acl)
	}

	if len(acls) > cur.Count {
		cur = &model.Cursor{
			Count:  cur.Count,
			Next:   strconv.FormatInt(latest.Unix(), 10),
			Search: cur.Search,
		}
		acls = acls[:cur.Count]
	} else {
		cur = nil
	}

	return acls, cur
}
