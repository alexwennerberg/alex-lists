package graph

import (
	"regexp"

	"git.sr.ht/~sircmpwn/lists.sr.ht/api/graph/model"
)

type Resolver struct{}

var (
	listNameRE = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

func ACLInputBits(input model.ACLInput) uint {
	var bits uint
	if input.Browse {
		bits |= model.ACCESS_BROWSE
	}
	if input.Reply {
		bits |= model.ACCESS_REPLY
	}
	if input.Post {
		bits |= model.ACCESS_POST
	}
	if input.Moderate {
		bits |= model.ACCESS_MODERATE
	}
	return bits
}
