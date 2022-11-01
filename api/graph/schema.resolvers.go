package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"git.sr.ht/~emersion/go-emailthreads"
	"git.sr.ht/~sircmpwn/core-go/auth"
	"git.sr.ht/~sircmpwn/core-go/config"
	"git.sr.ht/~sircmpwn/core-go/database"
	coremodel "git.sr.ht/~sircmpwn/core-go/model"
	"git.sr.ht/~sircmpwn/core-go/server"
	"git.sr.ht/~sircmpwn/core-go/valid"
	corewebhooks "git.sr.ht/~sircmpwn/core-go/webhooks"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/account"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/lists"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/loaders"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/webhooks"
	"github.com/99designs/gqlgen/graphql"
	sq "github.com/Masterminds/squirrel"
	_ "github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Sender is the resolver for the sender field.
func (r *emailResolver) Sender(ctx context.Context, obj *model.Email) (model.Entity, error) {
	if obj.SenderID != nil {
		return loaders.ForContext(ctx).UsersByID.Load(*obj.SenderID)
	}
	list, err := obj.RawHeader.AddressList("From")
	if err != nil {
		return nil, err
	}
	if len(list) != 1 {
		panic(fmt.Errorf("Malformed email %d, multiple senders", obj.ID))
	}
	return &model.Mailbox{
		Name:    list[0].Name,
		Address: list[0].Address,
	}, nil
}

// Date is the resolver for the date field.
func (r *emailResolver) Date(ctx context.Context, obj *model.Email) (*time.Time, error) {
	date, err := obj.RawHeader.Date()
	if err != nil {
		return nil, nil
	}
	return &date, nil
}

// Header is the resolver for the header field.
func (r *emailResolver) Header(ctx context.Context, obj *model.Email, want string) ([]string, error) {
	var values []string
	iter := obj.RawHeader.FieldsByKey(want)
	for iter.Next() {
		text, err := iter.Text()
		if err != nil {
			return nil, err
		}
		values = append(values, text)
	}
	return values, nil
}

// AddressList is the resolver for the addressList field.
func (r *emailResolver) AddressList(ctx context.Context, obj *model.Email, want string) ([]*model.Mailbox, error) {
	list, err := obj.RawHeader.AddressList(want)
	if err != nil {
		return nil, err
	}
	var addrs []*model.Mailbox
	for _, item := range list {
		addrs = append(addrs, &model.Mailbox{
			Name:    item.Name,
			Address: item.Address,
		})
	}
	return addrs, nil
}

// Envelope is the resolver for the envelope field.
func (r *emailResolver) Envelope(ctx context.Context, obj *model.Email) (*model.URL, error) {
	origin := config.GetOrigin(config.ForContext(ctx), "lists.sr.ht", true)
	uri := fmt.Sprintf("%s/query/email/%d", origin, obj.ID)
	url, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}
	return &model.URL{url}, nil
}

// Thread is the resolver for the thread field.
func (r *emailResolver) Thread(ctx context.Context, obj *model.Email) (*model.Thread, error) {
	// Regarding the use of an unsafe loader: if you have access to the email
	// object, you have access to the thread also.
	if obj.ThreadID == nil {
		return loaders.ForContext(ctx).ThreadsByIDUnsafe.Load(obj.ID)
	}
	return loaders.ForContext(ctx).ThreadsByIDUnsafe.Load(*obj.ThreadID)
}

// Parent is the resolver for the parent field.
func (r *emailResolver) Parent(ctx context.Context, obj *model.Email) (*model.Email, error) {
	if obj.ParentID == nil {
		return nil, nil
	}
	// Regarding the use of an unsafe loader: if you have access to the email
	// object, you have access to its parent also.
	return loaders.ForContext(ctx).EmailsByIDUnsafe.Load(*obj.ParentID)
}

// Patchset is the resolver for the patchset field.
func (r *emailResolver) Patchset(ctx context.Context, obj *model.Email) (*model.Patchset, error) {
	if obj.PatchsetID == nil {
		return nil, nil
	}
	// Regarding the use of an unsafe loader: if you have access to the email
	// object, you have access to the patchset also.
	return loaders.ForContext(ctx).PatchsetsByIDUnsafe.Load(*obj.PatchsetID)
}

// List is the resolver for the list field.
func (r *emailResolver) List(ctx context.Context, obj *model.Email) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByID.Load(obj.MailingListID)
}

// Owner is the resolver for the owner field.
func (r *mailingListResolver) Owner(ctx context.Context, obj *model.MailingList) (model.Entity, error) {
	return loaders.ForContext(ctx).UsersByID.Load(obj.OwnerID)
}

// Threads is the resolver for the threads field.
func (r *mailingListResolver) Threads(ctx context.Context, obj *model.MailingList, cursor *coremodel.Cursor) (*model.ThreadCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var threads []*model.Thread
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		thread := (&model.Thread{}).As(`thread`)
		query := database.
			Select(ctx, thread).
			From(`email thread`).
			Where(`thread.list_id = ?`, obj.ID).
			Where(`thread.thread_id IS NULL`)
		threads, cursor = thread.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.ThreadCursor{threads, cursor}, nil
}

// Emails is the resolver for the emails field.
func (r *mailingListResolver) Emails(ctx context.Context, obj *model.MailingList, cursor *coremodel.Cursor) (*model.EmailCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var emails []*model.Email
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		email := (&model.Email{}).As(`email`)
		query := database.
			Select(ctx, email).
			From(`email`).
			Where(`email.list_id = ?`, obj.ID).
			OrderBy("email.created DESC")
		emails, cursor = email.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.EmailCursor{emails, cursor}, nil
}

// Patches is the resolver for the patches field.
func (r *mailingListResolver) Patches(ctx context.Context, obj *model.MailingList, cursor *coremodel.Cursor) (*model.PatchsetCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var patches []*model.Patchset
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		patch := (&model.Patchset{}).As(`patch`)
		query := database.
			Select(ctx, patch).
			From(`patchset patch`).
			Where(`patch.list_id = ?`, obj.ID)
		patches, cursor = patch.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.PatchsetCursor{patches, cursor}, nil
}

// Access is the resolver for the access field.
func (r *mailingListResolver) Access(ctx context.Context, obj *model.MailingList) (model.ACL, error) {
	if obj.AccessID != nil {
		return loaders.ForContext(ctx).ACLsByID.Load(*obj.AccessID)
	}
	p := obj.Access
	return &model.GeneralACL{
		Browse:   p&model.ACCESS_BROWSE != 0,
		Reply:    p&model.ACCESS_REPLY != 0,
		Post:     p&model.ACCESS_POST != 0,
		Moderate: p&model.ACCESS_MODERATE != 0,
	}, nil
}

// Subscription is the resolver for the subscription field.
func (r *mailingListResolver) Subscription(ctx context.Context, obj *model.MailingList) (*model.MailingListSubscription, error) {
	if obj.SubscriptionID == nil {
		return nil, nil
	}
	sub, err := loaders.ForContext(ctx).SubscriptionsByIDUnsafe.Load(*obj.SubscriptionID)
	if err != nil {
		return nil, err
	}
	return sub.(*model.MailingListSubscription), nil
}

// Archive is the resolver for the archive field.
func (r *mailingListResolver) Archive(ctx context.Context, obj *model.MailingList) (*model.URL, error) {
	origin := config.GetOrigin(config.ForContext(ctx), "lists.sr.ht", true)
	uri := fmt.Sprintf("%s/query/list/%d.mbox", origin, obj.ID)
	url, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}
	return &model.URL{url}, nil
}

// Last30days is the resolver for the last30days field.
func (r *mailingListResolver) Last30days(ctx context.Context, obj *model.MailingList) (*model.URL, error) {
	origin := config.GetOrigin(config.ForContext(ctx), "lists.sr.ht", true)
	uri := fmt.Sprintf("%s/query/list/%d.mbox?since=30", origin, obj.ID)
	url, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}
	return &model.URL{url}, nil
}

// ACL is the resolver for the acl field.
func (r *mailingListResolver) ACL(ctx context.Context, obj *model.MailingList, cursor *coremodel.Cursor) (*model.MailingListACLCursor, error) {
	if obj.OwnerID != auth.ForContext(ctx).UserID {
		return nil, fmt.Errorf("Access denied")
	}
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var acls []*model.MailingListACL
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		acl := (&model.MailingListACL{}).As(`acl`)
		query := database.
			Select(ctx, acl).
			From(`access acl`).
			Where(`acl.list_id = ?`, obj.ID)
		acls, cursor = acl.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.MailingListACLCursor{acls, cursor}, nil
}

