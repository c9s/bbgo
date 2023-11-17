package csvsource

import (
	"fmt"
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
	Symbols     []string
	Path        string
}

var (
	expectedCandles = []int{864, 144, 72}
	intervals       = []types.Interval{types.Interval5m, types.Interval30m, types.Interval1h}
	symbols         = []string{"FXSUSDT", "BTCUSDT"}
	since           = time.Date(2023, 11, 17, 0, 0, 0, 0, time.UTC)
	until           = time.Date(2023, 11, 19, 0, 0, 0, 0, time.UTC)
)

func Test_CSV_Download(t *testing.T) {
	// if _, ok := os.LookupEnv("TEST_CSV_DOWNLOADER"); !ok {
	// 	t.Skip()
	// }
	var tests = []DownloadTester{
		{
			Exchange:    types.ExchangeBinance,
			Reader:      NewBinanceCSVTickReader,
			Market:      SPOT,
			Granularity: AGGTRADES,
			Symbols:     symbols,
			Path:        "testdata/binance",
		},
		{
			Exchange:    types.ExchangeBybit,
			Reader:      NewBybitCSVTickReader,
			Market:      FUTURES,
			Granularity: AGGTRADES,
			Symbols:     symbols,
			Path:        "testdata/bybit",
		},
		{
			Exchange:    types.ExchangeOKEx,
			Reader:      NewOKExCSVTickReader,
			Market:      SPOT,
			Granularity: AGGTRADES,
			Symbols:     symbols,
			Path:        "testdata/okex",
		},
	}

	for _, tt := range tests {
		for _, symbol := range tt.Symbols {
			path := filepath.Join(tt.Path, symbol)
			err := Download(
				path,
				symbol,
				tt.Exchange,
				tt.Market,
				tt.Granularity,
				since,
				until,
			)
			assert.NoError(t, err)

			klineMap, err := ReadTicksFromCSVWithDecoder(
				filepath.Join(path, string(tt.Granularity)),
				symbol,
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
						symbol,
						interval.String(),
						expectedCandles[i],
					),
				)

				err = WriteKLines(path, symbol, klines)
				assert.NoError(t, err)
			}
		}

		// err = os.RemoveAll(tt.Path)
		// assert.NoError(t, err)
	}
}
