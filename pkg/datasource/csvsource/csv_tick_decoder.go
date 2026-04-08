package csvsource

import (
	"strconv"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// CSVTickDecoder decodes a normalized CSV record into a CsvTick.
// The normalized format is: trade_id, side, size, price, timestamp
// where side is "buy" or "sell" and timestamp is in milliseconds.
func CSVTickDecoder(row []string) (*CsvTick, error) {
	if len(row) < 5 {
		return nil, ErrNotEnoughColumns
	}

	id, err := strconv.ParseUint(row[0], 10, 64)
	if err != nil {
		return nil, ErrInvalidIDFormat
	}

	side, err := types.StrToSideType(row[1])
	if err != nil {
		return nil, ErrInvalidOrderSideFormat
	}

	size, err := fixedpoint.NewFromString(row[2])
	if err != nil {
		return nil, ErrInvalidVolumeFormat
	}

	price, err := fixedpoint.NewFromString(row[3])
	if err != nil {
		return nil, ErrInvalidPriceFormat
	}

	ts, err := strconv.ParseInt(row[4], 10, 64)
	if err != nil {
		return nil, ErrInvalidTimeFormat
	}

	isBuyerMaker := side == types.SideTypeSell

	return &CsvTick{
		TradeID:         id,
		Side:            side,
		Size:            size,
		Price:           price,
		IsBuyerMaker:    isBuyerMaker,
		HomeNotional:    size,
		ForeignNotional: price.Mul(size),
		Timestamp:       types.NewMillisecondTimestampFromInt(ts),
	}, nil
}
