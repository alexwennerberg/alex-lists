package lists

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"

	"git.sr.ht/~sircmpwn/core-go/database"
	"github.com/emersion/go-mbox"
	"github.com/emersion/go-message/mail"
	"github.com/lib/pq"
)

func importMailingListSpool(ctx context.Context, listID int, spool io.Reader) error {
	r := mbox.NewReader(spool)
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		for {
			select {
			case <-ctx.Done():
				return errors.New("Mailing list spool import timed out")
			default:
			}

			msg, err := r.NextMessage()
			if err == io.EOF {
				break
			} else if err != nil {
				return fmt.Errorf("Error reading mailing list spool: %v", err)
			}

			if err := archiveMessage(tx, listID, msg, true); err != nil {
				// TODO: Collect errors and email them to the user
				log.Printf("Error importing message: %v", err)
				continue
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func archiveMessage(tx *sql.Tx, listID int, r io.Reader, isImported bool) error {
	var envelope bytes.Buffer

	mr, err := mail.CreateReader(io.TeeReader(r, &envelope))
	if err != nil {
		return err
	}
	subject, err := mr.Header.Subject()
	if err != nil {
		return fmt.Errorf("Error reading Subject: %w", err)
	}
	messageID, err := mr.Header.MessageID()
	if err != nil {
		return fmt.Errorf("Error reading Message-ID: %w", err)
	}
	// TODO: Store Message-ID without "<>" in database
	messageID = "<" + messageID + ">"
	date, err := mr.Header.Date()
	if err != nil {
		return fmt.Errorf("Error reading Date: %w", err)
	}
	inReplyToList, err := mr.Header.MsgIDList("In-Reply-To")
	if err != nil {
		return fmt.Errorf("Error reading In-Reply-To: %w", err)
	}
	var inReplyTo sql.NullString
	if len(inReplyToList) > 0 {
		// TODO: multiple In-Reply-To message IDs?
		inReplyTo.String = inReplyToList[0]
		inReplyTo.Valid = true
	}

	var body string

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("Error reading message part: %w", err)
		}

		switch p.Header.(type) {
		case *mail.InlineHeader:
			b, _ := ioutil.ReadAll(p.Body)
			body = string(b)
			// TODO: multiple text parts?
			break
		case *mail.AttachmentHeader:
			// Do nothing
		}
	}

	isPatch := identifyPatch(body)
	// TODO: Identify request-pull
	isRequestPull := false

	headerMap, err := json.Marshal(mr.Header.Map())
	if err != nil {
		return err
	}

	var exists bool
	row := tx.QueryRow(`
		SELECT EXISTS(
			SELECT FROM email WHERE list_id = $1 AND message_id = $2
		)`,
		listID, messageID,
	)
	if err := row.Scan(&exists); err != nil {
		return err
	}
	if exists {
		// Skip this message
		return fmt.Errorf("Skipping duplicate message %q", messageID)
	}

	var emailID int32
	row = tx.QueryRow(`
		INSERT INTO email (
			created, updated, subject, message_id, message_date,
			envelope, headers, body,
			list_id, parent_id, thread_id, sender_id,
			is_patch, is_request_pull,
			nreplies,
			nparticipants,
			in_reply_to,
			patchset_id,
			patch_index,
			patch_count,
			patch_version,
			patch_prefix,
			patch_subject,
			superseded_by_id
		) VALUES (
			CASE WHEN $1 THEN $2 ELSE NOW() at time zone 'utc' END,
			CASE WHEN $1 THEN $2 ELSE NOW() at time zone 'utc' END,
			$3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24
		) RETURNING id`,
		isImported, date,
		subject, messageID, date,
		envelope.String(), string(headerMap), body,
		listID, nil, nil, nil,
		isPatch, isRequestPull,
		0, 1, inReplyTo,
		nil, nil, nil, nil, nil, nil, nil,
	)
	if err := row.Scan(&emailID); err != nil {
		return err
	}

	// Set parent of this email
	var parentID int
	row = tx.QueryRow(
		`SELECT id FROM email WHERE list_id = $1 AND message_id = $2;`,
		listID, "<"+inReplyTo.String+">",
	)
	if err := row.Scan(&parentID); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	} else {
		if _, err := tx.Exec(
			`UPDATE email SET parent_id = $1 WHERE id = $2`,
			parentID, emailID,
		); err != nil {
			return err
		}
	}

	threadID, err := computeThreadID(tx, emailID)
	if err != nil {
		return err
	}
	if threadID != emailID {
		if _, err := tx.Exec(
			`UPDATE email SET thread_id = $1 WHERE id = $2`,
			threadID, emailID,
		); err != nil {
			return err
		}
	}

	if err := reparentEmails(tx, listID, threadID, emailID, messageID); err != nil {
		return err
	}
	if err := updateThreadReplies(tx, threadID); err != nil {
		return err
	}

	// TODO: Enumerate CC's and create SQL relationships for them
	// TODO: Some users will have many email addresses
	// TODO: Mutliple From addresses?
	senders, err := mr.Header.AddressList("From")
	if err != nil {
		return fmt.Errorf("Error reading From: %q %w", mr.Header.Get("From"), err)
	}
	if len(senders) == 0 {
		return errors.New("expected at least one From address")
	}
	// Lookup sender by email
	row = tx.QueryRow(
		`SELECT id FROM "user" WHERE email = $1`,
		senders[0].Address,
	)
	var senderID int
	if err := row.Scan(&senderID); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	} else {
		if _, err := tx.Exec(
			`UPDATE email SET sender_id = $1 WHERE id = $2`,
			senderID, emailID,
		); err != nil {
			return err
		}
	}

	if err := importPatch(tx, listID, emailID, threadID, subject, isPatch); err != nil {
		return err
	}

	// Update patchset status
	const updateHeader = "X-Sourcehut-Patchset-Update"
	if !isPatch && mr.Header.Has(updateHeader) {
		status := strings.ToLower(mr.Header.Get(updateHeader))
		if err := updatePatchsetStatus(tx, threadID, senderID, senders[0].Address, status); err != nil {
			return err
		}
	}

	log.Printf("Archived message %q", messageID)

	return nil
}

// Computes the thread ID for the given email
func computeThreadID(tx *sql.Tx, emailID int32) (int32, error) {
	// Keep track of seen emails to avoid reference loops
	threadID := emailID
	seen := map[int32]struct{}{}
	for {
		if _, ok := seen[threadID]; ok {
			// Reference loop
			break
		}
		seen[threadID] = struct{}{}
		row := tx.QueryRow(
			`SELECT parent_id FROM email WHERE id = $1`,
			threadID,
		)
		var nextID *int32
		if err := row.Scan(&nextID); err != nil {
			return 0, err
		}
		if nextID == nil {
			break
		}
		threadID = *nextID
	}
	return threadID, nil
}

// Reparent emails that arrived out-of-order
func reparentEmails(tx *sql.Tx, listID int, threadID, emailID int32, messageID string) error {
	children, err := tx.Query(
		`SELECT id, thread_id FROM email WHERE list_id = $1 AND in_reply_to = $2`,
		listID, messageID,
	)
	if err != nil {
		return err
	}
	defer children.Close()
	var childIDs []int32
	var oldThreadIDs []int32
	for children.Next() {
		var childID int32
		var childThreadID *int32
		if err := children.Scan(&childID, &childThreadID); err != nil {
			return err
		}
		childIDs = append(childIDs, childID)
		if childThreadID == nil {
			oldThreadIDs = append(oldThreadIDs, childID)
		} else if *childThreadID != threadID {
			oldThreadIDs = append(oldThreadIDs, *childThreadID)
		}
	}
	if _, err := tx.Exec(
		`UPDATE email SET parent_id = $1, thread_id = $2 WHERE id = ANY($3)`,
		emailID, threadID, pq.Int32Array(childIDs),
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE email SET thread_id = $1 WHERE thread_id = ANY($2)`,
		threadID, pq.Int32Array(oldThreadIDs),
	); err != nil {
		return err
	}
	return nil
}

// Updates thread nreplies and nparticipants
func updateThreadReplies(tx *sql.Tx, threadID int32) error {
	nreplies := 0
	memberIDs := []int32{threadID}
	participants := make(map[string]struct{})
	threadMembers, err := tx.Query(
		`SELECT id, headers -> 'From' -> 0 FROM email WHERE thread_id = $1`,
		threadID,
	)
	if err != nil {
		return err
	}
	defer threadMembers.Close()
	for threadMembers.Next() {
		var memberID int32
		var fromHeader string
		if err := threadMembers.Scan(&memberID, &fromHeader); err != nil {
			return err
		}
		memberIDs = append(memberIDs, memberID)
		// TODO: multiple From addresses?
		participants[fromHeader] = struct{}{}
		nreplies++
	}
	if _, err := tx.Exec(
		`UPDATE email SET nreplies = $1, nparticipants = $2 WHERE id = $3`,
		nreplies, len(participants), threadID,
	); err != nil {
		return err
	}
	return nil
}
