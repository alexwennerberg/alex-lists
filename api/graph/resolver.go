package graph

import (
	"regexp"
)

//go:generate go run github.com/99designs/gqlgen

type Resolver struct{}

var (
	listNameRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)
