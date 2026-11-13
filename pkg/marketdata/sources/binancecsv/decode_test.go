package binancecsv

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/marketdata"
	"github.com/c9s/bbgo/pkg/marketdata/archive"
	"github.com/c9s/bbgo/pkg/types"
)

// decodeFixture runs a decoder over a fixture file and returns every event.
func decodeFixture(
	t *testing.T, path string, decoder RecordDecoder, meta RecordMeta,
) []marketdata.Event {
	t.Helper()

	f, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })

	reader := archive.NewReader(f)

	var out []marketdata.Event
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		meta.Columns = reader.Columns()
		meta.LineNo = reader.Line()

		out, err = decoder.Decode(out, record, meta)
		require.NoError(t, err, "line %d", reader.Line())
	}

	return out
}

func tradeMeta(market Market) RecordMeta {
	return RecordMeta{
		Exchange: types.ExchangeBinance,
		Market:   market,
		Symbol:   "BTCUSDT",
		Dataset:  DatasetAggTrades,
	}
}

// TestAggTradeDecoder_FuturesLayout decodes the futures archive: header row,
// millisecond timestamps, lowercase booleans.
func TestAggTradeDecoder_FuturesLayout(t *testing.T) {
	events := decodeFixture(t,
		"testdata/um/BTCUSDT-aggTrades-2026-09-15.csv",
		newAggTradeDecoder(Config{}),
		tradeMeta(MarketUSDMFutures))

	require.Len(t, events, 5)

	first := events[0]
	assert.Equal(t, marketdata.EventTypeTrade, first.Type)
	assert.True(t, first.Flags.Has(marketdata.FlagAggregated))
	assert.Equal(t, uint64(3449899747), first.Key.Seq)
	assert.Equal(t, "2026-09-15T00:00:00.003Z",
		first.Time().Format("2006-01-02T15:04:05.999Z"))

	require.NotNil(t, first.Trade)
	assert.Equal(t, "78153", first.Trade.Price.String())
	assert.Equal(t, "0.001", first.Trade.Quantity.String())
	assert.True(t, first.Trade.IsFutures)

	for i := 1; i < len(events); i++ {
		assert.LessOrEqual(t, events[i-1].Key.Compare(events[i].Key), 0)
	}
}

// TestAggTradeDecoder_IsBuyerMakerMapping is the regression test for a real bug
// in the code this replaces.
//
// csvsource set types.Trade.IsMaker from is_buyer_maker, so every public sell
// trade came out with IsMaker=true. IsMaker means "my order was the maker" — a
// private-trade field with no meaning for a public trade — and any fee model
// keyed off it would have been silently wrong.
func TestAggTradeDecoder_IsBuyerMakerMapping(t *testing.T) {
	events := decodeFixture(t,
		"testdata/um/BTCUSDT-aggTrades-2026-09-15.csv",
		newAggTradeDecoder(Config{}),
		tradeMeta(MarketUSDMFutures))

	// fixture line 1: is_buyer_maker=false, line 2: is_buyer_maker=true
	buyAggressor := events[0].Trade
	sellAggressor := events[1].Trade

	assert.Equal(t, types.SideTypeBuy, buyAggressor.Side,
		"is_buyer_maker=false means the buyer took, so the aggressor side is buy")
	assert.True(t, buyAggressor.IsBuyer)
	assert.False(t, buyAggressor.IsMaker)

	assert.Equal(t, types.SideTypeSell, sellAggressor.Side,
		"is_buyer_maker=true means the buyer rested, so the aggressor side is sell")
	assert.False(t, sellAggressor.IsBuyer)
	assert.False(t, sellAggressor.IsMaker,
		"a public trade has no 'my' side, so IsMaker must never be set from is_buyer_maker")
}

// TestAggTradeDecoder_SpotLayouts covers the headerless spot archives on both
// sides of the undocumented millisecond-to-microsecond switch. The same decoder
// must handle both without configuration.
func TestAggTradeDecoder_SpotLayouts(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantFirst string
		wantID    uint64
	}{
		{
			name:      "2023, milliseconds",
			path:      "testdata/spot/BTCUSDT-aggTrades-2023-11-17.csv",
			wantFirst: "2023-11-17T00:00:00Z",
			wantID:    2759293842,
		},
		{
			name:      "2026, microseconds",
			path:      "testdata/spot/BTCUSDT-aggTrades-2026-09-15.csv",
			wantFirst: "2026-09-15T00:00:00.017025Z",
			wantID:    4063759318,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := decodeFixture(t, tt.path,
				newAggTradeDecoder(Config{}), tradeMeta(MarketSpot))

			require.NotEmpty(t, events)
			assert.Equal(t, tt.wantID, events[0].Key.Seq)
			assert.Equal(t, tt.wantFirst,
				events[0].Time().Format("2006-01-02T15:04:05.999999Z"),
				"the decoder must infer the timestamp unit, not assume it")
			assert.False(t, events[0].Trade.IsFutures)
		})
	}
}

