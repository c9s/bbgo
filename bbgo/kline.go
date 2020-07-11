package bbgo

import (
	"github.com/c9s/bbgo/pkg/types"
)

type KLineEvent struct {
	EventBase
	Symbol string       `json:"s"`
	KLine  *types.KLine `json:"k,omitempty"`
}

