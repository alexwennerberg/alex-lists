package model

import (
	"time"
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
	Browse   bool      `json:"browse"`
	Reply    bool      `json:"reply"`
	Post     bool      `json:"post"`
	Moderate bool      `json:"moderate"`

	UserID        int
	MailingListID int
}

func (MailingListACL) IsACL() {}