// Webhooks is the resolver for the webhooks field.
func (r *mailingListResolver) Webhooks(ctx context.Context, obj *model.MailingList, cursor *coremodel.Cursor) (*model.WebhookSubscriptionCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	filter, err := corewebhooks.FilterWebhooks(ctx)
	if err != nil {
		return nil, err
	}

	var subs []model.WebhookSubscription
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		sub := (&model.MailingListWebhookSubscription{}).As(`sub`)
		query := database.
			Select(ctx, sub).
			From(`gql_list_wh_sub sub`).
			Where(sq.And{sq.Expr(`list_id = ?`, obj.ID), filter})
		subs, cursor = sub.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.WebhookSubscriptionCursor{subs, cursor}, nil
}

// Webhook is the resolver for the webhook field.
func (r *mailingListResolver) Webhook(ctx context.Context, obj *model.MailingList, id int) (model.WebhookSubscription, error) {
	var sub model.MailingListWebhookSubscription

	filter, err := corewebhooks.FilterWebhooks(ctx)
	if err != nil {
		return nil, err
	}

	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		row := database.
			Select(ctx, &sub).
			From(`gql_list_wh_sub`).
			Where(sq.And{
				sq.Expr(`id = ?`, id),
				sq.Expr(`list_id = ?`, obj.ID),
				filter,
			}).
			RunWith(tx).
			QueryRowContext(ctx)
		if err := row.Scan(database.Scan(ctx, &sub)...); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("No mailing list webhook by ID %d found for this user", id)
		}
		return nil, err
	}

	return &sub, nil
}

// List is the resolver for the list field.
func (r *mailingListACLResolver) List(ctx context.Context, obj *model.MailingListACL) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByID.Load(obj.MailingListID)
}

// Entity is the resolver for the entity field.
func (r *mailingListACLResolver) Entity(ctx context.Context, obj *model.MailingListACL) (model.Entity, error) {
	if obj.UserID != nil {
		return loaders.ForContext(ctx).UsersByID.Load(*obj.UserID)
	} else if obj.Email != nil {
		addr, err := mail.ParseAddress(*obj.Email)
		if err != nil {
			panic(err)
		}
		return &model.Mailbox{
			Name:    addr.Name,
			Address: addr.Address,
		}, nil
	} else {
		panic(fmt.Errorf("Invalid ACL record %d", obj.ID))
	}
}

// List is the resolver for the list field.
func (r *mailingListSubscriptionResolver) List(ctx context.Context, obj *model.MailingListSubscription) (*model.MailingList, error) {
	// XXX: We could use an unsafe resolver here if we wrote one
	return loaders.ForContext(ctx).MailingListsByID.Load(obj.ListID)
}

// Client is the resolver for the client field.
func (r *mailingListWebhookSubscriptionResolver) Client(ctx context.Context, obj *model.MailingListWebhookSubscription) (*model.OAuthClient, error) {
	if obj.ClientID == nil {
		return nil, nil
	}
	return &model.OAuthClient{
		UUID: *obj.ClientID,
	}, nil
}

// Deliveries is the resolver for the deliveries field.
func (r *mailingListWebhookSubscriptionResolver) Deliveries(ctx context.Context, obj *model.MailingListWebhookSubscription, cursor *coremodel.Cursor) (*model.WebhookDeliveryCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var deliveries []*model.WebhookDelivery
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		d := (&model.WebhookDelivery{}).
			WithName(`list`).
			As(`delivery`)
		query := database.
			Select(ctx, d).
			From(`gql_list_wh_delivery delivery`).
			Where(`delivery.subscription_id = ?`, obj.ID)
		deliveries, cursor = d.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.WebhookDeliveryCursor{deliveries, cursor}, nil
}

// Sample is the resolver for the sample field.
func (r *mailingListWebhookSubscriptionResolver) Sample(ctx context.Context, obj *model.MailingListWebhookSubscription, event model.WebhookEvent) (string, error) {
	payloadUUID := uuid.New()
	webhook := corewebhooks.WebhookContext{
		User:        auth.ForContext(ctx),
		PayloadUUID: payloadUUID,
		Name:        "list",
		Event:       event.String(),
		Subscription: &corewebhooks.WebhookSubscription{
			ID:         obj.ID,
			URL:        obj.URL,
			Query:      obj.Query,
			AuthMethod: obj.AuthMethod,
			TokenHash:  obj.TokenHash,
			Grants:     obj.Grants,
			ClientID:   obj.ClientID,
			Expires:    obj.Expires,
			NodeID:     obj.NodeID,
		},
	}

	auth := auth.ForContext(ctx)
	switch event {
	case model.WebhookEventListUpdated, model.WebhookEventListDeleted:
		desc := "Sample mailing list for testing webhooks"
		webhook.Payload = &model.MailingListEvent{
			UUID:  payloadUUID.String(),
			Event: event,
			Date:  time.Now().UTC(),
			List: &model.MailingList{
				ID:          -1,
				Created:     time.Now().UTC(),
				Updated:     time.Now().UTC(),
				Name:        "sample-list",
				Description: &desc,
				Visibility:  model.VisibilityPublic,

				OwnerID:        auth.UserID,
				RawPermitMime:  "",
				RawRejectMime:  "",
				Access:         model.ACCESS_ALL,
				DefaultAccess:  model.ACCESS_ALL,
				AccessID:       nil,
				SubscriptionID: nil,
			},
		}
	case model.WebhookEventEmailReceived:
		email := &model.Email{
			ID:        -1,
			Received:  time.Now().UTC(),
			Body:      "Sample email body\r\n",
			Subject:   "Sample email",
			MessageID: "970701.32784@example.com",
			InReplyTo: nil,
			Patch: model.Patch{
				Index:   nil,
				Count:   nil,
				Version: nil,
				Prefix:  nil,
				Subject: nil,
			},
			MailingListID: -1,
			PatchsetID:    nil,
			ThreadID:      nil,
			ParentID:      nil,
			SenderID:      nil,

			RawEnvelope: []byte("Mime-Version: 1.0\r\nContent-Transfer-Encoding: quoted-printable\r\nContent-Type: text/plain; charset=UTF-8\r\nSubject: Sample email\r\nFrom: <someone@example.com>\r\nTo: <sample-list@example.com>\r\nDate: Tue, 14 Jun 2022 09:31:03 +0000\r\nMessage-Id: <970701.32784@example.com>\r\n\r\nSample email body\r\n"),
			RawHeader:   mail.Header{},
		}
		email.Populate()
		webhook.Payload = &model.EmailEvent{
			UUID:  payloadUUID.String(),
			Event: event,
			Date:  time.Now().UTC(),
			Email: email,
		}
	case model.WebhookEventPatchsetReceived:
		webhook.Payload = &model.PatchsetEvent{
			UUID:  payloadUUID.String(),
			Event: event,
			Date:  time.Now().UTC(),
			Patchset: &model.Patchset{
				ID:             -1,
				Created:        time.Now().UTC(),
				Updated:        time.Now().UTC(),
				Subject:        "Sample patchset",
				Prefix:         nil,
				Version:        1,
				MailingListID:  -1,
				CoverLetterID:  nil,
				SupersededByID: nil,
				RawStatus:      "proposed",
			},
		}
	default:
		return "", fmt.Errorf("Unsupported event %s", event.String())
	}

	subctx := corewebhooks.Context(ctx, webhook.Payload)
	bytes, err := webhook.Exec(subctx, server.ForContext(ctx).Schema)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// List is the resolver for the list field.
func (r *mailingListWebhookSubscriptionResolver) List(ctx context.Context, obj *model.MailingListWebhookSubscription) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByID.Load(obj.ListID)
}

