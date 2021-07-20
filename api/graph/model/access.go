package model

import (
	"time"

	"git.sr.ht/~sircmpwn/core-go/database"
)

type GeneralACL struct {
	Browse   bool `json:"browse"`
	Reply    bool `json:"reply"`
	Post     bool `json:"post"`
	Moderate bool `json:"moderate"`
}

func (GeneralACL) IsACL() {}

type MailingListACL struct {
	ID       int       `json:"id"`
	Created  time.Time `json:"created"`

	UserID        *int
	Email         *string
	MailingListID int
	RawAccess     uint

	alias  string
	fields *database.ModelFields
}

func (MailingListACL) IsACL() {}

func (acl *MailingListACL) As(alias string) *MailingListACL {
	acl.alias = alias
	return acl
}

func (acl *MailingListACL) Alias() string {
	return acl.alias
}

func (acl *MailingListACL) Table() string {
	return "access"
}

func (acl *MailingListACL) Browse() bool {
	return acl.RawAccess & ACCESS_BROWSE != 0
}

func (acl *MailingListACL) Reply() bool {
	return acl.RawAccess & ACCESS_REPLY != 0
}

func (acl *MailingListACL) Post() bool {
	return acl.RawAccess & ACCESS_POST != 0
}

func (acl *MailingListACL) Moderate() bool {
	return acl.RawAccess & ACCESS_MODERATE != 0
}

func (acl *MailingListACL) Fields() *database.ModelFields {
	if acl.fields != nil {
		return acl.fields
	}
	acl.fields = &database.ModelFields{
		Fields: []*database.FieldMap{
			{ "created", "created", &acl.Created },

			// Always fetch:
			{ "id", "", &acl.ID },
			{ "permissions", "", &acl.RawAccess },
			{ "list_id", "", &acl.MailingListID },
			{ "user_id", "", &acl.UserID },
			{ "email", "", &acl.Email },
		},
	}
	return acl.fields
}
