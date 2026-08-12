// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (c) 2024 Robin Jarry

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"strings"

	"git.sr.ht/~sircmpwn/core-go/database"
	apierr "git.sr.ht/~sircmpwn/lists.sr.ht/api/errors"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/lists"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
)

const (
	LISTS_SERVICE = "lists.sr.ht"
)

func parseListAddr(addr string) (owner, name string, cmd Command, err error) {
	cmd = CMD_POST

	// Note: we assume postfix took care of the domain
	listName, _, _ := strings.Cut(addr, "@")
	if i := strings.LastIndex(listName, "+"); i > 0 {
		cmd = Command(listName[i+1:])
		listName = listName[:i]
	}
	if redirect, ok := Config.Redirects[listName]; ok {
		listName = redirect
	}

	// split the list name into owner / listname
	if strings.HasPrefix(listName, "~") {
		var found bool
		owner, name, found = strings.Cut(listName, "/")
		if !found {
			err = &UnknownListError{addr}
			return
		}
		owner = strings.TrimPrefix(owner, "~")
	} else {
		// some mail providers do not allow "~" and "/" in addresses
		tokens := strings.Split(listName, ".")
		if len(tokens) < 3 || tokens[0] != "u" {
			err = &UnknownListError{addr}
			return
		}
		owner = tokens[1]
		name = strings.Join(tokens[2:], ".")
	}

	return
}

// Generate a random confirmation token encoded in base64.
func newConfirmationToken() string {
	buf := make([]byte, 18)
	rand.Read(buf)
	return base64.URLEncoding.EncodeToString(buf)
}

func LookupEmailDetails(
	ctx context.Context, msg *message.Entity, listAddr string,
) (*Sender, *MailingList, error) {
	fromAddr := msg.Header.Get("From")
	inReplyTo := msg.Header.Get("In-Reply-To")

	from, err := mail.ParseAddress(fromAddr)
	if err != nil {
		return nil, nil, err
	}
	owner, name, cmd, err := parseListAddr(listAddr)
	if err != nil {
		return nil, nil, err
	}

	var (
		list    model.MailingList
		acl     *model.GeneralACL
		isReply bool
	)

	// Unlike a client of the API, the ingress has no user to act on behalf of.
	// It looks the list up unconditionally and resolves the sender's access
	// itself; the queries this replaces ran as the list owner and were
	// likewise unfiltered.
	err = database.WithReadOnlyTx(ctx, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT l.id, l.permit_mimetypes, l.reject_mimetypes
			FROM list l
			JOIN "user" u ON u.id = l.owner_id
			WHERE u.username = $1 AND l.name = $2;
		`, owner, name)
		switch err := row.Scan(
			&list.ID, &list.RawPermitMime, &list.RawRejectMime,
		); err {
		case nil:
		case sql.ErrNoRows:
			return &UnknownListError{listAddr}
		default:
			return err
		}

		var err error
		acl, err = model.UserACL(ctx, tx, list.ID, from.Address)
		if err != nil {
			return err
		}

		if inReplyTo == "" {
			return nil
		}
		// Message-ID is stored with the angle brackets it has in the header.
		row = tx.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM email
				WHERE list_id = $1 AND message_id = $2
			);
		`, list.ID, inReplyTo)
		return row.Scan(&isReply)
	})
	if err != nil {
		return nil, nil, err
	}

	sender := &Sender{
		Name:  from.Name,
		Email: from.Address,
		ACL:   acl,
	}

	mailingList := &MailingList{
		Owner:           owner,
		Name:            name,
		Command:         cmd,
		ID:              list.ID,
		PermitMimetypes: list.PermitMime(),
		RejectMimetypes: list.RejectMime(),
		IsReply:         isReply,
	}

	switch cmd {
	case CMD_SUBSCRIBE, CMD_UNSUBSCRIBE, CMD_CONFIRM_SUB, CMD_CONFIRM_UNSUB, CMD_POST:
		return sender, mailingList, nil
	default:
		return nil, nil, &UnknownCommandError{mailingList}
	}
}

