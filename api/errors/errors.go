package errors

import "errors"

// Outcomes the ingress switches on. These were GraphQL errors carrying a code
// extension, matched with core-go/errors.Is over the wire; the ingress calls
// this package directly now, so they are plain sentinels for errors.Is.
var (
	ErrNotSubscribed        = errors.New("not subscribed to this mailing list")
	ErrAlreadySubscribed    = errors.New("already subscribed to this mailing list")
	ErrSubscriptionNotFound = errors.New("subscription not found")
	ErrInvalidToken         = errors.New("invalid subscription token")
	ErrDuplicateEmail       = errors.New("message already archived in this list")
)
