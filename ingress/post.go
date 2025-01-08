// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (c) 2024 Robin Jarry

package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"git.sr.ht/~sircmpwn/core-go/email"
	apierr "git.sr.ht/~sircmpwn/lists.sr.ht/api/errors"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/mail"
)

func (b *Backend) Post(sender *Sender, data []byte, msg *message.Entity, list *MailingList) error {
	if !(sender.ACL.Post || (list.IsReply && sender.ACL.Reply)) {
		return &PostPermError{sender, list}
	}

	if err := Validate(sender, msg, list); err != nil {
		return err
	}

	msgID := msg.Header.Get("Message-ID")

	// Validate() consumed the message.Entity body, directly archive raw data
	err := ArchiveMessage(data, list)

	switch {
	case err == nil:
		log.Printf("Archived %s to %s", msgID, list.FullName())
		return b.ForwardMessage(data, list)

	case errors.Is(err, apierr.ErrDuplicateEmail):
		log.Printf("Dropping duplicate message %s on %s", msgID, list.FullName())
		return nil

	default:
		return err
	}
}

var reservedHeaders = []string{
	"List-Unsubscribe",
	"List-Subscribe",
	"List-Archive",
	"List-Post",
	"List-ID",
	"Sender",
}

func (b *Backend) ForwardMessage(data []byte, list *MailingList) error {
	var recipients []string
	alreadyCopied := make(map[string]bool)

	// fetch subscriber emails from database
	subscribers, err := LookupSubscribers(list)
	if err != nil {
		return err
	}

	// cannot fail, the message has already been validated
	msg, _ := message.Read(bytes.NewReader(data))

	// eliminate recipients that were already included in the original message
	header := mail.Header{Header: msg.Header}
	for _, name := range []string{"From", "To", "Cc"} {
		addresses, _ := header.AddressList(name)
		for _, addr := range addresses {
			alreadyCopied[addr.Address] = true
		}
	}
	for _, email := range subscribers {
		if !alreadyCopied[email] {
			recipients = append(recipients, email)
		}
	}

	msgID := msg.Header.Get("Message-Id")

	if len(recipients) == 0 {
		log.Printf("No recipients to forward message %s to.", msgID)
		return nil
	}

	// prepare message with appropriate mailing list headers
	for _, h := range reservedHeaders {
		msg.Header.Del(h)
	}
	msg.Header.Set("List-Unsubscribe",
		fmt.Sprintf("<mailto:%s?subject=unsubscribe>",
			list.PlusAddress(CMD_UNSUBSCRIBE)))
	msg.Header.Set("List-Subscribe",
		fmt.Sprintf("<mailto:%s?subject=subscribe>",
			list.PlusAddress(CMD_SUBSCRIBE)))
	msg.Header.Set("List-Archive",
		fmt.Sprintf("<%s/%s>", Config.OriginUrl, list.FullName()))
	msg.Header.Set("Archived-At",
		fmt.Sprintf("<%s/%s/%s>",
			Config.OriginUrl, list.FullName(),
			msgID))
	msg.Header.Set("List-Post",
		fmt.Sprintf("<mailto:%s>", list.Address()))
	msg.Header.Set("List-ID",
		fmt.Sprintf("%s <%s.%s>",
			list.FullName(), list.FullName(), Config.Domain))
	msg.Header.Set("Sender",
		fmt.Sprintf("%s <%s>", list.FullName(), list.Address()))

	// forward the message to all subscribers
	log.Printf("Forwarding message %s to %d subscribers",
		msgID, len(recipients))
	ForwardsCounter.Inc()
	email.SendRaw(b.ctx, msg, recipients)
	return nil
}

var (
	requiredHeaders   = []string{"From", "Subject", "Message-Id"}
	prohibitedHeaders = []string{"Return-Receipt-To", "Disposition-Notification-To"}
)

func Validate(sender *Sender, msg *message.Entity, list *MailingList) error {
	for _, h := range requiredHeaders {
		if !msg.Header.Has(h) {
			return InvalidHeaderErrorf("The %s header is required.", h)
		}
	}
	for _, h := range prohibitedHeaders {
		if msg.Header.Has(h) {
			return InvalidHeaderErrorf("The %s header is prohibited.", h)
		}
	}
	if !msg.Header.Has("To") && !msg.Header.Has("Cc") {
		return InvalidHeaderErrorf("The To or Cc header is required.")
	}
	foundTextPart := false
	var rejected []string

	err := msg.Walk(func(path []int, part *message.Entity, err error) error {
		if err != nil {
			return err
		}
		contentType, _, err := part.Header.ContentType()
		if err != nil {
			contentType = "text/plain"
		}
		if strings.HasPrefix(contentType, "multipart/") {
			return nil
		}
		disp, _, err := part.Header.ContentDisposition()
		if err != nil {
			disp = "inline"
		}
		if contentType == "text/plain" && disp == "inline" {
			foundTextPart = true
		}
		permit := false
		for _, mime := range list.PermitMimetypes {
			if match, _ := filepath.Match(mime, contentType); match {
				permit = true
				break
			}
		}
		if !permit {
			rejected = append(rejected, contentType)
		}
		for _, mime := range list.RejectMimetypes {
			if match, _ := filepath.Match(mime, contentType); match {
				rejected = append(rejected, contentType)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(rejected) != 0 {
		if rejected[0] == "text/html" && !foundTextPart {
			return &HtmlError{sender}
		} else {
			return &ForbidenMimeError{sender, rejected[0]}
		}
	}
	if !foundTextPart {
		return &NoTextError{sender}
	}
	return nil
}
