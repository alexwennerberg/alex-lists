package loaders

//go:generate ./gen UsersByIDLoader int api/graph/model.User
//go:generate ./gen UsersByNameLoader string api/graph/model.User
//go:generate ./gen MailingListsByIDLoader int api/graph/model.MailingList
//go:generate ./gen EmailsByIDLoader int api/graph/model.Email

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"git.sr.ht/~sircmpwn/core-go/auth"
	"git.sr.ht/~sircmpwn/core-go/database"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
)

var loadersCtxKey = &contextKey{"loaders"}

type contextKey struct {
	name string
}

type Loaders struct {
	UsersByID        UsersByIDLoader
	UsersByName      UsersByNameLoader
	MailingListsByID MailingListsByIDLoader
	EmailsByID       EmailsByIDLoader
}

func fetchUsersByID(ctx context.Context) func(ids []int) ([]*model.User, []error) {
	return func(ids []int) ([]*model.User, []error) {
		users := make([]*model.User, len(ids))
		if err := database.WithTx(ctx, &sql.TxOptions{
			Isolation: 0,
			ReadOnly: true,
		}, func (tx *sql.Tx) error {
			var (
				err  error
				rows *sql.Rows
			)
			query := database.
				Select(ctx, (&model.User{}).As(`u`)).
				From(`"user" u`).
				Where(sq.Expr(`u.id = ANY(?)`, pq.Array(ids)))
			if rows, err = query.RunWith(tx).QueryContext(ctx); err != nil {
				panic(err)
			}
			defer rows.Close()

			usersById := map[int]*model.User{}
			for rows.Next() {
				var user model.User
				if err := rows.Scan(database.Scan(ctx, &user)...); err != nil {
					panic(err)
				}
				usersById[user.ID] = &user
			}
			if err = rows.Err(); err != nil {
				panic(err)
			}

			for i, id := range ids {
				users[i] = usersById[id]
			}
			return nil
		}); err != nil {
			panic(err)
		}
		return users, nil
	}
}

func fetchUsersByName(ctx context.Context) func(names []string) ([]*model.User, []error) {
	return func(names []string) ([]*model.User, []error) {
		users := make([]*model.User, len(names))
		if err := database.WithTx(ctx, &sql.TxOptions{
			Isolation: 0,
			ReadOnly: true,
		}, func (tx *sql.Tx) error {
			var (
				err  error
				rows *sql.Rows
			)
			query := database.
				Select(ctx, (&model.User{}).As(`u`)).
				From(`"user" u`).
				Where(sq.Expr(`u.username = ANY(?)`, pq.Array(names)))
			if rows, err = query.RunWith(tx).QueryContext(ctx); err != nil {
				panic(err)
			}
			defer rows.Close()

			usersByName := map[string]*model.User{}
			for rows.Next() {
				user := model.User{}
				if err := rows.Scan(database.Scan(ctx, &user)...); err != nil {
					panic(err)
				}
				usersByName[user.Username] = &user
			}
			if err = rows.Err(); err != nil {
				panic(err)
			}

			for i, name := range names {
				users[i] = usersByName[name]
			}
			return nil
		}); err != nil {
			panic(err)
		}
		return users, nil
	}
}

func fetchMailingListsByID(ctx context.Context) func(ids []int) ([]*model.MailingList, []error) {
	return func(ids []int) ([]*model.MailingList, []error) {
		lists := make([]*model.MailingList, len(ids))
		if err := database.WithTx(ctx, &sql.TxOptions{
			Isolation: 0,
			ReadOnly: true,
		}, func (tx *sql.Tx) error {
			var (
				err  error
				rows *sql.Rows
			)
			// TODO: Test these auth bits
			user := auth.ForContext(ctx)
			query := database.
				Select(ctx, (&model.MailingList{}).As(`list`)).
				From(`list`).
				// XXX: We could fetch the rest of the ACL and subscription
				// details here and cache them to return via
				// MailingList { access, subscription }
				// if we were so inclined.
				LeftJoin(`access ON access.list_id = list.id`).
				LeftJoin(`subscription sub ON sub.list_id = list.id`).
				Column(`COALESCE(
					access.permissions,
					CASE WHEN list.owner_id = ?
					THEN ?
					ELSE CASE WHEN sub.id IS NOT NULL
						THEN list.subscriber_permissions
						ELSE null END
					END,
					list.nonsubscriber_permissions | list.account_permissions)`,
					user.UserID, model.ACCESS_ALL).
				Column(`access.id`).
				Column(`sub.id`).
				Where(sq.And{
					sq.Expr(`list.id = ANY(?)`, pq.Array(ids)),
					sq.Or{
						// List owner, or
						sq.Expr(`list.owner_id = ?`, user.UserID),
						// ACL entry exists, or
						sq.And{
							sq.Expr(`access.id IS NOT NULL`),
							sq.Expr(`access.permissions & ? > 0`, model.ACCESS_BROWSE),
						},
						// Subscribers, or
						sq.And{
							sq.Expr(`access.id IS NULL`),
							sq.Expr(`sub.id IS NULL`),
							sq.Expr(`list.nonsubscriber_permissions & ? > 0`, model.ACCESS_BROWSE),
						},
						// Or:
						sq.And{
							sq.Expr(`access.id IS NULL`),
							sq.Expr(`
								(list.subscriber_permissions | list.account_permissions) & ? > 0`,
								model.ACCESS_BROWSE,
							),
						},
					},
				})
			if rows, err = query.RunWith(tx).QueryContext(ctx); err != nil {
				panic(err)
			}
			defer rows.Close()

			listsByID := map[int]*model.MailingList{}
			for rows.Next() {
				list := model.MailingList{}
				if err := rows.Scan(append(
						database.Scan(ctx, &list),
						&list.Permissions,
						&list.AccessID,
						&list.SubscriptionID,
					)...); err != nil {
					panic(err)
				}
				listsByID[list.ID] = &list
			}
			if err = rows.Err(); err != nil {
				panic(err)
			}

			for i, id := range ids {
				lists[i] = listsByID[id]
			}
			return nil
		}); err != nil {
			panic(err)
		}
		return lists, nil
	}
}

