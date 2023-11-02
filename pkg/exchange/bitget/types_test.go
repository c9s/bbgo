package bitget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestKLine_ToGlobal(t *testing.T) {
	startTime := int64(1698744600000)
	interval := types.Interval1m
	k := KLine{
		StartTime:    types.NewMillisecondTimestampFromInt(startTime),
		OpenPrice:    fixedpoint.NewFromFloat(34361.49),
		HighestPrice: fixedpoint.NewFromFloat(34458.98),
		LowestPrice:  fixedpoint.NewFromFloat(34355.53),
		ClosePrice:   fixedpoint.NewFromFloat(34416.41),
		Volume:       fixedpoint.NewFromFloat(99.6631),
	}

	assert.Equal(t, types.KLine{
		Exchange:                 types.ExchangeBitget,
		Symbol:                   "BTCUSDT",
		StartTime:                types.Time(types.NewMillisecondTimestampFromInt(startTime).Time()),
		EndTime:                  types.Time(types.NewMillisecondTimestampFromInt(startTime).Time().Add(interval.Duration() - time.Millisecond)),
		Interval:                 interval,
		Open:                     fixedpoint.NewFromFloat(34361.49),
		Close:                    fixedpoint.NewFromFloat(34416.41),
		High:                     fixedpoint.NewFromFloat(34458.98),
		Low:                      fixedpoint.NewFromFloat(34355.53),
		Volume:                   fixedpoint.NewFromFloat(99.6631),
		QuoteVolume:              fixedpoint.Zero,
		TakerBuyBaseAssetVolume:  fixedpoint.Zero,
		TakerBuyQuoteAssetVolume: fixedpoint.Zero,
		LastTradeID:              0,
		NumberOfTrades:           0,
		Closed:                   false,
	}, k.ToGlobal(interval, "BTCUSDT"))
}
