// SPDX-License-Identifier: AGPL-3.0-only
// Copyright (c) 2024 Robin Jarry

package main

import (
	"database/sql"
	"fmt"
	"net/mail"
	"strconv"

	"git.sr.ht/~sircmpwn/core-go/config"
	"git.sr.ht/~sircmpwn/core-go/crypto"
	_ "github.com/lib/pq"
	"github.com/vaughan0/go-ini"
)

type WorkerConfig struct {
	Sock           string
	Protocol       string
	SockGroup      string
	RejectUrl      string
	MetricsSock    string
	Redirects      ini.Section
	MaxMessageSize int64
	Domain         string
	MailerAddr     string
	OwnerAddr      string
	OriginUrl      string
}

var Config = WorkerConfig{
	Sock:           ":25",
	Protocol:       "smtp",
	MaxMessageSize: 8 * 1024 * 1024,
	MetricsSock:    ":8006",
	RejectUrl:      "https://useplaintext.email",
}

var SrhtConfig ini.File

func LoadConfig() error {
	var err error

	c := config.LoadConfig()
	crypto.InitCrypto(c)

	for k, v := range c.Section("lists.sr.ht::worker") {
		switch k {
		case "sock":
			Config.Sock = v
		case "protocol":
			Config.Protocol = v
		case "sock-group":
			Config.SockGroup = v
		case "reject-url":
			Config.RejectUrl = v
		case "max-message-size":
			Config.MaxMessageSize, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				return fmt.Errorf("max-message-size: %w", err)
			}
		case "metrics-sock":
			Config.MetricsSock = v
		}
	}
	Config.Domain, _ = c.Get("lists.sr.ht", "posting-domain")
	Config.OriginUrl, _ = c.Get("lists.sr.ht", "origin")
	Config.MailerAddr = "mailer@" + Config.Domain
	Config.Redirects = c.Section("lists.sr.ht::redirects")
	var owner mail.Address
	owner.Name, _ = c.Get("sr.ht", "owner-name")
	owner.Address, _ = c.Get("sr.ht", "owner-email")
	Config.OwnerAddr = owner.String()

	SrhtConfig = c

	return nil
}

// Connect to the database the API server writes to. The ingress archives mail
// and manages subscriptions there directly, it does not go through the API.
func OpenDB() (*sql.DB, error) {
	pgcs, ok := SrhtConfig.Get(LISTS_SERVICE, "connection-string")
	if !ok {
		return nil, fmt.Errorf("no connection-string in [%s]", LISTS_SERVICE)
	}
	return sql.Open("postgres", pgcs)
}
