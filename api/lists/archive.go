package lists

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	apiErr "git.sr.ht/~sircmpwn/lists.sr.ht/api/errors"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
	"github.com/emersion/go-mbox"
	"github.com/emersion/go-message/mail"
	"github.com/lib/pq"
)

type Archiver struct {
	ctx      context.Context
	tx       *sql.Tx
	listID   int
	isImport bool
}

// Create a new message archiver.
//
// Note: the archiver does not attempt to verify access controls and will
// unconditionally complete the requested operation. The user is expected to
// verify the necessary permissions are available before use.
func NewArchiver(ctx context.Context, tx *sql.Tx, listID int) *Archiver {
	return &Archiver{ctx: ctx, tx: tx, listID: listID, isImport: false}
}

// Import an mbox spool into a mailing list.
//
// Does not enforce access controls.
func (ar *Archiver) ImportSpool(spool io.Reader) error {
	ar.isImport = true
	defer func() { ar.isImport = false }()

	r := mbox.NewReader(spool)
	for {
		select {
		case <-ar.ctx.Done():
			return errors.New("Mailing list spool import timed out")
		default:
		}

		msg, err := r.NextMessage()
		if err == io.EOF {
			break
		} else if err != nil {
			return fmt.Errorf("Error reading mailing list spool: %v", err)
		}

		if err = ar.ArchiveMessage(msg); err != nil {
			if errors.Is(err, apiErr.ErrDuplicateEmail) {
				continue
			}
			// TODO: Collect errors and email them to the user
			log.Printf("Error importing message: %v", err)
		}
	}

	return nil
}

// Import a single email (RFC 2045 MIME message) into a mailing list archive.
//
// Does not enforce access controls.
func (ar *Archiver) ArchiveMessage(r io.Reader) error {
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
			b, _ := io.ReadAll(p.Body)
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
	row := ar.tx.QueryRow(`
		SELECT EXISTS(
			SELECT FROM email WHERE list_id = $1 AND message_id = $2
		)`,
		ar.listID, messageID,
	)
	if err := row.Scan(&exists); err != nil {
		return err
	}
	if exists {
		// Skip this message
		log.Printf("Skipping duplicate message %q", messageID)
		return apiErr.ErrDuplicateEmail
	}

	var emailID int32
	row = ar.tx.QueryRow(`
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
		ar.isImport, date,
		subject, messageID, date,
		envelope.String(), string(headerMap), body,
		ar.listID, nil, nil, nil,
		isPatch, isRequestPull,
		0, 1, inReplyTo,
		nil, nil, nil, nil, nil, nil, nil,
	)
	if err := row.Scan(&emailID); err != nil {
		return err
	}

	// Set parent of this email
	var parentID int
	row = ar.tx.QueryRow(
		`SELECT id FROM email WHERE list_id = $1 AND message_id = $2;`,
		ar.listID, "<"+inReplyTo.String+">",
	)
	if err := row.Scan(&parentID); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	} else {
		if _, err := ar.tx.Exec(
			`UPDATE email SET parent_id = $1 WHERE id = $2`,
			parentID, emailID,
		); err != nil {
			return err
		}
	}

	threadID, err := ar.computeThreadID(emailID)
	if err != nil {
		return err
	}
	if threadID != emailID {
		if _, err := ar.tx.Exec(
			`UPDATE email SET thread_id = $1 WHERE id = $2`,
			threadID, emailID,
		); err != nil {
			return err
		}
	}

	if err := ar.reparentEmails(threadID, emailID, messageID); err != nil {
		return err
	}
	if err := ar.updateThreadReplies(threadID); err != nil {
		return err
	}

	// TODO: Enumerate CC's and create SQL relationships for them
	// TODO: Some users will have many email addresses
	// TODO: Multiple From addresses?
	senders, err := mr.Header.AddressList("From")
	if err != nil {
		return fmt.Errorf("Error reading From: %q %w", mr.Header.Get("From"), err)
	}
	if len(senders) == 0 {
		return errors.New("expected at least one From address")
	}

	// Lookup sender by email
	row = ar.tx.QueryRow(
		`SELECT id FROM "user" WHERE email = $1`,
		senders[0].Address,
	)
	var senderID int
	if err := row.Scan(&senderID); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	} else {
		if _, err := ar.tx.Exec(
			`UPDATE email SET sender_id = $1 WHERE id = $2`,
			senderID, emailID,
		); err != nil {
			return err
		}
	}

	status := string(model.PatchsetStatusProposed)
	if ar.isImport {
		// Only allow forcing patchset status when importing from mbox
		const statusHeader = "X-Sourcehut-Patchset-Final"
		if mr.Header.Has(statusHeader) {
			s := mr.Header.Get(statusHeader)
			if model.PatchsetStatus(strings.ToUpper(s)).IsValid() {
				status = s
			}
		}
		status = strings.ToLower(status)
	}

	if err := ar.importPatch(emailID, threadID, subject, status, isPatch); err != nil {
		return err
	}

	log.Printf("Archived message %q", messageID)

	return nil
}

// Computes the thread ID for the given email
func (ar *Archiver) computeThreadID(emailID int32) (int32, error) {
	// Keep track of seen emails to avoid reference loops
	threadID := emailID
	seen := map[int32]struct{}{}
	for {
		if _, ok := seen[threadID]; ok {
			// Reference loop
			break
		}
		seen[threadID] = struct{}{}
		row := ar.tx.QueryRow(
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
func (ar *Archiver) reparentEmails(threadID, emailID int32, messageID string) error {
	children, err := ar.tx.Query(
		`SELECT id, thread_id FROM email WHERE list_id = $1 AND in_reply_to = $2`,
		ar.listID, messageID,
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
	if _, err := ar.tx.Exec(
		`UPDATE email SET parent_id = $1, thread_id = $2 WHERE id = ANY($3)`,
		emailID, threadID, pq.Int32Array(childIDs),
	); err != nil {
		return err
	}
	if _, err := ar.tx.Exec(
		`UPDATE email SET thread_id = $1 WHERE thread_id = ANY($2)`,
		threadID, pq.Int32Array(oldThreadIDs),
	); err != nil {
		return err
	}
	return nil
}

// Updates thread nreplies and nparticipants
func (ar *Archiver) updateThreadReplies(threadID int32) error {
	nreplies := 0
	memberIDs := []int32{threadID}
	participants := make(map[string]struct{})
	threadMembers, err := ar.tx.Query(
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
	if _, err := ar.tx.Exec(
		`UPDATE email SET nreplies = $1, nparticipants = $2 WHERE id = $3`,
		nreplies, len(participants), threadID,
	); err != nil {
		return err
	}
	return nil
}
