package webhooks

import (
	"context"
	"encoding/json"
	"time"

	"git.sr.ht/~sircmpwn/core-go/auth"
	sq "github.com/Masterminds/squirrel"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
)

func encodePermissions(bits uint) []string {
	var result []string
	if bits&model.ACCESS_BROWSE == model.ACCESS_BROWSE {
		result = append(result, "browse")
	}
	if bits&model.ACCESS_REPLY == model.ACCESS_REPLY {
		result = append(result, "reply")
	}
	if bits&model.ACCESS_POST == model.ACCESS_POST {
		result = append(result, "post")
	}
	if bits&model.ACCESS_MODERATE == model.ACCESS_MODERATE {
		result = append(result, "moderate")
	}
	return result
}

type ListWebhookPayload struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"created"`
	Description *string   `json:"description"`

	Permissions struct {
		Nonsubscriber []string `json:"nonsubscriber"`
		Subscriber    []string `json:"subscriber"`
		Account       []string `json:"account"`
	} `json:"permissions"`

	Owner struct {
		CanonicalName string `json:"canonical_name"`
		Name          string `json:"name"`
	} `json:"owner"`
}

func DeliverLegacyUserListEvent(
	ctx context.Context,
	list *model.MailingList,
	event string,
) {
	queue := LegacyQueueForContext(ctx)

	payload := ListWebhookPayload{
		ID:          list.ID,
		Name:        list.Name,
		Created:     list.Created,
		Updated:     list.Updated,
		Description: list.Description,
	}
	payload.Permissions.Nonsubscriber = encodePermissions(list.RawNonsubscriber)
	payload.Permissions.Subscriber = encodePermissions(list.RawSubscriber)
	payload.Permissions.Account = encodePermissions(list.RawIdentified)

	// TODO: User groups
	user := auth.ForContext(ctx)
	if user.UserID != list.OwnerID {
		panic("Invariant broken: sending event for non-authed user")
	}
	payload.Owner.CanonicalName = "~" + user.Username
	payload.Owner.Name = user.Username

	encoded, err := json.Marshal(&payload)
	if err != nil {
		panic(err) // Programmer error
	}

	query := sq.
		Select().
		From("user_webhook_subscription sub").
		Where("sub.user_id = ?", user.UserID)
	queue.Schedule(ctx, query, "user", event, encoded)
}

func DeliverLegacyListEvent(
	ctx context.Context,
	list *model.MailingList,
	event string,
) {
	queue := LegacyQueueForContext(ctx)

	payload := ListWebhookPayload{
		ID:          list.ID,
		Name:        list.Name,
		Created:     list.Created,
		Updated:     list.Updated,
		Description: list.Description,
	}
	payload.Permissions.Nonsubscriber = encodePermissions(list.RawNonsubscriber)
	payload.Permissions.Subscriber = encodePermissions(list.RawSubscriber)
	payload.Permissions.Account = encodePermissions(list.RawIdentified)

	// TODO: User groups
	user := auth.ForContext(ctx)
	if user.UserID != list.OwnerID {
		panic("Invariant broken: sending event for non-authed user")
	}
	payload.Owner.CanonicalName = "~" + user.Username
	payload.Owner.Name = user.Username

	encoded, err := json.Marshal(&payload)
	if err != nil {
		panic(err) // Programmer error
	}

	query := sq.
		Select().
		From("list_webhook_subscription sub").
		Where("sub.list_id = ?", list.ID)
	queue.Schedule(ctx, query, "list", event, encoded)
}
