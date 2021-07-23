package model

import (
	"fmt"
	"strings"
	"time"

	"git.sr.ht/~sircmpwn/core-go/database"
)

type PatchsetTool struct {
	ID      int       `json:"id"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	Details string    `json:"details"`
	Key     string    `json:"key"`

	PatchsetID int
	RawIcon    string

	alias  string
	fields *database.ModelFields
}

func (tool *PatchsetTool) Icon() ToolIcon {
	icon := ToolIcon(strings.ToUpper(tool.RawIcon))
	if !icon.IsValid() {
		panic(fmt.Errorf("patchset_tool %d has invalid icon %s",
			tool.ID, tool.RawIcon))
	}
	return icon
}

func (tool *PatchsetTool) As(alias string) *PatchsetTool {
	tool.alias = alias
	return tool
}

func (tool *PatchsetTool) Alias() string {
	return tool.alias
}

func (tool *PatchsetTool) Table() string {
	return "patchset_tool"
}

func (tool *PatchsetTool) Fields() *database.ModelFields {
	if tool.fields != nil {
		return tool.fields
	}
	tool.fields = &database.ModelFields{
		Fields: []*database.FieldMap{
			{"updated", "updated", &tool.Updated},
			{"icon", "icon", &tool.RawIcon},
			{"details", "details", &tool.Details},
			{"key", "key", &tool.Key},

			// Always fetch:
			{"id", "", &tool.ID},
			{"created", "", &tool.Created},
			{"patchset_id", "", &tool.PatchsetID},
		},
	}
	return tool.fields
}
