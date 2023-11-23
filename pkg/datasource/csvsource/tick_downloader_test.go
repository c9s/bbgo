package csvsource

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/types"
)

type DownloadTester struct {
	Exchange    types.ExchangeName
	Reader      MakeCSVTickReader
	Market      MarketType
	Granularity DataType
	Symbol      string
	Path        string
}

var (
	expectedCandles = []int{864, 144, 72}
	intervals       = []types.Interval{types.Interval5m, types.Interval30m, types.Interval1h}
	since           = time.Date(2023, 11, 17, 0, 0, 0, 0, time.UTC)
	until           = time.Date(2023, 11, 19, 0, 0, 0, 0, time.UTC)
)

func Test_CSV_Download(t *testing.T) {
	if _, ok := os.LookupEnv("TEST_CSV_DOWNLOADER"); !ok {
		t.Skip()
	}
	var tests = []DownloadTester{
		{
			Exchange:    types.ExchangeBinance,
			Reader:      NewBinanceCSVTickReader,
			Market:      SPOT,
			Granularity: AGGTRADES,
			Symbol:      "FXSUSDT",
			Path:        "testdata/binance/FXSUSDT",
		},
		{
			Exchange:    types.ExchangeBybit,
			Reader:      NewBybitCSVTickReader,
			Market:      FUTURES,
			Granularity: AGGTRADES,
			Symbol:      "FXSUSDT",
			Path:        "testdata/bybit/FXSUSDT",
		},
		{
			Exchange:    types.ExchangeOKEx,
			Reader:      NewOKExCSVTickReader,
			Market:      SPOT,
			Granularity: AGGTRADES,
			Symbol:      "FXSUSDT",
			Path:        "testdata/okex/FXSUSDT",
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
			filepath.Join(tt.Path, string(tt.Granularity)),
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
