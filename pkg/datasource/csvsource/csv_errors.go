package csvsource

import "errors"

var (
	// ErrNotEnoughColumns is returned when the CSV price record does not have enough columns.
	ErrNotEnoughColumns = errors.New("not enough columns")

	// ErrInvalidTimeFormat is returned when the CSV price record does not have a valid time unix milli format.
	ErrInvalidIDFormat = errors.New("cannot parse trade id string")

	// ErrInvalidBoolFormat is returned when the CSV isBuyerMaker record does not have a valid bool representation.
	ErrInvalidBoolFormat = errors.New("cannot parse bool to string")

	// ErrInvalidTimeFormat is returned when the CSV price record does not have a valid time unix milli format.
	ErrInvalidTimeFormat = errors.New("cannot parse time string")

	// ErrInvalidOrderSideFormat is returned when the CSV side record does not have a valid buy or sell string.
	ErrInvalidOrderSideFormat = errors.New("cannot parse order side string")

	// ErrInvalidPriceFormat is returned when the CSV price record does not prices in expected format.
	ErrInvalidPriceFormat = errors.New("OHLC prices must be valid number format")

	// ErrInvalidVolumeFormat is returned when the CSV price record does not have a valid volume format.
	ErrInvalidVolumeFormat = errors.New("volume must be valid number format")
)
