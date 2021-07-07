package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"git.sr.ht/~sircmpwn/core-go/auth"
	"git.sr.ht/~sircmpwn/core-go/config"
	"git.sr.ht/~sircmpwn/core-go/database"
	coremodel "git.sr.ht/~sircmpwn/core-go/model"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/loaders"
)

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

func (r *emailResolver) Date(ctx context.Context, obj *model.Email) (*time.Time, error) {
	date, err := obj.RawHeader.Date()
	if err != nil {
		return nil, nil
	}
	return &date, nil
}

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

func (r *emailResolver) Envelope(ctx context.Context, obj *model.Email) (*model.URL, error) {
	origin := config.GetOrigin(config.ForContext(ctx), "lists.sr.ht", true)
	uri := fmt.Sprintf("%s/query/email/%d", origin, obj.ID)
	url, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}
	return &model.URL{url}, nil
}

func (r *emailResolver) Thread(ctx context.Context, obj *model.Email) (*model.Thread, error) {
	// Regarding the use of an unsafe loader: if you have access to the email
	// object, you have access to the thread also.
	if obj.ThreadID == nil {
		return loaders.ForContext(ctx).ThreadsByIDUnsafe.Load(obj.ID)
	}
	return loaders.ForContext(ctx).ThreadsByIDUnsafe.Load(*obj.ThreadID)
}

func (r *emailResolver) Parent(ctx context.Context, obj *model.Email) (*model.Email, error) {
	if obj.ParentID == nil {
		return nil, nil
	}
	// Regarding the use of an unsafe loader: if you have access to the email
	// object, you have access to its parent also.
	return loaders.ForContext(ctx).EmailsByIDUnsafe.Load(*obj.ParentID)
}

func (r *emailResolver) Patchset(ctx context.Context, obj *model.Email) (*model.Patchset, error) {
	if obj.PatchsetID == nil {
		return nil, nil
	}
	// Regarding the use of an unsafe loader: if you have access to the email
	// object, you have access to the patchset also.
	return loaders.ForContext(ctx).PatchsetsByIDUnsafe.Load(*obj.PatchsetID)
}

func (r *emailResolver) List(ctx context.Context, obj *model.Email) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByID.Load(obj.MailingListID)
}

func (r *mailingListResolver) Owner(ctx context.Context, obj *model.MailingList) (model.Entity, error) {
	return loaders.ForContext(ctx).UsersByID.Load(obj.OwnerID)
}

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

func (r *mailingListResolver) Access(ctx context.Context, obj *model.MailingList) (model.ACL, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListResolver) Subscription(ctx context.Context, obj *model.MailingList) (model.Subscription, error) {
	if obj.SubscriptionID == nil {
		return nil, nil
	}
	return loaders.ForContext(ctx).SubscriptionsByIDUnsafe.Load(*obj.SubscriptionID)
}

func (r *mailingListResolver) Archive(ctx context.Context, obj *model.MailingList) (*model.URL, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListResolver) Last30days(ctx context.Context, obj *model.MailingList) (*model.URL, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListACLResolver) List(ctx context.Context, obj *model.MailingListACL) (*model.MailingList, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListACLResolver) Entity(ctx context.Context, obj *model.MailingListACL) (model.Entity, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListSubscriptionResolver) List(ctx context.Context, obj *model.MailingListSubscription) (*model.MailingList, error) {
	// XXX: This can be unsafe if we ever write that loader
	return loaders.ForContext(ctx).MailingListsByID.Load(obj.ListID)
}

func (r *patchsetResolver) Submitter(ctx context.Context, obj *model.Patchset) (model.Entity, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *patchsetResolver) CoverLetter(ctx context.Context, obj *model.Patchset) (*model.Email, error) {
	if obj.CoverLetterID == nil {
		return nil, nil
	}
	return loaders.ForContext(ctx).EmailsByID.Load(*obj.CoverLetterID)
}

func (r *patchsetResolver) Thread(ctx context.Context, obj *model.Patchset) (*model.Thread, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *patchsetResolver) SupersededBy(ctx context.Context, obj *model.Patchset) (*model.Patchset, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *patchsetResolver) List(ctx context.Context, obj *model.Patchset) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByID.Load(obj.MailingListID)
}

func (r *patchsetResolver) Patches(ctx context.Context, obj *model.Patchset, cursor *coremodel.Cursor) (*model.EmailCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *patchsetResolver) Tools(ctx context.Context, obj *model.Patchset) ([]*model.PatchsetTool, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *patchsetResolver) Mbox(ctx context.Context, obj *model.Patchset) (*model.URL, error) {
	origin := config.GetOrigin(config.ForContext(ctx), "lists.sr.ht", true)
	uri := fmt.Sprintf("%s/query/patchset/%d.mbox", origin, obj.ID)
	url, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}
	return &model.URL{url}, nil
}

