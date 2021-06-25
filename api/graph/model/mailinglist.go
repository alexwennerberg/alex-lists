package model

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"

	"git.sr.ht/~sircmpwn/core-go/database"
	"git.sr.ht/~sircmpwn/core-go/model"
)

const (
    ACCESS_NONE = 0
    ACCESS_BROWSE = 1
    ACCESS_REPLY = 2
    ACCESS_POST = 4
    ACCESS_MODERATE = 8
	ACCESS_ALL = 1 | 2 | 4 | 8
)

type MailingList struct {
	ID          int       `json:"id"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Importing   bool      `json:"importing"`

	OwnerID       int
	RawPermitMime string
	RawRejectMime string

	Permissions    int
	AccessID       *int
	SubscriptionID *int

	alias  string
	fields *database.ModelFields
}

func (list *MailingList) PermitMime() []string {
	if len(list.RawPermitMime) == 0 {
		return []string{}
	}
	return strings.Split(list.RawPermitMime, ",")
}

func (list *MailingList) RejectMime() []string {
	if len(list.RawRejectMime) == 0 {
		return []string{}
	}
	return strings.Split(list.RawRejectMime, ",")
}

func (list *MailingList) As(alias string) *MailingList {
	list.alias = alias
	return list
}

func (list *MailingList) Alias() string {
	return list.alias
}

func (list *MailingList) Table() string {
	return "list"
}

func (list *MailingList) Fields() *database.ModelFields {
	if list.fields != nil {
		return list.fields
	}
	list.fields = &database.ModelFields{
		Fields: []*database.FieldMap{
			{ "id", "id", &list.ID },
			{ "created", "created", &list.Created },
			{ "updated", "updated", &list.Updated },
			{ "name", "name", &list.Name },
			{ "description", "description", &list.Description },
			{ "import_in_progress", "importing", &list.Importing },
			{ "permit_mimetypes", "permit_mime", &list.RawPermitMime },
			{ "reject_mimetypes", "reject_mime", &list.RawRejectMime },

			// Always fetch:
			{ "id", "", &list.ID },
			{ "owner_id", "", &list.OwnerID },
		},
	}
	return list.fields
}

func (list *MailingList) QueryWithCursor(ctx context.Context,
	runner sq.BaseRunner, q sq.SelectBuilder,
	cur *model.Cursor) ([]*MailingList, *model.Cursor) {
	var (
		err  error
		rows *sql.Rows
	)

	if cur.Next != "" {
		ts, _ := strconv.ParseInt(cur.Next, 10, 64)
		updated := time.Unix(ts, 0)
		q = q.Where(database.WithAlias(list.alias, "updated") + "<= ?", updated)
	}
	q = q.
		OrderBy(database.WithAlias(list.alias, `updated`) + " DESC").
		Limit(uint64(cur.Count + 1))

	if rows, err = q.RunWith(runner).QueryContext(ctx); err != nil {
		panic(err)
	}
	defer rows.Close()

	var lists []*MailingList
	for rows.Next() {
		var list MailingList
		if err := rows.Scan(database.Scan(ctx, &list)...); err != nil {
			panic(err)
		}
		lists = append(lists, &list)
	}

	if len(lists) > cur.Count {
		cur = &model.Cursor{
			Count:  cur.Count,
			Next:   strconv.FormatInt(lists[len(lists)-1].Updated.Unix(), 10),
			Search: cur.Search,
		}
		lists = lists[:cur.Count]
	} else {
		cur = nil
	}

	return lists, cur
}