func RequestSubscription(
	ctx context.Context, sender *Sender, list *MailingList,
) (string, error) {
	var confirmToken string

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT count(*)
			FROM subscription s
			LEFT OUTER JOIN "user" u ON u.id = s.user_id
			WHERE s.list_id = $1
			AND (s.email = $2 OR u.email = $2);
		`, list.ID, sender.Email)

		var count int
		if err := row.Scan(&count); err != nil {
			return err
		} else if count != 0 {
			return apierr.ErrAlreadySubscribed
		}

		confirmToken = newConfirmationToken()

		// Must use 'ON CONFLICT DO UPDATE' resetting the same email
		// address, otherwise the query returns nothing and there is no
		// way to get the previous confirmation hash.
		row = tx.QueryRowContext(ctx, `
			INSERT INTO subscription_request (
				list_id, email, confirmation_hash
			) VALUES ($1, $2, $3)
			ON CONFLICT ON CONSTRAINT sr_list_id_email_unique
			DO UPDATE SET email = $2
			RETURNING confirmation_hash;
		`, list.ID, sender.Email, confirmToken)

		// If a subscription request already exists. The token present
		// in the database will be returned.
		return row.Scan(&confirmToken)
	}); err != nil {
		return "", err
	}

	return confirmToken, nil
}

func ConfirmSubscription(ctx context.Context, sender *Sender, token string) error {
	return database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			DELETE FROM subscription_request
			WHERE email = $1 AND confirmation_hash = $2
			RETURNING list_id;
		`, sender.Email, token)

		var listID int
		switch err := row.Scan(&listID); err {
		case nil:
		case sql.ErrNoRows:
			return apierr.ErrInvalidToken
		default:
			return err
		}

		var (
			optEmail *string
			optUser  *int
			userID   int
		)
		row = tx.QueryRowContext(
			ctx, `SELECT id FROM "user" WHERE email = $1;`, sender.Email,
		)
		switch err := row.Scan(&userID); err {
		case nil:
			optUser = &userID
		case sql.ErrNoRows:
			optEmail = &sender.Email
		default:
			return err
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO subscription (
				created, updated, email, user_id, list_id
			) VALUES (
				NOW() at time zone 'utc',
				NOW() at time zone 'utc',
				$1, $2, $3
			);
		`, optEmail, optUser, listID)
		return err
	})
}

func RequestUnsubscription(
	ctx context.Context, sender *Sender, list *MailingList,
) (string, error) {
	var confirmToken string

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT count(*)
			FROM subscription s
			LEFT OUTER JOIN "user" u ON u.id = s.user_id
			WHERE s.list_id = $1
			AND (s.email = $2 OR u.email = $2);
		`, list.ID, sender.Email)

		var count int
		if err := row.Scan(&count); err != nil {
			return err
		} else if count == 0 {
			return apierr.ErrNotSubscribed
		}

		row = tx.QueryRowContext(ctx, `
			SELECT confirmation_hash
			FROM subscription_request
			WHERE list_id = $1 AND email = $2;
		`, list.ID, sender.Email)
		err := row.Scan(&confirmToken)
		if err == nil {
			// Unsubscription request already exists. Return the token again.
			return nil
		} else if err != sql.ErrNoRows {
			return err
		}

		confirmToken = newConfirmationToken()

		_, err = tx.ExecContext(ctx, `
			INSERT INTO subscription_request (
				list_id, email, confirmation_hash
			) VALUES ($1, $2, $3);
		`, list.ID, sender.Email, confirmToken)
		return err
	}); err != nil {
		return "", err
	}

	return confirmToken, nil
}

func ConfirmUnsubscription(ctx context.Context, sender *Sender, token string) error {
	return database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			DELETE FROM subscription_request
			WHERE email = $1 AND confirmation_hash = $2
			RETURNING list_id;
		`, sender.Email, token)

		var listID int
		switch err := row.Scan(&listID); err {
		case nil:
		case sql.ErrNoRows:
			return apierr.ErrInvalidToken
		default:
			return err
		}

		row = tx.QueryRowContext(ctx, `
			DELETE FROM subscription s
			WHERE s.list_id = $1 AND (
				s.email = $2 OR s.user_id IN (
					SELECT u.id FROM "user" u
					WHERE u.email = $2
				)
			)
			RETURNING s.id;
		`, listID, sender.Email)

		var subID int
		switch err := row.Scan(&subID); err {
		case nil:
			return nil
		case sql.ErrNoRows:
			return apierr.ErrSubscriptionNotFound
		default:
			return err
		}
	})
}

// Archive a message into a mailing list.
//
// This does not deliver the emailReceived and patchsetReceived webhooks that
// the API's archiveMessage mutation triggers: their delivery evaluates each
// subscriber's GraphQL query against the executable schema, which is only
// available in the API server. See mutation { triggerListEmailWebhooks }.
func ArchiveMessage(ctx context.Context, data []byte, list *MailingList) error {
	// The archiver opens its own transaction, it takes none here.
	_, err := lists.NewArchiver(ctx, nil, list.ID).
		ArchiveMessage(bytes.NewReader(data))
	return err
}

func LookupSubscribers(ctx context.Context, list *MailingList) ([]string, error) {
	var emails []string

	if err := database.WithReadOnlyTx(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx, `
			SELECT COALESCE(s.email, u.email)
			FROM subscription s
			LEFT OUTER JOIN "user" u ON u.id = s.user_id
			WHERE s.list_id = $1;
		`, list.ID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var email string
			if err := rows.Scan(&email); err != nil {
				return err
			}
			emails = append(emails, email)
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}

	return emails, nil
}

func CopySelf(ctx context.Context, address string) bool {
	var copySelf bool

	if err := database.WithReadOnlyTx(ctx, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT copy_self FROM "user" WHERE email = $1;
		`, address)
		if err := row.Scan(&copySelf); err != nil && err != sql.ErrNoRows {
			return err
		}
		return nil
	}); err != nil {
		return false
	}

	return copySelf
}
