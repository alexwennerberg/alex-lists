package model

import (
	"context"
	"database/sql"
)

// Permission bits, as stored in list.default_access and access.permissions.
const (
	ACCESS_BROWSE   = 1
	ACCESS_REPLY    = 2
	ACCESS_POST     = 4
	ACCESS_MODERATE = 8
)

type GeneralACL struct {
	Browse   bool
	Reply    bool
	Post     bool
	Moderate bool
}

// Resolve the permissions an email address has on a list: everything if it
// owns the list, otherwise its own grant, otherwise the list default.
func UserACL(ctx context.Context, tx *sql.Tx, listID int, email string) (*GeneralACL, error) {
	var access uint
	row := tx.QueryRowContext(ctx,
		`SELECT COALESCE( (
			SELECT 0xF
			FROM list l JOIN "user" u ON l.owner_id = u.id
			WHERE l.id = $1 AND u.email = $2
		), (
			SELECT a.permissions
			FROM access a LEFT OUTER JOIN "user" u ON a.user_id = u.id
			WHERE a.list_id = $1 AND (u.email = $2 OR a.email = $2)
			LIMIT 1
		), (
			SELECT default_access
			FROM list
			WHERE list.id = $1
		) );`,
		listID, email,
	)
	if err := row.Scan(&access); err != nil {
		return nil, err
	}
	return &GeneralACL{
		Browse:   access&ACCESS_BROWSE == ACCESS_BROWSE,
		Reply:    access&ACCESS_REPLY == ACCESS_REPLY,
		Post:     access&ACCESS_POST == ACCESS_POST,
		Moderate: access&ACCESS_MODERATE == ACCESS_MODERATE,
	}, nil
}
