package binancecsv

import (
	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/archive"
	"github.com/c9s/bbgo/pkg/types"
)

// RecordMeta is the context a decoder needs beyond the record itself.
type RecordMeta struct {
	Exchange types.ExchangeName
	Market   Market
	Symbol   string
	Dataset  string
	Interval types.Interval

	// Columns maps a header name to its index when the archive had a header
	// row, and is nil otherwise. Binance added headers to the futures archives
	// while the spot archives stayed headerless, so a decoder must handle both:
	// look the column up by name when Columns is set, fall back to a fixed
	// position when it is not.
	Columns map[string]int

	File   string
	LineNo int
}

// column returns the index of name, falling back to fallback for a headerless
// archive or an absent column.
func (m RecordMeta) column(name string, fallback int) int {
	if m.Columns == nil {
		return fallback
	}
	if i, ok := m.Columns[name]; ok {
		return i
	}
	return fallback
}

// RecordDecoder converts one CSV record into zero or more events.
//
// Decode appends to dst and returns the extended slice, so a decoder that
// produces several events per record — bookDepth emits one per percentage band
// — allocates nothing per record.
//
// An implementation must not retain the record slice: archive.Reader reuses its
// backing array between records.
type RecordDecoder interface {
	// Columns returns the header this decoder expects, or nil for a dataset
	// that has never had one. It is used to fail loudly when Binance inserts or
	// renames a column, instead of silently shifting every field.
	Columns() []string

	// Decode appends the events decoded from record to dst.
	Decode(dst []marketdata.Event, record []string, meta RecordMeta) ([]marketdata.Event, error)
}

// parseEventTimeNano is the shared timestamp path. It exists so every decoder
// inherits the magnitude-based unit detection rather than assuming a precision
// per dataset; see archive.ParseEpochNano for why that matters.
func parseEventTimeNano(s string) (int64, error) {
	ns, _, err := archive.ParseEpochNano(s)
	return ns, err
}
