package model

import "strings"

// The part of the list table the ingress reads. The mimetype columns are
// stored as comma separated lists.
type MailingList struct {
	ID            int
	RawPermitMime string
	RawRejectMime string
}

func (list *MailingList) PermitMime() []string {
	if len(list.RawPermitMime) == 0 {
		return nil
	}
	return strings.Split(list.RawPermitMime, ",")
}

func (list *MailingList) RejectMime() []string {
	if len(list.RawRejectMime) == 0 {
		return nil
	}
	return strings.Split(list.RawRejectMime, ",")
}
