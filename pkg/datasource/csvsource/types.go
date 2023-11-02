package csvsource

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type CsvTick struct {
	Symbol          string                     `json:"symbol"`
	TickDirection   string                     `json:"tickDirection"`
	Side            types.SideType             `json:"side"`
	Size            fixedpoint.Value           `json:"size"`
	Price           fixedpoint.Value           `json:"price"`
	HomeNotional    fixedpoint.Value           `json:"homeNotional"`
	ForeignNotional fixedpoint.Value           `json:"foreignNotional"`
	Timestamp       types.MillisecondTimestamp `json:"timestamp"`
}
