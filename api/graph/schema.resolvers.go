package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	model1 "git.sr.ht/~sircmpwn/core-go/model"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
)

func (r *queryResolver) Version(ctx context.Context) (*model.Version, error) {
	return &model.Version{
		Major:           0,
		Minor:           0,
		Patch:           0,
		DeprecationDate: nil,
	}, nil
}

func (r *queryResolver) Me(ctx context.Context) (*model.User, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) User(ctx context.Context, id int) (*model.User, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) UserByName(ctx context.Context, username string) (*model.User, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) MailingLists(ctx context.Context, cursor *model1.Cursor) (*model.MailingListCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) MailingList(ctx context.Context, id int) (*model.MailingList, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) MailingListByName(ctx context.Context, name string) (*model.MailingList, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) MailingListByOwner(ctx context.Context, ownerName string, listName string) (*model.MailingList, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Subscriptions(ctx context.Context, cursor *model1.Cursor) (*model.SubscriptionCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

// Query returns api.QueryResolver implementation.
func (r *Resolver) Query() api.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }
