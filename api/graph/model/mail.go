package model

import (
	"github.com/emersion/go-message/mail"
	_ "github.com/emersion/go-message/charset"

	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
)

// XXX: Why does this need to be a struct? gqlgen vomits on []byte directly
type Mail struct {
	Data []byte
}

func (msg *Mail) UnmarshalGQL(v interface{}) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("Mail format is a base64-encoded string")
	}
	dec, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	msg.Data = dec
	return nil
}

func (msg Mail) MarshalGQL(w io.Writer) {
	data, err := json.Marshal(base64.StdEncoding.EncodeToString(msg.Data))
	if err != nil {
		panic(err)
	}
	w.Write(data)
}

func NewMail(data []byte) *Mail {
	return &Mail{data}
}

func (msg *Mail) Reader() (*mail.Reader, error) {
	return mail.CreateReader(bytes.NewBuffer(msg.Data))
}
