package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSortTradesAscending(t *testing.T) {
	var trades = []Trade{
		{
			ID:      1,
			Symbol:  "BTCUSDT",
			Side:    SideTypeBuy,
			IsBuyer: false,
			IsMaker: false,
			Time:    Time(time.Unix(2000, 0)),
		},
		{
			ID:      2,
			Symbol:  "BTCUSDT",
			Side:    SideTypeBuy,
			IsBuyer: false,
			IsMaker: false,
			Time:    Time(time.Unix(1000, 0)),
		},
	}
	trades = SortTradesAscending(trades)
	assert.True(t, trades[0].Time.Before(trades[1].Time.Time()))
}