// TestAggTradeDecoder_SpotBooleans documents that the spot archives'
// Python-style True/False needs no special handling: strconv.ParseBool takes it.
func TestAggTradeDecoder_SpotBooleans(t *testing.T) {
	events := decodeFixture(t,
		"testdata/spot/BTCUSDT-aggTrades-2026-09-15.csv",
		newAggTradeDecoder(Config{}), tradeMeta(MarketSpot))

	require.NotEmpty(t, events)
	assert.Equal(t, types.SideTypeSell, events[0].Trade.Side, "fixture has is_buyer_maker=True")
}

func TestTradeDecoder(t *testing.T) {
	meta := tradeMeta(MarketUSDMFutures)
	meta.Dataset = DatasetTrades

	events := decodeFixture(t,
		"testdata/um/BTCUSDT-trades-2026-09-15.csv", newTradeDecoder(Config{}), meta)

	require.Len(t, events, 3)

	first := events[0]
	assert.Equal(t, uint64(8078264332), first.Key.Seq)
	assert.False(t, first.Flags.Has(marketdata.FlagAggregated),
		"raw trades are not aggregated")
	assert.Equal(t, "78.153", first.Trade.QuoteQuantity.String(),
		"the published quote_qty must be preferred over price*quantity")
}

func klineMeta(market Market, interval types.Interval) RecordMeta {
	return RecordMeta{
		Exchange: types.ExchangeBinance,
		Market:   market,
		Symbol:   "BTCUSDT",
		Dataset:  DatasetKLines,
		Interval: interval,
	}
}

// TestKLineDecoder_KeyIsCloseTime pins the most confusable part of the model:
// a kline event happens at its close, so that is what orders it.
func TestKLineDecoder_KeyIsCloseTime(t *testing.T) {
	events := decodeFixture(t,
		"testdata/um/BTCUSDT-1h-2026-09-15.csv",
		newKLineDecoder(Config{}), klineMeta(MarketUSDMFutures, types.Interval1h))

	require.Len(t, events, 3)

	first := events[0]
	require.NotNil(t, first.KLine)

	assert.Equal(t, "2026-09-15T00:00:00Z",
		first.KLine.StartTime.Time().UTC().Format("2006-01-02T15:04:05Z"))
	assert.Equal(t, "2026-09-15T00:59:59.999Z",
		first.KLine.EndTime.Time().UTC().Format("2006-01-02T15:04:05.999Z"),
		"Binance ends a candle one millisecond before the next one opens")

	assert.Equal(t, first.KLine.EndTime.Time().UnixNano(), first.Key.TimeNs,
		"a kline is ordered by its close time, not its open time")
	assert.Equal(t, marketdata.KLineRank(types.Interval1h), first.Key.Rank)
	assert.True(t, first.KLine.Closed)
}

// TestKLineDecoder_SpotMicroseconds is the kline half of the precision problem:
// the spot archives moved to microseconds, and the old decoder's
// ParseFloat-then-treat-as-milliseconds produced dates around the year 58000.
func TestKLineDecoder_SpotMicroseconds(t *testing.T) {
	events := decodeFixture(t,
		"testdata/spot/BTCUSDT-1m-2026-09-15.csv",
		newKLineDecoder(Config{}), klineMeta(MarketSpot, types.Interval1m))

	require.Len(t, events, 4)

	first := events[0].KLine
	assert.Equal(t, "2026-09-15T00:00:00Z",
		first.StartTime.Time().UTC().Format("2006-01-02T15:04:05Z"))
	assert.Equal(t, "2026-09-15T00:00:59.999999Z",
		first.EndTime.Time().UTC().Format("2006-01-02T15:04:05.999999Z"))

	assert.Equal(t, "78189.2", first.Open.String())
	assert.Equal(t, "78199.66", first.Close.String())
	assert.Equal(t, uint64(1076), first.NumberOfTrades)
}

// TestKLineDecoder_ScientificNotation keeps the one genuinely non-obvious piece
// of the code being replaced: some older archives write the timestamp in
// scientific notation.
func TestKLineDecoder_ScientificNotation(t *testing.T) {
	events := decodeFixture(t, writeTemp(t, "1.70027E+12,100,110,90,105,10\n"),
		newKLineDecoder(Config{}), klineMeta(MarketSpot, types.Interval1h))

	require.Len(t, events, 1)
	assert.Equal(t, "2023-11-18T01:13:20Z",
		events[0].KLine.StartTime.Time().UTC().Format("2006-01-02T15:04:05Z"))
}

