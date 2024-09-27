package csvsource

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

type DownloadTester struct {
	Exchange    types.ExchangeName
	Reader      MakeCSVTickReader
	Market      types.MarketType
	Granularity types.MarketDataType
	Symbol      string
	Path        string
}

var (
	expectedCandles = []int{1440, 48, 24}
	intervals       = []types.Interval{types.Interval1m, types.Interval30m, types.Interval1h}
	until           = time.Now().Round(0)
	since           = until.Add(-24 * time.Hour)
)

func Test_CSV_Download(t *testing.T) {
	if _, ok := os.LookupEnv("TEST_CSV_DOWNLOADER"); !ok {
		t.Skip()
	}
	var tests = []DownloadTester{
		{
			Exchange:    types.ExchangeBinance,
			Reader:      NewBinanceCSVTickReader,
			Market:      types.MarketTypeSpot,
			Granularity: types.MarketDataTypeAggTrades,
			Symbol:      "FXSUSDT",
			Path:        "testdata/binance/FXSUSDT",
		},
		{
			Exchange:    types.ExchangeBybit,
			Reader:      NewBybitCSVTickReader,
			Market:      types.MarketTypeFutures,
			Granularity: types.MarketDataTypeAggTrades,
			Symbol:      "FXSUSDT",
			Path:        "testdata/bybit/FXSUSDT",
		},
		{
			Exchange:    types.ExchangeOKEx,
			Reader:      NewOKExCSVTickReader,
			Market:      types.MarketTypeSpot,
			Granularity: types.MarketDataTypeAggTrades,
			Symbol:      "BTCUSDT",
			Path:        "testdata/okex/BTCUSDT",
		},
	}

	for _, tt := range tests {
		err := Download(
			tt.Path,
			tt.Symbol,
			tt.Exchange,
			tt.Market,
			tt.Granularity,
			since,
			until,
		)
		assert.NoError(t, err)

		klineMap, err := ReadTicksFromCSVWithDecoder(
			tt.Path,
			tt.Symbol,
			intervals,
			MakeCSVTickReader(tt.Reader),
		)
		assert.NoError(t, err)

		for i, interval := range intervals {
			klines := klineMap[interval]

			assert.Equal(
				t,
				expectedCandles[i],
				len(klines),
				fmt.Sprintf("%s: %s/%s should have %d kLines",
					tt.Exchange.String(),
					tt.Symbol,
					interval.String(),
					expectedCandles[i],
				),
			)

			err = WriteKLines(tt.Path, tt.Symbol, klines)
			assert.NoError(t, err)
		}

		err = os.RemoveAll(tt.Path)
		assert.NoError(t, err)
	}
}
