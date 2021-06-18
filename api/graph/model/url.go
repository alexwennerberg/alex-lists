package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

// XXX: gqlgen bug prevents us from using type URL *url.URL
type URL struct {
	url *url.URL
}

func (u *URL) UnmarshalGQL(v interface{}) error {
	raw, ok := v.(string)
	if !ok {
		return fmt.Errorf("Mail format is a base64-encoded string")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	u.url = parsed
	return nil
}

func (u URL) MarshalGQL(w io.Writer) {
	data, err := json.Marshal(u.url.String())
	if err != nil {
		panic(err)
	}
	w.Write(data)
}