// CreateMailingList is the resolver for the createMailingList field.
func (r *mutationResolver) CreateMailingList(ctx context.Context, name string, description *string, visibility model.Visibility) (*model.MailingList, error) {
	valid := valid.New(ctx)
	valid.Expect(listNameRE.MatchString(name), "Name must match %s", listNameRE.String()).
		WithField("name").
		And(name != "." && name != ".." && name != ".git" && name != ".hg",
			"This is a reserved name and cannot be used for user mailing lists.").
		WithField("name")
	valid.Expect(description == nil || len(*description) < 2048,
		"Description must be fewer than 2048 characters").
		WithField("description")
	if !valid.Ok() {
		return nil, nil
	}

	var list model.MailingList
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO list (
				created, updated, name, description, visibility, owner_id
			) VALUES (
				NOW() at time zone 'utc',
				NOW() at time zone 'utc',
				$1, $2, $3, $4
			) RETURNING
				id, created, updated, name, description, visibility, owner_id,
				permit_mimetypes, reject_mimetypes, default_access;
		`, name, description, visibility.String(), auth.ForContext(ctx).UserID)

		if err := row.Scan(&list.ID, &list.Created, &list.Updated, &list.Name,
			&list.Description, &list.Visibility, &list.OwnerID,
			&list.RawPermitMime, &list.RawRejectMime,
			&list.DefaultAccess); err != nil {
			if err, ok := err.(*pq.Error); ok &&
				err.Code == "23505" && // unique_violation
				err.Constraint == "uq_list_owner_id_name" {
				return fmt.Errorf("A mailing list with this name already exists.")
			}
			return err
		}
		list.Access = model.ACCESS_ALL

		_, err := tx.ExecContext(ctx, `
			INSERT INTO subscription (
				created, updated, list_id, user_id
			) VALUES (
				NOW() at time zone 'utc',
				NOW() at time zone 'utc',
				$1, $2
			);
		`, list.ID, auth.ForContext(ctx).UserID)
		return err
	}); err != nil {
		return nil, err
	}

	webhooks.DeliverLegacyUserListEvent(ctx, &list, "list:create")
	webhooks.DeliverUserMailingListEvent(ctx, model.WebhookEventListCreated, &list)

	return &list, nil
}

// UpdateMailingList is the resolver for the updateMailingList field.
func (r *mutationResolver) UpdateMailingList(ctx context.Context, id int, input map[string]interface{}) (*model.MailingList, error) {
	valid := valid.New(ctx).WithInput(input)
	query := sq.Update(`list`).PlaceholderFormat(sq.Dollar)

	valid.OptionalString("description", func(desc string) {
		valid.Expect(len(desc) < 2048,
			"Description must be fewer than 2048 characters").
			WithField("description")
		query = query.Set("description", desc)
	})
	valid.OptionalString("visibility", func(visibility string) {
		query = query.Set("visibility", visibility)
	})
	mime := func(name string) {
		valid.Optional(name+"Mime", func(object interface{}) {
			list, ok := object.([]interface{})
			if !ok {
				panic("Invalid mime list") // GraphQL invariant
			}
			items := make([]string, len(list))
			for i, item := range list {
				str, ok := item.(string)
				if !ok {
					panic("Invalid mime list") // GraphQL invariant
				}
				items[i] = str
			}
			// TODO: This should be updated to a native Postgres array type
			query = query.Set(name+"_mimetypes", strings.Join(items, ","))
		})
	}
	mime("permit")
	mime("reject")
	if !valid.Ok() {
		return nil, nil
	}

	var list model.MailingList
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := query.
			Where(`list.id = ? AND list.owner_id = ?`,
				id, auth.ForContext(ctx).UserID).
			Suffix(`RETURNING
				id, created, updated, name, description, visibility, owner_id,
				permit_mimetypes, reject_mimetypes, default_access`).
			RunWith(tx).
			QueryRowContext(ctx)

		if err := row.Scan(&list.ID, &list.Created, &list.Updated, &list.Name,
			&list.Description, &list.Visibility, &list.OwnerID,
			&list.RawPermitMime, &list.RawRejectMime,
			&list.DefaultAccess); err != nil {
			return err
		}
		list.Access = model.ACCESS_ALL
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	webhooks.DeliverLegacyListEvent(ctx, &list, "list:update")
	webhooks.DeliverUserMailingListEvent(ctx, model.WebhookEventListUpdated, &list)
	webhooks.DeliverMailingListEvent(ctx, model.WebhookEventListUpdated, &list)

	return &list, nil
}

// DeleteMailingList is the resolver for the deleteMailingList field.
func (r *mutationResolver) DeleteMailingList(ctx context.Context, id int) (*model.MailingList, error) {
	var list model.MailingList
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		// XXX: It would be nice if we generalized database.Scan a little bit
		// so it can work with queries other than Select. Might call for
		// forking squirrel to add PostgreSQL-specific features.
		row := tx.QueryRowContext(ctx, `
			DELETE FROM list
			WHERE id = $1 AND owner_id = $2
			RETURNING
				id, created, updated, name, description, visibility, owner_id,
				permit_mimetypes, reject_mimetypes, default_access;`,
			id, auth.ForContext(ctx).UserID)
		if err := row.Scan(&list.ID, &list.Created, &list.Updated, &list.Name,
			&list.Description, &list.Visibility, &list.OwnerID,
			&list.RawPermitMime, &list.RawRejectMime, &list.DefaultAccess); err != nil {
			return err
		}
		list.Access = model.ACCESS_ALL

		// We need to do this here so that it picks up the subscription list
		// before the cascade sets their list_id columns to null.
		webhooks.DeliverLegacyListEvent(ctx, &list, "list:delete")
		webhooks.DeliverUserMailingListEvent(ctx, model.WebhookEventListDeleted, &list)
		webhooks.DeliverMailingListEvent(ctx, model.WebhookEventListDeleted, &list)
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &list, nil
}

// UpdateUserACL is the resolver for the updateUserACL field.
func (r *mutationResolver) UpdateUserACL(ctx context.Context, listID int, userID int, input model.ACLInput) (*model.MailingListACL, error) {
	bits := ACLInputBits(input)
	var acl model.MailingListACL
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO access (
				created, updated, list_id, user_id, permissions
			) VALUES (
				NOW() at time zone 'utc',
				NOW() at time zone 'utc',
				-- The purpose of this is to filter out lists that the user is
				-- not an owner of. Saves us a round-trip
				(SELECT id FROM list WHERE id = $1 AND owner_id = $4),
				$2, $3
			)
			ON CONFLICT ON CONSTRAINT uq_access_list_id_user_id
			DO UPDATE SET
				updated = NOW() at time zone 'utc',
				permissions = $3
			RETURNING id, created, list_id, user_id, permissions;
		`, listID, userID, bits, auth.ForContext(ctx).UserID)
		if err := row.Scan(&acl.ID, &acl.Created, &acl.MailingListID,
			&acl.UserID, &acl.RawAccess); err != nil {
			if err, ok := err.(*pq.Error); ok &&
				err.Code == "23502" && // not_null_violation
				err.Column == "list_id" {
				return sql.ErrNoRows
			} else if ok &&
				err.Code == "23503" && // foreign_key_violation
				err.Constraint == "access_list_id_fkey" {
				return sql.ErrNoRows
			}
			return err
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &acl, nil
}

// UpdateSenderACL is the resolver for the updateSenderACL field.
func (r *mutationResolver) UpdateSenderACL(ctx context.Context, listID int, address string, input model.ACLInput) (*model.MailingListACL, error) {
	bits := ACLInputBits(input)
	var acl model.MailingListACL
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO access (
				created, updated, list_id, email, permissions
			) VALUES (
				NOW() at time zone 'utc',
				NOW() at time zone 'utc',
				-- The purpose of this is to filter out lists that the user is
				-- not an owner of. Saves us a round-trip
				(SELECT id FROM list WHERE id = $1 AND owner_id = $4),
				$2, $3
			)
			ON CONFLICT ON CONSTRAINT uq_access_list_id_email
			DO UPDATE SET
				updated = NOW() at time zone 'utc',
				permissions = $3
			RETURNING id, created, list_id, email, permissions;
		`, listID, address, bits, auth.ForContext(ctx).UserID)
		if err := row.Scan(&acl.ID, &acl.Created, &acl.MailingListID,
			&acl.Email, &acl.RawAccess); err != nil {
			if err, ok := err.(*pq.Error); ok &&
				err.Code == "23502" && // not_null_violation
				err.Column == "list_id" {
				return sql.ErrNoRows
			} else if ok &&
				err.Code == "23503" && // foreign_key_violation
				err.Constraint == "access_list_id_fkey" {
				return sql.ErrNoRows
			}
			return err
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &acl, nil
}

// UpdateMailingListACL is the resolver for the updateMailingListACL field.
func (r *mutationResolver) UpdateMailingListACL(ctx context.Context, listID int, input model.ACLInput) (*model.MailingList, error) {
	bits := ACLInputBits(input)
	var list model.MailingList
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			UPDATE list SET default_access = $1
			WHERE id = $2 AND owner_id = $3
			RETURNING
				id, created, updated, name, description, visibility, owner_id,
				permit_mimetypes, reject_mimetypes, default_access;
		`, bits, listID, auth.ForContext(ctx).UserID)
		if err := row.Scan(&list.ID, &list.Created, &list.Updated, &list.Name,
			&list.Description, &list.Visibility, &list.OwnerID,
			&list.RawPermitMime, &list.RawRejectMime, &list.DefaultAccess); err != nil {
			return err
		}
		list.Access = model.ACCESS_ALL
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	webhooks.DeliverLegacyListEvent(ctx, &list, "list:update")
	webhooks.DeliverUserMailingListEvent(ctx, model.WebhookEventListUpdated, &list)
	webhooks.DeliverMailingListEvent(ctx, model.WebhookEventListUpdated, &list)
	return &list, nil
}

