package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"git.sr.ht/~sircmpwn/core-go/auth"
	coremodel "git.sr.ht/~sircmpwn/core-go/model"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/loaders"
)

func (r *mailingListResolver) Owner(ctx context.Context, obj *model.MailingList) (model.Entity, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListResolver) Threads(ctx context.Context, obj *model.MailingList, cursor *coremodel.Cursor) (*model.ThreadCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListResolver) Emails(ctx context.Context, obj *model.MailingList, cursor *coremodel.Cursor) (*model.EmailCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListResolver) Patches(ctx context.Context, obj *model.MailingList, cursor *coremodel.Cursor) (*model.PatchsetCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListResolver) Access(ctx context.Context, obj *model.MailingList) (model.ACL, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListResolver) Subscription(ctx context.Context, obj *model.MailingList) (model.Subscription, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListACLResolver) List(ctx context.Context, obj *model.MailingListACL) (*model.MailingList, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mailingListACLResolver) Entity(ctx context.Context, obj *model.MailingListACL) (model.Entity, error) {
	panic(fmt.Errorf("not implemented"))
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

func (r *queryResolver) MailingLists(ctx context.Context, cursor *coremodel.Cursor) (*model.MailingListCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) MailingList(ctx context.Context, id int) (*model.MailingList, error) {
	return loaders.ForContext(ctx).MailingListsByID.Load(id)
}

func (r *queryResolver) MailingListByName(ctx context.Context, name string) (*model.MailingList, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) MailingListByOwner(ctx context.Context, ownerName string, listName string) (*model.MailingList, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Subscriptions(ctx context.Context, cursor *coremodel.Cursor) (*model.SubscriptionCursor, error) {
	panic(fmt.Errorf("not implemented"))
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

// MailingList returns api.MailingListResolver implementation.
func (r *Resolver) MailingList() api.MailingListResolver { return &mailingListResolver{r} }

// MailingListACL returns api.MailingListACLResolver implementation.
func (r *Resolver) MailingListACL() api.MailingListACLResolver { return &mailingListACLResolver{r} }

// Query returns api.QueryResolver implementation.
func (r *Resolver) Query() api.QueryResolver { return &queryResolver{r} }

// User returns api.UserResolver implementation.
func (r *Resolver) User() api.UserResolver { return &userResolver{r} }

type mailingListResolver struct{ *Resolver }
type mailingListACLResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
