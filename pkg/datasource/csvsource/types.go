package csvsource

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type CsvTick struct {
	Timestamp       int64            `json:"timestamp"`
	Symbol          string           `json:"symbol"`
	Side            string           `json:"side"`
	TickDirection   string           `json:"tickDirection"`
	Size            fixedpoint.Value `json:"size"`
	Price           fixedpoint.Value `json:"price"`
	HomeNotional    fixedpoint.Value `json:"homeNotional"`
	ForeignNotional fixedpoint.Value `json:"foreignNotional"`
}
