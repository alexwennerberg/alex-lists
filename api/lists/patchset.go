package lists

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/lists.sr.ht/api/webhooks"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

func identifyPatch(body string) bool {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	files, _, err := gitdiff.Parse(strings.NewReader(body))
	if err != nil {
		return false
	}
	return len(files) > 0
}

var (
	patchSubjectRE = regexp.MustCompile(
		`^.*\[(?:RFC )?PATCH(?: (?P<prefix>[^\]]+))?\] (?P<subject>.*)$`,
	)
	patchVersionRE = regexp.MustCompile(
		`(?:v(?P<version>[0-9]+))?(?: ?(?P<index>[0-9]+)/(?P<count>[0-9]+))?$`,
	)
)

type PatchDetails struct {
	Prefix  string
	Subject string
	Version int
	Index   int
	Count   int
}

func parsePatchSubject(subject string) (*PatchDetails, error) {
	var patch PatchDetails

	submatch := patchSubjectRE.FindStringSubmatch(subject)
	if submatch == nil {
		// TODO: figure out a better way of dealing with patches that have weird
		// subjects
		return nil, nil
	}

	patch.Prefix = submatch[patchSubjectRE.SubexpIndex("prefix")]
	patch.Subject = submatch[patchSubjectRE.SubexpIndex("subject")]
	patch.Version = 1
	patch.Index = 1
	patch.Count = 1

	submatch = patchVersionRE.FindStringSubmatch(patch.Prefix)
	if submatch != nil {
		patch.Prefix = strings.TrimSuffix(patch.Prefix, submatch[0])

		var err error
		versionMatch := submatch[patchVersionRE.SubexpIndex("version")]
		if len(versionMatch) != 0 {
			patch.Version, err = strconv.Atoi(versionMatch)
			if err != nil {
				return nil, err
			}
		}
		indexMatch := submatch[patchVersionRE.SubexpIndex("index")]
		if len(indexMatch) != 0 {
			patch.Index, err = strconv.Atoi(indexMatch)
			if err != nil {
				return nil, err
			}
		}
		countMatch := submatch[patchVersionRE.SubexpIndex("count")]
		if len(countMatch) != 0 {
			patch.Count, err = strconv.Atoi(countMatch)
			if err != nil {
				return nil, err
			}
		}
	}

	return &patch, nil
}

func (ar *Archiver) importPatch(emailID, threadID int32, subject, status string, isPatch bool) error {
	patch, err := parsePatchSubject(subject)
	if err != nil {
		return fmt.Errorf("Error parsing patch subject: %v", err)
	} else if patch == nil {
		return nil
	}

	// Consider cover letters (index = 0) as valid patches. This makes it much
	// easier to look up the cover letter later on by querying patch_index = 0.
	// This also handles the case where the cover letter is received last.
	if !isPatch && patch.Index != 0 {
		return nil
	}

	if _, err := ar.tx.Exec(
		`UPDATE email SET
			patch_index = $1, patch_count = $2, patch_version = $3,
			patch_prefix = $4, patch_subject = $5
		WHERE id = $6`,
		patch.Index, patch.Count, patch.Version, patch.Prefix, patch.Subject,
		emailID,
	); err != nil {
		return err
	}

	log.Printf("Received patch %d/%d: %q", patch.Index, patch.Count, patch.Subject)

	// Check if the patchset is complete
	complete := true
	for i := 1; i <= patch.Count; i++ {
		var exists bool
		row := ar.tx.QueryRow(
			`SELECT EXISTS (
				SELECT FROM email
				WHERE (id = $1 OR thread_id = $1) AND patch_index = $2
			)`,
			threadID, i,
		)
		if err := row.Scan(&exists); err != nil {
			return err
		}
		if !exists {
			complete = false
			break
		}
	}
	if !complete {
		return nil
	}
	log.Println("Complete patchset received")

	// Look for existing patchset
	var existing bool
	row := ar.tx.QueryRow(
		`SELECT EXISTS (
			SELECT FROM email
			WHERE (id = $1 OR thread_id = $1) AND patchset_id IS NOT NULL
		)`,
		threadID,
	)
	if err := row.Scan(&existing); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	} else if existing {
		// TODO: is this a new revision? complicated
		return nil
	}

	// Look for a cover letter
	var coverLetterID *int
	var patchsetSubject string
	var patchsetPrefix string
	var patchsetVersion int
	row = ar.tx.QueryRow(
		`SELECT
			id, patch_subject, patch_prefix, patch_version
		FROM email WHERE (id = $1 OR thread_id = $1) AND patch_index = 0`,
		threadID,
	)
	if err := row.Scan(&coverLetterID,
		&patchsetSubject, &patchsetPrefix, &patchsetVersion); err != nil {
		if err != sql.ErrNoRows {
			return err
		}
	}

	if coverLetterID == nil {
		// TODO: handle the case where patch subjects/prefixes/versions/senders
		// don't match?
		patchsetSubject = patch.Subject
		patchsetPrefix = patch.Prefix
		patchsetVersion = patch.Version
	}

	// Get the info need to reply to the last message in the patchset.
	// This info is exposed via the lists.sr.ht REST API and is used by
	// hub.sr.ht to automatically reply to patches.
	var messageID string
	var submitter sql.NullString
	var replyTo sql.NullString
	row = ar.tx.QueryRow(
		`SELECT
			message_id, headers -> 'From' -> 0, headers -> 'Reply-To' -> 0
		FROM email WHERE (id = $1 OR thread_id = $1) AND patch_index = $2`,
		threadID, patch.Count,
	)
	if err := row.Scan(&messageID, &submitter, &replyTo); err != nil {
		return err
	}

	var patchsetID int
	var created, updated time.Time
	row = ar.tx.QueryRow(`
		INSERT INTO patchset (
			created, updated,
			subject, prefix, version, list_id, cover_letter_id,
			message_id, submitter, reply_to, status
		) VALUES (
			NOW() at time zone 'utc',
			NOW() at time zone 'utc',
			$1, $2, $3, $4, $5, $6, $7, $8, $9
		) RETURNING id, created, updated`,
		patchsetSubject, patchsetPrefix, patchsetVersion, ar.listID, coverLetterID,
		messageID, submitter, replyTo, status,
	)
	if err := row.Scan(&patchsetID, &created, &updated); err != nil {
		return err
	}

	if _, err := ar.tx.Exec(
		`UPDATE email SET patchset_id = $1 WHERE id = $2 OR thread_id = $2`,
		patchsetID, threadID,
	); err != nil {
		return err
	}

	if !ar.isImport {
		webhooks.DeliverListPatchsetEvent(
			ar.ctx, ar.listID, model.WebhookEventPatchsetReceived,
			&model.Patchset{
				ID:            patchsetID,
				Created:       created,
				Updated:       updated,
				Subject:       subject,
				Prefix:        &patchsetPrefix,
				Version:       patchsetVersion,
				MailingListID: ar.listID,
				CoverLetterID: coverLetterID,
				RawStatus:     status,
			},
		)
	}

	// TODO: identify patchset that this supersedes, if appropriate

	return nil
}
