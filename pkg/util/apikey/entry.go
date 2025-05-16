package apikey

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/util"
)

type Entry struct {
	Index  int
	Key    string
	Secret string
	Fields map[string]string
}

func (e *Entry) String() string {
	return fmt.Sprintf("%d) %s:%s %+v", e.Index, util.MaskKey(e.Key), util.MaskKey(e.Secret), e.Fields)
}

func (e *Entry) GetKeySecret() (key, secret string) {
	if e.Key != "" && e.Secret != "" {
		return e.Key, e.Secret
	}

	return "", ""
}

func (e *Entry) Get(field string) (val string, ok bool) {
	val, ok = e.Fields[field]
	return val, ok
}