// DeleteACL is the resolver for the deleteACL field.
func (r *mutationResolver) DeleteACL(ctx context.Context, id int) (*model.MailingListACL, error) {
	var acl model.MailingListACL
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			DELETE FROM access
			USING list
			WHERE
				access.list_id = list.id AND
				access.id = $1 AND
				list.owner_id = $2
			RETURNING
				access.id, access.created, access.list_id, access.user_id,
				access.email, access.permissions;
		`, id, auth.ForContext(ctx).UserID)
		if err := row.Scan(&acl.ID, &acl.Created, &acl.MailingListID,
			&acl.UserID, &acl.Email, &acl.RawAccess); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &acl, nil
}

// UpdatePatchset is the resolver for the updatePatchset field.
func (r *mutationResolver) UpdatePatchset(ctx context.Context, id int, status model.PatchsetStatus) (*model.Patchset, error) {
	var patchset model.Patchset
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			WITH lists AS (
				SELECT id
				FROM list
				WHERE owner_id = $1
			)
			UPDATE patchset
			SET status = $3
			WHERE
				list_id in (SELECT id FROM lists) AND
				id = $2
			RETURNING
				id, created, updated, subject, prefix, version, status,
				list_id, cover_letter_id, superseded_by_id;
		`, auth.ForContext(ctx).UserID, id, strings.ToLower(status.String()))
		if err := row.Scan(&patchset.ID, &patchset.Created, &patchset.Updated,
			&patchset.Subject, &patchset.Prefix, &patchset.Version,
			&patchset.RawStatus, &patchset.MailingListID,
			&patchset.CoverLetterID, &patchset.SupersededByID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &patchset, nil
}

// CreateTool is the resolver for the createTool field.
func (r *mutationResolver) CreateTool(ctx context.Context, patchsetID int, details string, icon model.ToolIcon) (*model.PatchsetTool, error) {
	var tool model.PatchsetTool
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			WITH patch AS (
				SELECT patchset.id
				FROM patchset
				JOIN list ON list.id = patchset.list_id
				LEFT JOIN access
					ON access.user_id = $2 AND
					access.list_id = list.id
				WHERE patchset.id = $1 AND (
					list.owner_id = $2 OR
					(access.id IS NOT NULL AND access.permissions & $3 > 0))
			)
			INSERT INTO patchset_tool (
				created, updated, patchset_id, key, icon, details
			) VALUES (
				NOW() at time zone 'utc',
				NOW() at time zone 'utc',
				(SELECT id FROM patch), 'graphql',
				$4, $5
			)
			RETURNING id, created, updated, icon, details, patchset_id;`,
			patchsetID, auth.ForContext(ctx).UserID,
			model.ACCESS_MODERATE, strings.ToLower(icon.String()), details)
		return row.Scan(&tool.ID, &tool.Created, &tool.Updated,
			&tool.RawIcon, &tool.Details, &tool.PatchsetID)
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &tool, nil
}

// UpdateTool is the resolver for the updateTool field.
func (r *mutationResolver) UpdateTool(ctx context.Context, id int, details *string, icon *model.ToolIcon) (*model.PatchsetTool, error) {
	var tool model.PatchsetTool
	query := sq.Update(`patchset_tool`).PlaceholderFormat(sq.Dollar)
	if details != nil {
		query = query.Set("details", *details)
	}
	if icon != nil {
		query = query.Set("icon", strings.ToLower(icon.String()))
	}

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		userID := auth.ForContext(ctx).UserID
		row := query.
			Prefix(`WITH patch AS (
				SELECT tool.id
				FROM patchset_tool tool
				JOIN patchset ON patchset.id = tool.patchset_id
				JOIN list ON list.id = patchset.list_id
				LEFT JOIN access
					ON access.user_id = ? AND
					access.list_id = list.id
				WHERE tool.id = ? AND (list.owner_id = ? OR
					(access.id IS NOT NULL AND access.permissions & ? > 0))
			)`, userID, id, userID, model.ACCESS_MODERATE).
			Where(`patchset_tool.id = (SELECT id FROM patch)`).
			Suffix(`RETURNING id, created, updated, icon, details, patchset_id`).
			RunWith(tx).
			QueryRowContext(ctx)
		return row.Scan(&tool.ID, &tool.Created, &tool.Updated,
			&tool.RawIcon, &tool.Details, &tool.PatchsetID)
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &tool, nil
}

// MailingListSubscribe is the resolver for the mailingListSubscribe field.
func (r *mutationResolver) MailingListSubscribe(ctx context.Context, listID int) (*model.MailingListSubscription, error) {
	var sub model.MailingListSubscription
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			WITH list AS (
				SELECT list.id
				FROM list
				LEFT JOIN access ON access.user_id = $1 AND access.list_id = list.id
				WHERE list.id = $2 AND (
					list.owner_id = $1 OR
					(access.id IS NOT NULL AND access.permissions & $3 > 0) OR
					(access.id IS NULL AND list.default_access & $3 > 0)
				)
			) INSERT INTO subscription (
				created, updated, user_id, list_id
			) VALUES (
				NOW() at time zone 'utc',
				NOW() at time zone 'utc',
				$1, (SELECT id FROM list)
			)
			ON CONFLICT ON CONSTRAINT subscription_list_id_user_id_unique
			DO UPDATE SET updated = NOW() at time zone 'utc'
			RETURNING id, created, user_id, list_id;`,
			auth.ForContext(ctx).UserID, listID, model.ACCESS_BROWSE)
		if err := row.Scan(&sub.ID, &sub.Created, &sub.UserID, &sub.ListID); err != nil {
			if err, ok := err.(*pq.Error); ok &&
				err.Code == "23502" &&
				err.Column == "list_id" { // not_null_violation
				return sql.ErrNoRows
			}
			return err
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

// MailingListUnsubscribe is the resolver for the mailingListUnsubscribe field.
func (r *mutationResolver) MailingListUnsubscribe(ctx context.Context, listID int) (*model.MailingListSubscription, error) {
	var sub model.MailingListSubscription
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			DELETE FROM subscription
			WHERE list_id = $1 AND user_id = $2
			RETURNING id, created, user_id, list_id;
		`, listID, auth.ForContext(ctx).UserID)
		return row.Scan(&sub.ID, &sub.Created, &sub.UserID, &sub.ListID)
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

// ImportMailingListSpool is the resolver for the importMailingListSpool field.
func (r *mutationResolver) ImportMailingListSpool(ctx context.Context, listID int, spool graphql.Upload) (bool, error) {
	const limit = 104857600 // 100 MiB
	if spool.Size > limit {
		return false, errors.New("Mailing list spool must not exceed 30 MiB in size")
	}
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE list
			SET import_in_progress = true
			WHERE id = $1 AND owner_id = $2
		`, listID, auth.ForContext(ctx).UserID)
		return err
	}); err != nil {
		return false, err
	}

	b, err := io.ReadAll(io.LimitReader(spool.File, limit))
	if err != nil {
		return false, err
	}
	lists.ImportMailingListSpool(ctx, listID, bytes.NewReader(b))
	return true, nil
}