func (r *queryResolver) Version(ctx context.Context) (*model.Version, error) {
	return &model.Version{
		Major:           0,
		Minor:           0,
		Patch:           0,
		DeprecationDate: nil,
	}, nil
}

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

func (r *queryResolver) User(ctx context.Context, id int) (*model.User, error) {
	return loaders.ForContext(ctx).UsersByID.Load(id)
}

func (r *queryResolver) UserByName(ctx context.Context, username string) (*model.User, error) {
	return loaders.ForContext(ctx).UsersByName.Load(username)
}

func (r *queryResolver) MailingList(ctx context.Context, id int) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByID.Load(id)
}

func (r *queryResolver) MailingListByName(ctx context.Context, name string) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByName.Load(name)
}

func (r *queryResolver) MailingListByOwner(ctx context.Context, ownerName string, listName string) (*model.MailingList, error) {
	if strings.HasPrefix(ownerName, "~") {
		ownerName = ownerName[1:]
	} else {
		return nil, fmt.Errorf("Expected owner to be a canonical name")
	}
	return loaders.ForContext(ctx).MailingListsByOwnerName.Load([2]string{ownerName, listName})
}

func (r *queryResolver) Email(ctx context.Context, id int) (*model.Email, error) {
	return loaders.ForContext(ctx).EmailsByID.Load(id)
}

func (r *queryResolver) Message(ctx context.Context, messageID string) (*model.Email, error) {
	return loaders.ForContext(ctx).EmailsByMessageID.Load(messageID)
}

func (r *queryResolver) Patchset(ctx context.Context, id int) (*model.Patchset, error) {
	return loaders.ForContext(ctx).PatchsetsByID.Load(id)
}

func (r *queryResolver) MailingLists(ctx context.Context, cursor *coremodel.Cursor) (*model.MailingListCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var lists []*model.MailingList
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		list := (&model.MailingList{})
		query := database.
			Select(ctx, list).
			From(`list`).
			Where(`list.owner_id = ?`, auth.ForContext(ctx).UserID)
		lists, cursor = list.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.MailingListCursor{lists, cursor}, nil
}

func (r *queryResolver) Subscriptions(ctx context.Context, cursor *coremodel.Cursor) (*model.SubscriptionCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var subs []model.Subscription
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

	return &model.SubscriptionCursor{subs, cursor}, nil
}

func (r *threadResolver) Sender(ctx context.Context, obj *model.Thread) (model.Entity, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *threadResolver) Root(ctx context.Context, obj *model.Thread) (*model.Email, error) {
	return loaders.ForContext(ctx).EmailsByID.Load(obj.ID)
}

func (r *threadResolver) List(ctx context.Context, obj *model.Thread) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByID.Load(obj.MailingListID)
}

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

func (r *threadResolver) Mailto(ctx context.Context, obj *model.Thread) (string, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *threadResolver) Mbox(ctx context.Context, obj *model.Thread) (*model.URL, error) {
	origin := config.GetOrigin(config.ForContext(ctx), "lists.sr.ht", true)
	uri := fmt.Sprintf("%s/query/thread/%d.mbox", origin, obj.ID)
	url, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}
	return &model.URL{url}, nil
}

func (r *userResolver) Lists(ctx context.Context, obj *model.User, cursor *coremodel.Cursor) (*model.MailingListCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *userResolver) Emails(ctx context.Context, obj *model.User, cursor *coremodel.Cursor) (*model.EmailCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *userResolver) Threads(ctx context.Context, obj *model.User, cursor *coremodel.Cursor) (*model.ThreadCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *userResolver) Patches(ctx context.Context, obj *model.User, cursor *coremodel.Cursor) (*model.PatchsetCursor, error) {
	panic(fmt.Errorf("not implemented"))
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

// Patchset returns api.PatchsetResolver implementation.
func (r *Resolver) Patchset() api.PatchsetResolver { return &patchsetResolver{r} }

// Query returns api.QueryResolver implementation.
func (r *Resolver) Query() api.QueryResolver { return &queryResolver{r} }

// Thread returns api.ThreadResolver implementation.
func (r *Resolver) Thread() api.ThreadResolver { return &threadResolver{r} }

// User returns api.UserResolver implementation.
func (r *Resolver) User() api.UserResolver { return &userResolver{r} }

type emailResolver struct{ *Resolver }
type mailingListResolver struct{ *Resolver }
type mailingListACLResolver struct{ *Resolver }
type mailingListSubscriptionResolver struct{ *Resolver }
type patchsetResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type threadResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
