// Package archive reads the compressed CSV archives that exchanges publish as
// historical market data dumps, and caches them locally.
//
// It deliberately caches the original archive bytes rather than a normalized
// form. The previous implementation rewrote every download into a five-column
// CSV, which discarded first/last trade ids, quote volumes and real trade ids,
// and made a cached day impossible to re-decode when the decoder improved.
package archive

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

// Epoch magnitude boundaries. Any timestamp for a date in this century is
// unambiguous by magnitude: seconds are ~1.7e9 (10 digits), milliseconds
// ~1.7e12 (13), microseconds ~1.7e15 (16), nanoseconds ~1.7e18 (19). The gaps
// between the units are three orders of magnitude, so the classification holds
// for any date from 1973 to roughly the year 5138.
const (
	epochMilliLowerBound = int64(1e11)
	epochMicroLowerBound = int64(1e14)
	epochNanoLowerBound  = int64(1e17)
)

// EpochUnit is the precision of a parsed epoch timestamp.
type EpochUnit uint8

const (
	EpochSeconds EpochUnit = iota
	EpochMilliseconds
	EpochMicroseconds
	EpochNanoseconds
)

func (u EpochUnit) String() string {
	switch u {
	case EpochSeconds:
		return "s"
	case EpochMilliseconds:
		return "ms"
	case EpochMicroseconds:
		return "us"
	case EpochNanoseconds:
		return "ns"
	}
	return "unknown"
}

// ClassifyEpoch reports the unit of a raw epoch value from its magnitude.
func ClassifyEpoch(v int64) EpochUnit {
	switch {
	case v >= epochNanoLowerBound:
		return EpochNanoseconds
	case v >= epochMicroLowerBound:
		return EpochMicroseconds
	case v >= epochMilliLowerBound:
		return EpochMilliseconds
	default:
		return EpochSeconds
	}
}

// ParseEpochNano parses an epoch timestamp of unknown precision and returns it
// in nanoseconds, together with the unit it was recognized as.
//
// The unit must be inferred per value, not assumed per dataset. Binance
// switched its spot archives from milliseconds to microseconds at some point
// between 2023-11 and 2025-01 without renaming the column and without
// announcing a cutover date, while the futures archives stayed on
// milliseconds. A date-keyed lookup table would therefore be wrong the day it
// was written, and would stay wrong because Binance regenerates historical
// files.
//
// Integers are parsed exactly; the float fallback exists only for the
// scientific-notation form ("1.70027E+12") that older Binance futures kline
// files use, and is lossy above 2^53, which is why it is not the fast path.
func ParseEpochNano(s string) (int64, EpochUnit, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, 0, fmt.Errorf("archive: empty timestamp")
	}

	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return 0, 0, fmt.Errorf("archive: cannot parse timestamp %q: %w", s, err)
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return 0, 0, fmt.Errorf("archive: timestamp %q is not finite", s)
		}
		v = int64(math.Round(f))
	}

	if v < 0 {
		return 0, 0, fmt.Errorf("archive: negative timestamp %q", s)
	}

	unit := ClassifyEpoch(v)
	switch unit {
	case EpochSeconds:
		return v * int64(time.Second), unit, nil
	case EpochMilliseconds:
		return v * int64(time.Millisecond), unit, nil
	case EpochMicroseconds:
		return v * int64(time.Microsecond), unit, nil
	default:
		return v, unit, nil
	}
}

// ParseEpoch is ParseEpochNano returning a UTC time.
func ParseEpoch(s string) (time.Time, error) {
	ns, _, err := ParseEpochNano(s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, ns).UTC(), nil
}

// DateTimeLayout is the layout Binance uses for the bookDepth and metrics
// datasets, which carry a formatted timestamp rather than an epoch. The files
// carry no timezone; Binance publishes them in UTC.
const DateTimeLayout = "2006-01-02 15:04:05"

// ParseDateTime parses the formatted timestamp used by bookDepth and metrics.
func ParseDateTime(s string) (time.Time, error) {
	return time.ParseInLocation(DateTimeLayout, strings.TrimSpace(s), time.UTC)
}

// Reader streams CSV records out of an archive, handling the two layouts
// exchanges mix within one dataset family: with and without a header row.
type Reader struct {
	csv *csv.Reader

	// columns maps a header name to its index when a header row was present.
	// It is nil for a headerless file.
	columns map[string]int

	header  []string
	line    int
	sniffed bool
}

// NewReader wraps r. Records are returned by Read, which reuses its backing
// array, so a decoder must not retain the returned slice.
func NewReader(r io.Reader) *Reader {
	cr := csv.NewReader(r)
	cr.ReuseRecord = true
	// Archives are machine-generated but the column count varies between
	// datasets and over time, so leave the count unchecked and let decoders
	// validate what they need.
	cr.FieldsPerRecord = -1

	return &Reader{csv: cr}
}

// Header returns the header row, or nil if the file had none.
func (r *Reader) Header() []string { return r.header }

// Columns returns the header name to index map, or nil if the file had none.
func (r *Reader) Columns() map[string]int { return r.columns }

// Line returns the 1-based index of the last record returned by Read,
// counting the header row when present.
func (r *Reader) Line() int { return r.line }

// Read returns the next data record, transparently consuming a header row if
// the file has one. It returns io.EOF at the end.
//
// A header is detected by content, not by dataset: Binance added header rows to
// the futures archives while the spot archives stayed headerless, so the first
// field of the first record is tested for being numeric instead.
func (r *Reader) Read() ([]string, error) {
	if !r.sniffed {
		r.sniffed = true

		rec, err := r.csv.Read()
		if err != nil {
			return nil, err
		}
		r.line++

		if !looksNumeric(rec[0]) {
			r.header = append([]string(nil), rec...)
			r.columns = make(map[string]int, len(r.header))
			for i, name := range r.header {
				r.columns[strings.TrimSpace(name)] = i
			}
		} else {
			return rec, nil
		}
	}

	rec, err := r.csv.Read()
	if err != nil {
		return nil, err
	}
	r.line++
	return rec, nil
}

// Column returns the index of a header column. It reports false for a
// headerless file or an absent column, so a decoder can fall back to fixed
// positions.
func (r *Reader) Column(name string) (int, bool) {
	if r.columns == nil {
		return 0, false
	}
	i, ok := r.columns[name]
	return i, ok
}

// RequireColumns checks that every named column is present when the file has a
// header. It is how a decoder fails loudly on an inserted column instead of
// silently shifting every field.
func (r *Reader) RequireColumns(names ...string) error {
	if r.columns == nil {
		return nil
	}
	for _, name := range names {
		if _, ok := r.columns[name]; !ok {
			return fmt.Errorf("archive: missing column %q, header is %v", name, r.header)
		}
	}
	return nil
}

// looksNumeric reports whether s parses as a number, which is what separates a
// data row from a header row in these archives. It also accepts the scientific
// notation some Binance kline files use.
func looksNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	_, err := strconv.ParseFloat(s, 64)
	return err == nil
}
