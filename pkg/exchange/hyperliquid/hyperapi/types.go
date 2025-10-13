package hyperapi

import (
	"encoding/json"

	"github.com/c9s/bbgo/pkg/types"
)

var (
	SupportedIntervals = map[types.Interval]int{
		types.Interval1m:  1 * 60,
		types.Interval3m:  3 * 60,
		types.Interval5m:  5 * 60,
		types.Interval15m: 15 * 60,
		types.Interval30m: 30 * 60,
		types.Interval1h:  60 * 60,
		types.Interval2h:  60 * 60 * 2,
		types.Interval4h:  60 * 60 * 4,
		types.Interval8h:  60 * 60 * 8,
		types.Interval12h: 60 * 60 * 12,
		types.Interval1d:  60 * 60 * 24,
		types.Interval3d:  60 * 60 * 24 * 3,
		types.Interval1w:  60 * 60 * 24 * 7,
		types.Interval1mo: 60 * 60 * 24 * 30,
	}
)

type SignatureResult struct {
	R string `json:"r"`
	S string `json:"s"`
	V int    `json:"v"`
}

type InfoReqType string

const (
	Meta        InfoReqType = "meta"
	SpotMeta    InfoReqType = "spotMeta"
	SubmitOrder InfoReqType = "order"
	CancelOrder InfoReqType = "cancel"
)

type APIResponse struct {
	Status   string `json:"status"`
	Response struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	} `json:"response"`
}