func TestBookTickerDecoder(t *testing.T) {
	meta := tradeMeta(MarketUSDMFutures)
	meta.Dataset = DatasetBookTicker

	events := decodeFixture(t,
		"testdata/um/BTCUSDT-bookTicker-2024-01-15.csv", newBookTickerDecoder(Config{}), meta)

	require.Len(t, events, 3)

	first := events[0]
	assert.Equal(t, marketdata.EventTypeBookTicker, first.Type)
	require.NotNil(t, first.BookTicker)

	assert.Equal(t, "41734.9", first.BookTicker.Buy.String())
	assert.Equal(t, "41735", first.BookTicker.Sell.String())
	assert.Equal(t, int64(3831603310899), first.BookTicker.UpdateID)

	// transaction_time 1705276800014, event_time 1705276800018
	assert.Equal(t, int64(1705276800014),
		first.BookTicker.TransactionTime.Time().UnixMilli())
	assert.Equal(t, int64(1705276800018),
		first.BookTicker.EventTime.Time().UnixMilli())
	assert.Equal(t, first.BookTicker.TransactionTime.Time().UnixNano(), first.Key.TimeNs,
		"ordering must use the matching-engine time, not the publish time")
}

func TestBookTickerDecoder_SynthesizedBook(t *testing.T) {
	meta := tradeMeta(MarketUSDMFutures)
	meta.Dataset = DatasetBookTicker

	events := decodeFixture(t,
		"testdata/um/BTCUSDT-bookTicker-2024-01-15.csv",
		newBookTickerDecoder(Config{SynthesizeBookFromBookTicker: true}), meta)

	require.Len(t, events, 6, "each record must yield a ticker and a snapshot")

	snapshot := events[1]
	assert.Equal(t, marketdata.EventTypeBookSnapshot, snapshot.Type)
	assert.True(t, snapshot.Flags.Has(marketdata.FlagSynthetic),
		"a book built from L1 must be labelled synthetic")
	assert.True(t, snapshot.Flags.Has(marketdata.FlagPartialDepth))

	require.NotNil(t, snapshot.Book)
	require.Len(t, snapshot.Book.Bids, 1)
	require.Len(t, snapshot.Book.Asks, 1)
	assert.Equal(t, "41734.9", snapshot.Book.Bids[0].Price.String())
}

// TestBookDepthDecoder pins that bookDepth is not a book.
func TestBookDepthDecoder(t *testing.T) {
	meta := tradeMeta(MarketUSDMFutures)
	meta.Dataset = DatasetBookDepth

	events := decodeFixture(t,
		"testdata/um/BTCUSDT-bookDepth-2026-09-15.csv", newBookDepthDecoder(Config{}), meta)

	require.Len(t, events, 3)

	first := events[0]
	assert.Equal(t, marketdata.EventTypeDepthBand, first.Type,
		"percentage-band notional must not be presented as an order book")
	assert.Nil(t, first.Book)

	require.NotNil(t, first.DepthBand)
	assert.Equal(t, "-5", first.DepthBand.Percentage.String())
	assert.Equal(t, "9258.555", first.DepthBand.Depth.String())

	// the dataset carries a formatted timestamp, not an epoch, and Binance
	// publishes it in UTC
	assert.Equal(t, "2026-09-15T00:00:06Z", first.Time().Format("2006-01-02T15:04:05Z"))
}

func TestMetricsDecoder(t *testing.T) {
	meta := tradeMeta(MarketUSDMFutures)
	meta.Dataset = DatasetMetrics

	events := decodeFixture(t,
		"testdata/um/BTCUSDT-metrics-2026-09-15.csv", newMetricsDecoder(Config{}), meta)

	require.Len(t, events, 2)

	first := events[0]
	assert.Equal(t, marketdata.EventTypeMetrics, first.Type)
	require.NotNil(t, first.Metrics)

	assert.Equal(t, "103513.561",
		first.Metrics.Values[marketdata.MetricSumOpenInterest].String())
	assert.Equal(t, "1.17124703",
		first.Metrics.Values[marketdata.MetricCountLongShortRatio].String())
	assert.Equal(t, "2026-09-15T00:00:00Z", first.Time().Format("2006-01-02T15:04:05Z"))
}

// TestDecoders_RejectShortRecords checks that a truncated record fails loudly
// rather than decoding into a zero-valued event.
func TestDecoders_RejectShortRecords(t *testing.T) {
	tests := []struct {
		name    string
		decoder RecordDecoder
	}{
		{"aggTrades", newAggTradeDecoder(Config{})},
		{"trades", newTradeDecoder(Config{})},
		{"bookTicker", newBookTickerDecoder(Config{})},
		{"bookDepth", newBookDepthDecoder(Config{})},
		{"metrics", newMetricsDecoder(Config{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.decoder.Decode(nil, []string{"1"}, tradeMeta(MarketUSDMFutures))
			assert.Error(t, err)
		})
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()

	path := t.TempDir() + "/fixture.csv"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}