// CreateUserWebhook is the resolver for the createUserWebhook field.
func (r *mutationResolver) CreateUserWebhook(ctx context.Context, config model.UserWebhookInput) (model.WebhookSubscription, error) {
	schema := server.ForContext(ctx).Schema
	if err := corewebhooks.Validate(schema, config.Query); err != nil {
		return nil, err
	}

	user := auth.ForContext(ctx)
	ac, err := corewebhooks.NewAuthConfig(ctx)
	if err != nil {
		return nil, err
	}

	var sub model.UserWebhookSubscription
	if len(config.Events) == 0 {
		return nil, fmt.Errorf("Must specify at least one event")
	}
	events := make([]string, len(config.Events))
	for i, ev := range config.Events {
		events[i] = ev.String()
		// TODO: gqlgen does not support doing anything useful with directives
		// on enums at the time of writing, so we have to do a little bit of
		// manual fuckery
		var access string
		switch ev {
		case model.WebhookEventListCreated, model.WebhookEventListUpdated,
			model.WebhookEventListDeleted:
			access = "LISTS"
		case model.WebhookEventEmailReceived:
			access = "EMAILS"
		case model.WebhookEventPatchsetReceived:
			access = "PATCHES"
		default:
			return nil, fmt.Errorf("Unsupported event %s", ev.String())
		}
		if !user.Grants.Has(access, auth.RO) {
			return nil, fmt.Errorf("Insufficient access granted for webhook event %s", ev.String())
		}
	}

	u, err := url.Parse(config.URL)
	if err != nil {
		return nil, err
	} else if u.Host == "" {
		return nil, fmt.Errorf("Cannot use URL without host")
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("Cannot use non-HTTP or HTTPS URL")
	}

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO gql_user_wh_sub (
				created, events, url, query,
				auth_method,
				token_hash, grants, client_id, expires,
				node_id,
				user_id
			) VALUES (
				NOW() at time zone 'utc',
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
			) RETURNING id, url, query, events, user_id;`,
			pq.Array(events), config.URL, config.Query,
			ac.AuthMethod,
			ac.TokenHash, ac.Grants, ac.ClientID, ac.Expires, // OAUTH2
			ac.NodeID, // INTERNAL
			user.UserID)

		if err := row.Scan(&sub.ID, &sub.URL,
			&sub.Query, pq.Array(&sub.Events), &sub.UserID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &sub, nil
}

// DeleteUserWebhook is the resolver for the deleteUserWebhook field.
func (r *mutationResolver) DeleteUserWebhook(ctx context.Context, id int) (model.WebhookSubscription, error) {
	var sub model.UserWebhookSubscription

	filter, err := corewebhooks.FilterWebhooks(ctx)
	if err != nil {
		return nil, err
	}

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := sq.Delete(`gql_user_wh_sub`).
			PlaceholderFormat(sq.Dollar).
			Where(sq.And{sq.Expr(`id = ?`, id), filter}).
			Suffix(`RETURNING id, url, query, events, user_id`).
			RunWith(tx).
			QueryRowContext(ctx)
		if err := row.Scan(&sub.ID, &sub.URL,
			&sub.Query, pq.Array(&sub.Events), &sub.UserID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("No user webhook by ID %d found for this user", id)
		}
		return nil, err
	}

	return &sub, nil
}

// CreateMailingListWebhook is the resolver for the createMailingListWebhook field.
func (r *mutationResolver) CreateMailingListWebhook(ctx context.Context, listID int, config model.MailingListWebhookInput) (model.WebhookSubscription, error) {
	schema := server.ForContext(ctx).Schema
	if err := corewebhooks.Validate(schema, config.Query); err != nil {
		return nil, err
	}

	user := auth.ForContext(ctx)
	ac, err := corewebhooks.NewAuthConfig(ctx)
	if err != nil {
		return nil, err
	}

	var sub model.MailingListWebhookSubscription
	if len(config.Events) == 0 {
		return nil, fmt.Errorf("Must specify at least one event")
	}
	events := make([]string, len(config.Events))
	for i, ev := range config.Events {
		events[i] = ev.String()
		// TODO: gqlgen does not support doing anything useful with directives
		// on enums at the time of writing, so we have to do a little bit of
		// manual fuckery
		var access string
		switch ev {
		case model.WebhookEventListUpdated, model.WebhookEventListDeleted:
			access = "LISTS"
		case model.WebhookEventEmailReceived:
			access = "EMAILS"
		case model.WebhookEventPatchsetReceived:
			access = "PATCHES"
		default:
			return nil, fmt.Errorf("Unsupported event %s", ev.String())
		}
		if !user.Grants.Has(access, auth.RO) {
			return nil, fmt.Errorf("Insufficient access granted for webhook event %s", ev.String())
		}
	}

	u, err := url.Parse(config.URL)
	if err != nil {
		return nil, err
	} else if u.Host == "" {
		return nil, fmt.Errorf("Cannot use URL without host")
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("Cannot use non-HTTP or HTTPS URL")
	}

	list, err := loaders.ForContext(ctx).MailingListsByID.Load(listID)
	if err != nil {
		return nil, err
	} else if list == nil {
		return nil, errors.New("Access denied")
	}

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO gql_list_wh_sub (
				created, events, url, query,
				auth_method,
				token_hash, grants, client_id, expires,
				node_id,
				user_id,
				list_id
			) VALUES (
				NOW() at time zone 'utc',
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
			) RETURNING id, url, query, events, user_id, list_id;`,
			pq.Array(events), config.URL, config.Query,
			ac.AuthMethod,
			ac.TokenHash, ac.Grants, ac.ClientID, ac.Expires, // OAUTH2
			ac.NodeID, // INTERNAL
			user.UserID,
			list.ID)

		if err := row.Scan(&sub.ID, &sub.URL,
			&sub.Query, pq.Array(&sub.Events), &sub.UserID, &sub.ListID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &sub, nil
}

// DeleteMailingListWebhook is the resolver for the deleteMailingListWebhook field.
func (r *mutationResolver) DeleteMailingListWebhook(ctx context.Context, id int) (model.WebhookSubscription, error) {
	var sub model.MailingListWebhookSubscription

	filter, err := corewebhooks.FilterWebhooks(ctx)
	if err != nil {
		return nil, err
	}

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := sq.Delete(`gql_list_wh_sub`).
			PlaceholderFormat(sq.Dollar).
			Where(sq.And{sq.Expr(`id = ?`, id), filter}).
			Suffix(`RETURNING id, url, query, events, user_id, list_id`).
			RunWith(tx).
			QueryRowContext(ctx)
		if err := row.Scan(&sub.ID, &sub.URL,
			&sub.Query, pq.Array(&sub.Events), &sub.UserID, &sub.ListID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("No mailing list webhook by ID %d found for this user", id)
		}
		return nil, err
	}

	return &sub, nil
}

// TriggerUserEmailWebhooks is the resolver for the triggerUserEmailWebhooks field.
func (r *mutationResolver) TriggerUserEmailWebhooks(ctx context.Context, emailID int) (*model.Email, error) {
	email, err := loaders.ForContext(ctx).EmailsByID.Load(emailID)
	if err != nil {
		return nil, err
	}
	webhooks.DeliverUserEmailEvent(ctx, model.WebhookEventEmailReceived, email)
	return email, nil
}

// TriggerListEmailWebhooks is the resolver for the triggerListEmailWebhooks field.
func (r *mutationResolver) TriggerListEmailWebhooks(ctx context.Context, listID int, emailID int) (*model.Email, error) {
	email, err := loaders.ForContext(ctx).EmailsByID.Load(emailID)
	if err != nil {
		return nil, err
	}
	webhooks.DeliverListEmailEvent(ctx, listID, model.WebhookEventEmailReceived, email)
	if email.PatchsetID != nil {
		patchset, err := loaders.ForContext(ctx).PatchsetsByID.Load(*email.PatchsetID)
		if err != nil {
			return nil, err
		}
		webhooks.DeliverListPatchsetEvent(ctx, listID, model.WebhookEventPatchsetReceived, patchset)
	}
	return email, nil
}

// DeleteUser is the resolver for the deleteUser field.
func (r *mutationResolver) DeleteUser(ctx context.Context) (int, error) {
	user := auth.ForContext(ctx)
	account.Delete(ctx, user.UserID, user.Username)
	return user.UserID, nil
}

// Submitter is the resolver for the submitter field.
func (r *patchsetResolver) Submitter(ctx context.Context, obj *model.Patchset) (model.Entity, error) {
	// XXX: It would be nice if we didn't have to fetch the thread details in
	// order to get the patchset submitter. The database has a submitter field
	// but it's not very useful.
	var submitter model.Entity

	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		var (
			err      error
			envelope []byte
			senderID *int
		)
		row := tx.QueryRowContext(ctx, `
			SELECT envelope, sender_id
			FROM email
			WHERE
				email.thread_id IS NULL AND
				email.patchset_id = $1
		`, obj.ID)

		if err = row.Scan(&envelope, &senderID); err != nil {
			return err
		}

		if senderID != nil {
			submitter, err = loaders.ForContext(ctx).UsersByID.Load(*senderID)
			return err
		}

		reader, err := mail.CreateReader(bytes.NewBuffer(envelope))
		if err != nil {
			panic(err)
		}
		defer reader.Close()

		list, err := reader.Header.AddressList("From")
		if err != nil {
			return err
		}
		if len(list) != 1 {
			panic(fmt.Errorf("Malformed email %d, multiple senders", obj.ID))
		}

		submitter = &model.Mailbox{
			Name:    list[0].Name,
			Address: list[0].Address,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return submitter, nil
}

// CoverLetter is the resolver for the coverLetter field.
func (r *patchsetResolver) CoverLetter(ctx context.Context, obj *model.Patchset) (*model.Email, error) {
	if obj.CoverLetterID == nil {
		return nil, nil
	}
	return loaders.ForContext(ctx).EmailsByID.Load(*obj.CoverLetterID)
}

// Thread is the resolver for the thread field.
func (r *patchsetResolver) Thread(ctx context.Context, obj *model.Patchset) (*model.Thread, error) {
	var thread model.Thread

	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		// Note that no authentication is required here because anyone with
		// access to the patchset also has access to the thread.
		row := database.
			Select(ctx, &thread).
			From(`email thread`).
			Where(`thread.patchset_id = ? AND thread.thread_id IS NULL`, obj.ID).
			RunWith(tx).
			QueryRowContext(ctx)
		if err := row.Scan(database.Scan(ctx, &thread)...); err != nil {
			return err
		}
		thread.Populate()
		return nil
	}); err != nil {
		return nil, err
	}

	return &thread, nil
}