func fetchEmailsByID(ctx context.Context) func(ids []int) ([]*model.Email, []error) {
	return func(ids []int) ([]*model.Email, []error) {
		emails := make([]*model.Email, len(ids))
		if err := database.WithTx(ctx, &sql.TxOptions{
			Isolation: 0,
			ReadOnly: true,
		}, func (tx *sql.Tx) error {
			var (
				err  error
				rows *sql.Rows
			)
			// TODO: Test these auth bits
			user := auth.ForContext(ctx)
			query := database.
				Select(ctx, (&model.Email{}).As(`email`)).
				From(`email`).
				LeftJoin(`list ON email.list_id = list.id`).
				LeftJoin(`access ON access.list_id = list.id`).
				LeftJoin(`subscription sub ON sub.list_id = list.id`).
				Column("envelope").
				Where(sq.And{
					sq.Expr(`email.id = ANY(?)`, pq.Array(ids)),
					sq.Or{
						// List owner, or
						sq.Expr(`list.owner_id = ?`, user.UserID),
						// ACL entry exists, or
						sq.And{
							sq.Expr(`access.id IS NOT NULL`),
							sq.Expr(`access.permissions & ? > 0`, model.ACCESS_BROWSE),
						},
						// Subscribers, or
						sq.And{
							sq.Expr(`access.id IS NULL`),
							sq.Expr(`sub.id IS NULL`),
							sq.Expr(`list.nonsubscriber_permissions & ? > 0`, model.ACCESS_BROWSE),
						},
						// Or:
						sq.And{
							sq.Expr(`access.id IS NULL`),
							sq.Expr(`
								(list.subscriber_permissions | list.account_permissions) & ? > 0`,
								model.ACCESS_BROWSE,
							),
						},
					},
				})
			if rows, err = query.RunWith(tx).QueryContext(ctx); err != nil {
				panic(err)
			}
			defer rows.Close()

			emailsByID := map[int]*model.Email{}
			for rows.Next() {
				var (
					email model.Email
					data  string
				)
				if err := rows.Scan(append(
						database.Scan(ctx, &email),
						&data,
					)...); err != nil {
					panic(err)
				}
				email.Populate(data)
				emailsByID[email.ID] = &email
			}
			if err = rows.Err(); err != nil {
				panic(err)
			}

			for i, id := range ids {
				emails[i] = emailsByID[id]
			}
			return nil
		}); err != nil {
			panic(err)
		}
		return emails, nil
	}
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), loadersCtxKey, &Loaders{
			UsersByID: UsersByIDLoader{
				maxBatch: 100,
				wait:     1 * time.Millisecond,
				fetch:    fetchUsersByID(r.Context()),
			},
			UsersByName: UsersByNameLoader{
				maxBatch: 100,
				wait:     1 * time.Millisecond,
				fetch:    fetchUsersByName(r.Context()),
			},
			MailingListsByID: MailingListsByIDLoader{
				maxBatch: 100,
				wait:     1 * time.Millisecond,
				fetch:    fetchMailingListsByID(r.Context()),
			},
			EmailsByID: EmailsByIDLoader{
				maxBatch: 100,
				wait:     1 * time.Millisecond,
				fetch:    fetchEmailsByID(r.Context()),
			},
		})
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func ForContext(ctx context.Context) *Loaders {
	raw, ok := ctx.Value(loadersCtxKey).(*Loaders)
	if !ok {
		panic(errors.New("Invalid data loaders context"))
	}
	return raw
}
