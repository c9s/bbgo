package binancecsv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/types"
)

func date(t *testing.T, s string) time.Time {
	t.Helper()
	got, err := time.Parse(time.DateOnly, s)
	require.NoError(t, err)
	return got
}

// TestFileRef_URL pins the URL layout against paths verified live against
// data.binance.vision. The kline families name the file after the interval
// rather than the dataset, which is the only real irregularity.
func TestFileRef_URL(t *testing.T) {
	tests := []struct {
		name string
		ref  FileRef
		want string
	}{
		{
			name: "spot daily aggTrades",
			ref: FileRef{
				Market: MarketSpot, Period: PeriodDaily, Dataset: DatasetAggTrades,
				Symbol: "BTCUSDT", Date: date(t, "2026-09-15"),
			},
			want: BaseURL + "/data/spot/daily/aggTrades/BTCUSDT/BTCUSDT-aggTrades-2026-09-15.zip",
		},
		{
			name: "spot monthly aggTrades",
			ref: FileRef{
				Market: MarketSpot, Period: PeriodMonthly, Dataset: DatasetAggTrades,
				Symbol: "BTCUSDT", Date: date(t, "2026-08-01"),
			},
			want: BaseURL + "/data/spot/monthly/aggTrades/BTCUSDT/BTCUSDT-aggTrades-2026-08.zip",
		},
		{
			name: "usdm daily aggTrades",
			ref: FileRef{
				Market: MarketUSDMFutures, Period: PeriodDaily, Dataset: DatasetAggTrades,
				Symbol: "BTCUSDT", Date: date(t, "2026-09-15"),
			},
			want: BaseURL + "/data/futures/um/daily/aggTrades/BTCUSDT/BTCUSDT-aggTrades-2026-09-15.zip",
		},
		{
			name: "coinm daily aggTrades",
			ref: FileRef{
				Market: MarketCOINMFutures, Period: PeriodDaily, Dataset: DatasetAggTrades,
				Symbol: "BTCUSD_PERP", Date: date(t, "2026-09-15"),
			},
			want: BaseURL + "/data/futures/cm/daily/aggTrades/BTCUSD_PERP/BTCUSD_PERP-aggTrades-2026-09-15.zip",
		},
		{
			name: "usdm daily klines name the file after the interval",
			ref: FileRef{
				Market: MarketUSDMFutures, Period: PeriodDaily, Dataset: DatasetKLines,
				Symbol: "BTCUSDT", Interval: types.Interval1h, Date: date(t, "2026-09-15"),
			},
			want: BaseURL + "/data/futures/um/daily/klines/BTCUSDT/1h/BTCUSDT-1h-2026-09-15.zip",
		},
		{
			name: "usdm monthly klines",
			ref: FileRef{
				Market: MarketUSDMFutures, Period: PeriodMonthly, Dataset: DatasetKLines,
				Symbol: "BTCUSDT", Interval: types.Interval1h, Date: date(t, "2026-08-01"),
			},
			want: BaseURL + "/data/futures/um/monthly/klines/BTCUSDT/1h/BTCUSDT-1h-2026-08.zip",
		},
		{
			name: "usdm markPriceKlines",
			ref: FileRef{
				Market: MarketUSDMFutures, Period: PeriodDaily, Dataset: DatasetMarkPriceKLines,
				Symbol: "BTCUSDT", Interval: types.Interval1h, Date: date(t, "2026-09-15"),
			},
			want: BaseURL + "/data/futures/um/daily/markPriceKlines/BTCUSDT/1h/BTCUSDT-1h-2026-09-15.zip",
		},
		{
			name: "usdm bookDepth",
			ref: FileRef{
				Market: MarketUSDMFutures, Period: PeriodDaily, Dataset: DatasetBookDepth,
				Symbol: "BTCUSDT", Date: date(t, "2026-09-15"),
			},
			want: BaseURL + "/data/futures/um/daily/bookDepth/BTCUSDT/BTCUSDT-bookDepth-2026-09-15.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.ref.URL())
			assert.Equal(t, tt.want+".CHECKSUM", tt.ref.ChecksumURL())
		})
	}
}

func TestEnumerateFiles(t *testing.T) {
	t.Run("daily, half-open range", func(t *testing.T) {
		refs := enumerateFiles(MarketUSDMFutures, PeriodDaily, DatasetAggTrades, "BTCUSDT", "",
			date(t, "2026-09-15"), date(t, "2026-09-18"))

		require.Len(t, refs, 3, "the end of the range is exclusive")
		assert.Equal(t, "BTCUSDT-aggTrades-2026-09-15.zip", refs[0].FileName())
		assert.Equal(t, "BTCUSDT-aggTrades-2026-09-17.zip", refs[2].FileName())
	})

	t.Run("daily, partial first and last day", func(t *testing.T) {
		since := date(t, "2026-09-15").Add(18 * time.Hour)
		until := date(t, "2026-09-17").Add(6 * time.Hour)

		refs := enumerateFiles(MarketUSDMFutures, PeriodDaily, DatasetAggTrades, "BTCUSDT", "",
			since, until)

		require.Len(t, refs, 3, "a partial day still needs its whole archive")
		assert.Equal(t, "BTCUSDT-aggTrades-2026-09-15.zip", refs[0].FileName())
		assert.Equal(t, "BTCUSDT-aggTrades-2026-09-17.zip", refs[2].FileName())
	})

	t.Run("monthly", func(t *testing.T) {
		refs := enumerateFiles(MarketSpot, PeriodMonthly, DatasetAggTrades, "BTCUSDT", "",
			date(t, "2026-07-10"), date(t, "2026-09-05"))

		require.Len(t, refs, 3)
		assert.Equal(t, "BTCUSDT-aggTrades-2026-07.zip", refs[0].FileName())
		assert.Equal(t, "BTCUSDT-aggTrades-2026-09.zip", refs[2].FileName())
	})
}

func TestParseMarket(t *testing.T) {
	for in, want := range map[string]Market{
		"spot":       MarketSpot,
		"um":         MarketUSDMFutures,
		"usdm":       MarketUSDMFutures,
		"futures":    MarketUSDMFutures,
		"futures/um": MarketUSDMFutures,
		"cm":         MarketCOINMFutures,
		"COINM":      MarketCOINMFutures,
	} {
		got, err := ParseMarket(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}

	_, err := ParseMarket("perpetual")
	assert.Error(t, err)
}

func TestPeriod_Truncate(t *testing.T) {
	mid := date(t, "2026-09-15").Add(13 * time.Hour)

	assert.Equal(t, date(t, "2026-09-15"), PeriodDaily.truncate(mid))
	assert.Equal(t, date(t, "2026-09-01"), PeriodMonthly.truncate(mid))

	assert.Equal(t, date(t, "2026-09-16"), PeriodDaily.next(mid))
	assert.Equal(t, date(t, "2026-10-01"), PeriodMonthly.next(mid))
}