// SupersededBy is the resolver for the supersededBy field.
func (r *patchsetResolver) SupersededBy(ctx context.Context, obj *model.Patchset) (*model.Patchset, error) {
	// TODO: This feature has not been completed
	return nil, nil
}

// List is the resolver for the list field.
func (r *patchsetResolver) List(ctx context.Context, obj *model.Patchset) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByID.Load(obj.MailingListID)
}

// Patches is the resolver for the patches field.
func (r *patchsetResolver) Patches(ctx context.Context, obj *model.Patchset, cursor *coremodel.Cursor) (*model.EmailCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var emails []*model.Email
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		email := (&model.Email{}).As(`email`)
		query := database.
			Select(ctx, email).
			From(`email`).
			Where(`email.patchset_id = ? AND email.is_patch`, obj.ID).
			OrderBy("email.created")
		emails, cursor = email.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.EmailCursor{emails, cursor}, nil
}

// Tools is the resolver for the tools field.
func (r *patchsetResolver) Tools(ctx context.Context, obj *model.Patchset) ([]*model.PatchsetTool, error) {
	var tools []*model.PatchsetTool

	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		// No authentication required because anyone who has access to the
		// patchset also has access to the tools.
		tool := (&model.PatchsetTool{}).As(`tool`)
		rows, err := database.
			Select(ctx, tool).
			From(`patchset_tool tool`).
			Where(`tool.patchset_id = ?`, obj.ID).
			RunWith(tx).
			QueryContext(ctx)
		if err != nil {
			return err
		}

		for rows.Next() {
			var tool model.PatchsetTool
			if err := rows.Scan(database.Scan(ctx, &tool)...); err != nil {
				return err
			}
			tools = append(tools, &tool)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return tools, nil
}

// Mbox is the resolver for the mbox field.
func (r *patchsetResolver) Mbox(ctx context.Context, obj *model.Patchset) (*model.URL, error) {
	origin := config.GetOrigin(config.ForContext(ctx), "lists.sr.ht", true)
	uri := fmt.Sprintf("%s/query/patchset/%d.mbox", origin, obj.ID)
	url, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}
	return &model.URL{url}, nil
}

// Patchset is the resolver for the patchset field.
func (r *patchsetToolResolver) Patchset(ctx context.Context, obj *model.PatchsetTool) (*model.Patchset, error) {
	return loaders.ForContext(ctx).PatchsetsByID.Load(obj.PatchsetID)
}

// Version is the resolver for the version field.
func (r *queryResolver) Version(ctx context.Context) (*model.Version, error) {
	return &model.Version{
		Major:           0,
		Minor:           0,
		Patch:           0,
		DeprecationDate: nil,
	}, nil
}

// Me is the resolver for the me field.
func (r *queryResolver) Me(ctx context.Context) (*model.User, error) {
	user := auth.ForContext(ctx)
	return &model.User{
		ID:       user.UserID,
		Created:  user.Created,
		Updated:  user.Updated,
		Username: user.Username,
		Email:    user.Email,
		URL:      user.URL,
		Location: user.Location,
		Bio:      user.Bio,
	}, nil
}

// User is the resolver for the user field.
func (r *queryResolver) User(ctx context.Context, id int) (*model.User, error) {
	return loaders.ForContext(ctx).UsersByID.Load(id)
}

// UserByName is the resolver for the userByName field.
func (r *queryResolver) UserByName(ctx context.Context, username string) (*model.User, error) {
	return loaders.ForContext(ctx).UsersByName.Load(username)
}

// MailingList is the resolver for the mailingList field.
func (r *queryResolver) MailingList(ctx context.Context, id int) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByID.Load(id)
}

// MailingListByName is the resolver for the mailingListByName field.
func (r *queryResolver) MailingListByName(ctx context.Context, name string) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByName.Load(name)
}

// MailingListByOwner is the resolver for the mailingListByOwner field.
func (r *queryResolver) MailingListByOwner(ctx context.Context, ownerName string, listName string) (*model.MailingList, error) {
	if strings.HasPrefix(ownerName, "~") {
		ownerName = ownerName[1:]
	} else {
		return nil, fmt.Errorf("Expected owner to be a canonical name")
	}
	return loaders.ForContext(ctx).MailingListsByOwnerName.Load([2]string{ownerName, listName})
}

// Email is the resolver for the email field.
func (r *queryResolver) Email(ctx context.Context, id int) (*model.Email, error) {
	return loaders.ForContext(ctx).EmailsByID.Load(id)
}

// Message is the resolver for the message field.
func (r *queryResolver) Message(ctx context.Context, messageID string) (*model.Email, error) {
	return loaders.ForContext(ctx).EmailsByMessageID.Load(messageID)
}

// Patchset is the resolver for the patchset field.
func (r *queryResolver) Patchset(ctx context.Context, id int) (*model.Patchset, error) {
	return loaders.ForContext(ctx).PatchsetsByID.Load(id)
}

// MailingLists is the resolver for the mailingLists field.
func (r *queryResolver) MailingLists(ctx context.Context, cursor *coremodel.Cursor) (*model.MailingListCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var lists []*model.MailingList
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		list := (&model.MailingList{}).As(`mailing_list`)
		query := database.
			Select(ctx, list).
			From(`list mailing_list`).
			Where(`mailing_list.owner_id = ?`, auth.ForContext(ctx).UserID)
		lists, cursor = list.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.MailingListCursor{lists, cursor}, nil
}

// Subscriptions is the resolver for the subscriptions field.
func (r *queryResolver) Subscriptions(ctx context.Context, cursor *coremodel.Cursor) (*model.ActivitySubscriptionCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var subs []model.ActivitySubscription
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		sub := (&model.MailingListSubscription{}).As(`sub`)
		query := database.
			Select(ctx, sub).
			From(`subscription sub`).
			Where(`sub.user_id = ?`, auth.ForContext(ctx).UserID)
		subs, cursor = sub.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.ActivitySubscriptionCursor{subs, cursor}, nil
}

// UserWebhooks is the resolver for the userWebhooks field.
func (r *queryResolver) UserWebhooks(ctx context.Context, cursor *coremodel.Cursor) (*model.WebhookSubscriptionCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	filter, err := corewebhooks.FilterWebhooks(ctx)
	if err != nil {
		return nil, err
	}

	var subs []model.WebhookSubscription
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		sub := (&model.UserWebhookSubscription{}).As(`sub`)
		query := database.
			Select(ctx, sub).
			From(`gql_user_wh_sub sub`).
			Where(filter)
		subs, cursor = sub.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.WebhookSubscriptionCursor{subs, cursor}, nil
}

// UserWebhook is the resolver for the userWebhook field.
func (r *queryResolver) UserWebhook(ctx context.Context, id int) (model.WebhookSubscription, error) {
	var sub model.UserWebhookSubscription

	filter, err := corewebhooks.FilterWebhooks(ctx)
	if err != nil {
		return nil, err
	}

	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		row := database.
			Select(ctx, &sub).
			From(`gql_user_wh_sub`).
			Where(sq.And{sq.Expr(`id = ?`, id), filter}).
			RunWith(tx).
			QueryRowContext(ctx)
		if err := row.Scan(database.Scan(ctx, &sub)...); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("No user webhook by ID %d found for this user", id)
		}
		return nil, err
	}

	return &sub, nil
}

// Webhook is the resolver for the webhook field.
func (r *queryResolver) Webhook(ctx context.Context) (model.WebhookPayload, error) {
	raw, err := corewebhooks.Payload(ctx)
	if err != nil {
		return nil, err
	}
	payload, ok := raw.(model.WebhookPayload)
	if !ok {
		panic("Invalid webhook payload context")
	}
	return payload, nil
}

// Sender is the resolver for the sender field.
func (r *threadResolver) Sender(ctx context.Context, obj *model.Thread) (model.Entity, error) {
	if obj.SenderID != nil {
		return loaders.ForContext(ctx).UsersByID.Load(*obj.SenderID)
	}
	list, err := obj.RawHeader.AddressList("From")
	if err != nil {
		return nil, err
	}
	if len(list) != 1 {
		panic(fmt.Errorf("Malformed email %d, multiple senders", obj.ID))
	}
	return &model.Mailbox{
		Name:    list[0].Name,
		Address: list[0].Address,
	}, nil
}

// Root is the resolver for the root field.
func (r *threadResolver) Root(ctx context.Context, obj *model.Thread) (*model.Email, error) {
	return loaders.ForContext(ctx).EmailsByID.Load(obj.ID)
}

// List is the resolver for the list field.
func (r *threadResolver) List(ctx context.Context, obj *model.Thread) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByID.Load(obj.MailingListID)
}

// Descendants is the resolver for the descendants field.
func (r *threadResolver) Descendants(ctx context.Context, obj *model.Thread, cursor *coremodel.Cursor) (*model.EmailCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var emails []*model.Email
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		email := (&model.Email{}).As(`email`)
		query := database.
			Select(ctx, email).
			From(`email`).
			Where(`email.thread_id = ?`, obj.ID).
			OrderBy("email.created")
		emails, cursor = email.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.EmailCursor{emails, cursor}, nil
}

// Mailto is the resolver for the mailto field.
func (r *threadResolver) Mailto(ctx context.Context, obj *model.Thread) (string, error) {
	var (
		header    mail.Header
		ownerName string
		listName  string
	)

	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		var envelope []byte
		row := tx.QueryRowContext(ctx, `
			SELECT envelope, "user".username, list.name
			FROM email
			JOIN list ON list.id = email.list_id
			JOIN "user" ON "user".id = list.owner_id
			WHERE email.thread_id = $1
			ORDER BY email.created DESC
			LIMIT 1;
		`, obj.ID)

		if err := row.Scan(&envelope, &ownerName, &listName); err != nil {
			return err
		}

		reader, err := mail.CreateReader(bytes.NewBuffer(envelope))
		if err != nil {
			panic(err)
		}
		header = reader.Header
		reader.Close()
		return nil
	}); err != nil {
		return "", err
	}

	v := url.Values{}

	if subject, err := header.Subject(); err != nil {
		panic(err)
	} else {
		if !strings.HasPrefix(subject, "Re: ") {
			subject = "Re: " + subject
		}
		v.Set("subject", subject)
	}

	if id, err := header.MessageID(); err != nil {
		panic(err)
	} else {
		v.Set("in-reply-to", id)
	}

	postTo, ok := config.ForContext(ctx).Get("lists.sr.ht", "posting-domain")
	if !ok {
		panic("No posting domain configured")
	}

	url := url.URL{
		Scheme:   "mailto",
		User:     url.User(fmt.Sprintf("~%s/%s", ownerName, listName)),
		Host:     postTo,
		RawQuery: v.Encode(),
	}

	return url.String(), nil
}

// Mbox is the resolver for the mbox field.
func (r *threadResolver) Mbox(ctx context.Context, obj *model.Thread) (*model.URL, error) {
	origin := config.GetOrigin(config.ForContext(ctx), "lists.sr.ht", true)
	uri := fmt.Sprintf("%s/query/thread/%d.mbox", origin, obj.ID)
	url, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}
	return &model.URL{url}, nil
}

// Blocks is the resolver for the blocks field.
func (r *threadResolver) Blocks(ctx context.Context, obj *model.Thread) ([]*model.ThreadBlock, error) {
	var (
		messages []emailthreads.Message
		emails   []model.Email
	)
	err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		query := database.
			SelectAll(new(model.Email)).
			From(`email`).
			Where(`email.id = ? OR email.thread_id = ?`, obj.ID, obj.ID).
			OrderBy(`email.created`)

		rows, err := query.RunWith(tx).QueryContext(ctx)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var email model.Email
			if err := rows.Scan(database.ScanAll(&email)...); err != nil {
				return err
			}
			email.Populate()

			mr, err := mail.CreateReader(bytes.NewReader(email.RawEnvelope))
			if err != nil {
				return fmt.Errorf("failed to create mail reader: %v", err)
			}
			header := mr.Header
			text, err := getMailText(mr)
			if err != nil {
				return fmt.Errorf("failed to get mail text: %v", err)
			}
			mr.Close()

			messages = append(messages, emailthreads.Message{
				Header: header,
				Body:   text,
			})
			emails = append(emails, email)
		}

		return nil
	})
	if err != nil {
		return nil, err
	} else if len(messages) == 0 {
		return nil, nil
	}

	blockTrees, err := emailthreads.Parse(messages)
	if err != nil {
		return nil, fmt.Errorf("failed to parse thread: %v", err)
	}

	sources := make(map[*emailthreads.Message]*model.Email)
	for i := range messages {
		sources[&messages[i]] = &emails[i]
	}

	var blocks []*model.ThreadBlock
	indexes := make(map[*emailthreads.Block]int)
	toThreadBlockList(&blocks, blockTrees, nil, sources, indexes)

	return blocks, nil
}

// Lists is the resolver for the lists field.
func (r *userResolver) Lists(ctx context.Context, obj *model.User, cursor *coremodel.Cursor) (*model.MailingListCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var lists []*model.MailingList
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		list := (&model.MailingList{}).As(`list`)
		user := auth.ForContext(ctx)
		query := database.
			Select(ctx, list).
			From(`list`).
			Where(sq.And{
				sq.Expr(`list.owner_id = ?`, obj.ID),
				sq.Or{
					sq.Expr(`list.owner_id = ?`, user.UserID),
					sq.Expr(`list.visibility != 'PRIVATE'`),
					sq.Expr(`access.permissions > 0`),
				},
			})
		lists, cursor = list.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.MailingListCursor{lists, cursor}, nil
}

// Emails is the resolver for the emails field.
func (r *userResolver) Emails(ctx context.Context, obj *model.User, cursor *coremodel.Cursor) (*model.EmailCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var emails []*model.Email
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		user := auth.ForContext(ctx)
		email := (&model.Email{}).As(`mail`)
		query := database.
			Select(ctx, email).
			From(`email mail`).
			Join(`list ON mail.list_id = list.id`).
			LeftJoin(`access ON
				access.list_id = list.id AND
				access.user_id = ?`, user.UserID).
			Where(sq.And{
				sq.Expr(`mail.sender_id = ?`, obj.ID),
				sq.Or{
					sq.Expr(`list.owner_id = ?`, user.UserID),
					sq.Expr(`access.permissions & ? > 0`, model.ACCESS_BROWSE),
					sq.Expr(`list.default_access & ? > 0`, model.ACCESS_BROWSE),
				},
			})
		emails, cursor = email.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.EmailCursor{emails, cursor}, nil
}

// Threads is the resolver for the threads field.
func (r *userResolver) Threads(ctx context.Context, obj *model.User, cursor *coremodel.Cursor) (*model.ThreadCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var threads []*model.Thread
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		user := auth.ForContext(ctx)
		thread := (&model.Thread{}).As(`mail`)
		query := database.
			Select(ctx, thread).
			From(`email mail`).
			Join(`list ON mail.list_id = list.id`).
			LeftJoin(`access ON
				access.list_id = list.id AND
				access.user_id = ?`, user.UserID).
			Where(sq.And{
				sq.Expr(`mail.sender_id = ?`, obj.ID),
				sq.Expr(`mail.thread_id IS NULL`),
				sq.Or{
					sq.Expr(`list.owner_id = ?`, user.UserID),
					sq.Expr(`access.permissions & ? > 0`, model.ACCESS_BROWSE),
					sq.Expr(`list.default_access & ? > 0`, model.ACCESS_BROWSE),
				},
			})
		threads, cursor = thread.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.ThreadCursor{threads, cursor}, nil
}

// Patches is the resolver for the patches field.
func (r *userResolver) Patches(ctx context.Context, obj *model.User, cursor *coremodel.Cursor) (*model.PatchsetCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var patches []*model.Patchset
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		user := auth.ForContext(ctx)
		patch := (&model.Patchset{}).As(`patch`)
		query := database.
			Select(ctx, patch).
			From(`patchset patch`).
			Join(`list ON patch.list_id = list.id`).
			Join(`email ON email.patchset_id = patch.id AND email.thread_id IS NULL`).
			LeftJoin(`access ON
				access.list_id = list.id AND
				access.user_id = ?`, user.UserID).
			Where(sq.And{
				sq.Expr(`email.sender_id = ?`, obj.ID),
				sq.Or{
					sq.Expr(`list.owner_id = ?`, user.UserID),
					sq.Expr(`access.permissions & ? > 0`, model.ACCESS_BROWSE),
					sq.Expr(`list.default_access & ? > 0`, model.ACCESS_BROWSE),
				},
			})
		patches, cursor = patch.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.PatchsetCursor{patches, cursor}, nil
}

// Client is the resolver for the client field.
func (r *userWebhookSubscriptionResolver) Client(ctx context.Context, obj *model.UserWebhookSubscription) (*model.OAuthClient, error) {
	if obj.ClientID == nil {
		return nil, nil
	}
	return &model.OAuthClient{
		UUID: *obj.ClientID,
	}, nil
}

// Deliveries is the resolver for the deliveries field.
func (r *userWebhookSubscriptionResolver) Deliveries(ctx context.Context, obj *model.UserWebhookSubscription, cursor *coremodel.Cursor) (*model.WebhookDeliveryCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var deliveries []*model.WebhookDelivery
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		d := (&model.WebhookDelivery{}).
			WithName(`user`).
			As(`delivery`)
		query := database.
			Select(ctx, d).
			From(`gql_user_wh_delivery delivery`).
			Where(`delivery.subscription_id = ?`, obj.ID)
		deliveries, cursor = d.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.WebhookDeliveryCursor{deliveries, cursor}, nil
}

// Sample is the resolver for the sample field.
func (r *userWebhookSubscriptionResolver) Sample(ctx context.Context, obj *model.UserWebhookSubscription, event model.WebhookEvent) (string, error) {
	payloadUUID := uuid.New()
	webhook := corewebhooks.WebhookContext{
		User:        auth.ForContext(ctx),
		PayloadUUID: payloadUUID,
		Name:        "user",
		Event:       event.String(),
		Subscription: &corewebhooks.WebhookSubscription{
			ID:         obj.ID,
			URL:        obj.URL,
			Query:      obj.Query,
			AuthMethod: obj.AuthMethod,
			TokenHash:  obj.TokenHash,
			Grants:     obj.Grants,
			ClientID:   obj.ClientID,
			Expires:    obj.Expires,
			NodeID:     obj.NodeID,
		},
	}

	auth := auth.ForContext(ctx)
	switch event {
	case model.WebhookEventListCreated, model.WebhookEventListUpdated,
		model.WebhookEventListDeleted:
		desc := "Sample mailing list for testing webhooks"
		webhook.Payload = &model.MailingListEvent{
			UUID:  payloadUUID.String(),
			Event: event,
			Date:  time.Now().UTC(),
			List: &model.MailingList{
				ID:          -1,
				Created:     time.Now().UTC(),
				Updated:     time.Now().UTC(),
				Name:        "sample-list",
				Description: &desc,
				Visibility:  model.VisibilityPublic,

				OwnerID:        auth.UserID,
				RawPermitMime:  "",
				RawRejectMime:  "",
				Access:         model.ACCESS_ALL,
				DefaultAccess:  model.ACCESS_ALL,
				AccessID:       nil,
				SubscriptionID: nil,
			},
		}
	case model.WebhookEventEmailReceived:
		email := &model.Email{
			ID:        -1,
			Received:  time.Now().UTC(),
			Body:      "Sample email body\r\n",
			Subject:   "Sample email",
			MessageID: "970701.32784@example.com",
			InReplyTo: nil,
			Patch: model.Patch{
				Index:   nil,
				Count:   nil,
				Version: nil,
				Prefix:  nil,
				Subject: nil,
			},
			MailingListID: -1,
			PatchsetID:    nil,
			ThreadID:      nil,
			ParentID:      nil,
			SenderID:      nil,

			RawEnvelope: []byte("Mime-Version: 1.0\r\nContent-Transfer-Encoding: quoted-printable\r\nContent-Type: text/plain; charset=UTF-8\r\nSubject: Sample email\r\nFrom: <someone@example.com>\r\nTo: <sample-list@example.com>\r\nDate: Tue, 14 Jun 2022 09:31:03 +0000\r\nMessage-Id: <970701.32784@example.com>\r\n\r\nSample email body\r\n"),
			RawHeader:   mail.Header{},
		}
		email.Populate()
		webhook.Payload = &model.EmailEvent{
			UUID:  payloadUUID.String(),
			Event: event,
			Date:  time.Now().UTC(),
			Email: email,
		}
	case model.WebhookEventPatchsetReceived:
		webhook.Payload = &model.PatchsetEvent{
			UUID:  payloadUUID.String(),
			Event: event,
			Date:  time.Now().UTC(),
			Patchset: &model.Patchset{
				ID:             -1,
				Created:        time.Now().UTC(),
				Updated:        time.Now().UTC(),
				Subject:        "Sample patchset",
				Prefix:         nil,
				Version:        1,
				MailingListID:  -1,
				CoverLetterID:  nil,
				SupersededByID: nil,
				RawStatus:      "proposed",
			},
		}
	default:
		return "", fmt.Errorf("Unsupported event %s", event.String())
	}

	subctx := corewebhooks.Context(ctx, webhook.Payload)
	bytes, err := webhook.Exec(subctx, server.ForContext(ctx).Schema)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Subscription is the resolver for the subscription field.
func (r *webhookDeliveryResolver) Subscription(ctx context.Context, obj *model.WebhookDelivery) (model.WebhookSubscription, error) {
	if obj.Name == "" {
		panic("WebhookDelivery without name")
	}

	// XXX: This could use a loader but it's unlikely to be a bottleneck
	var sub model.WebhookSubscription
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		// XXX: This needs some work to generalize to other kinds of webhooks
		var subscription interface {
			model.WebhookSubscription
			database.Model
		} = nil
		switch obj.Name {
		case "user":
			subscription = (&model.UserWebhookSubscription{}).As(`sub`)
		case "list":
			subscription = (&model.MailingListWebhookSubscription{}).As(`sub`)
		default:
			panic(fmt.Errorf("unknown webhook name %q", obj.Name))
		}
		// Note: No filter needed because, if we have access to the delivery,
		// we also have access to the subscription.
		row := database.
			Select(ctx, subscription).
			From(`gql_`+obj.Name+`_wh_sub sub`).
			Where(`sub.id = ?`, obj.SubscriptionID).
			RunWith(tx).
			QueryRowContext(ctx)
		if err := row.Scan(database.Scan(ctx, subscription)...); err != nil {
			return err
		}
		sub = subscription
		return nil
	}); err != nil {
		return nil, err
	}
	return sub, nil
}

// Email returns api.EmailResolver implementation.
func (r *Resolver) Email() api.EmailResolver { return &emailResolver{r} }

// MailingList returns api.MailingListResolver implementation.
func (r *Resolver) MailingList() api.MailingListResolver { return &mailingListResolver{r} }

// MailingListACL returns api.MailingListACLResolver implementation.
func (r *Resolver) MailingListACL() api.MailingListACLResolver { return &mailingListACLResolver{r} }

// MailingListSubscription returns api.MailingListSubscriptionResolver implementation.
func (r *Resolver) MailingListSubscription() api.MailingListSubscriptionResolver {
	return &mailingListSubscriptionResolver{r}
}

// MailingListWebhookSubscription returns api.MailingListWebhookSubscriptionResolver implementation.
func (r *Resolver) MailingListWebhookSubscription() api.MailingListWebhookSubscriptionResolver {
	return &mailingListWebhookSubscriptionResolver{r}
}

// Mutation returns api.MutationResolver implementation.
func (r *Resolver) Mutation() api.MutationResolver { return &mutationResolver{r} }

// Patchset returns api.PatchsetResolver implementation.
func (r *Resolver) Patchset() api.PatchsetResolver { return &patchsetResolver{r} }

// PatchsetTool returns api.PatchsetToolResolver implementation.
func (r *Resolver) PatchsetTool() api.PatchsetToolResolver { return &patchsetToolResolver{r} }

// Query returns api.QueryResolver implementation.
func (r *Resolver) Query() api.QueryResolver { return &queryResolver{r} }

// Thread returns api.ThreadResolver implementation.
func (r *Resolver) Thread() api.ThreadResolver { return &threadResolver{r} }

// User returns api.UserResolver implementation.
func (r *Resolver) User() api.UserResolver { return &userResolver{r} }

// UserWebhookSubscription returns api.UserWebhookSubscriptionResolver implementation.
func (r *Resolver) UserWebhookSubscription() api.UserWebhookSubscriptionResolver {
	return &userWebhookSubscriptionResolver{r}
}

// WebhookDelivery returns api.WebhookDeliveryResolver implementation.
func (r *Resolver) WebhookDelivery() api.WebhookDeliveryResolver { return &webhookDeliveryResolver{r} }

type emailResolver struct{ *Resolver }
type mailingListResolver struct{ *Resolver }
type mailingListACLResolver struct{ *Resolver }
type mailingListSubscriptionResolver struct{ *Resolver }
type mailingListWebhookSubscriptionResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }
type patchsetResolver struct{ *Resolver }
type patchsetToolResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type threadResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
type userWebhookSubscriptionResolver struct{ *Resolver }
type webhookDeliveryResolver struct{ *Resolver }
