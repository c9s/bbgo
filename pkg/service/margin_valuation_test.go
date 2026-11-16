package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

// fixedTol: the default fixedpoint build multiplies and divides through float64
// and truncates to 8 decimals, so a result can be one unit of 1e-8 low
const fixedTol = 2e-8

func TestValueInterest(t *testing.T) {
	h10 := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	h9 := h10.Add(-time.Hour)
	now := h10.Add(3 * time.Hour)
	closes := map[int64]fixedpoint.Value{h9.UnixMilli(): Number(600)}

	row := func(asset, rawAsset, typ string) types.MarginInterest {
		return types.MarginInterest{
			GID: 7, Exchange: types.ExchangeBinance, Session: "a", Time: types.Time(h10),
			Asset: asset, RawAsset: rawAsset, InterestType: typ,
			Principal: Number("0.5"), Interest: Number("0.0001"), InterestRate: Number("0.0024"),
		}
	}

	t.Run("priced by the close of the hour before", func(t *testing.T) {
		v := valueInterest(row("BNB", "BNB", "PERIODIC"), pricePair{symbol: "BNBUSDT"}, closes, now)
		require.NotNil(t, v.InterestUSDT)
		assert.Equal(t, "600", v.PriceInUSDT.String())
		assert.InDelta(t, 0.06, v.InterestUSDT.Float64(), 1e-12)
		assert.Equal(t, h9, v.PriceKLineStart.Time().UTC())
		assert.Equal(t, uint64(7), v.InterestGID)
		assert.False(t, v.Converted)
		assert.Equal(t, "BNB", v.BorrowedAsset)
		assert.Equal(t, "0.0001", v.BorrowedInterest.String())
	})

	t.Run("a charge inside the hour uses the same kline", func(t *testing.T) {
		m := row("BNB", "", "ON_BORROW")
		m.Time = types.Time(h10.Add(25 * time.Minute))
		v := valueInterest(m, pricePair{symbol: "BNBUSDT"}, closes, now)
		require.NotNil(t, v.PriceInUSDT)
		assert.Equal(t, h9, v.PriceKLineStart.Time().UTC())
		assert.Equal(t, "BNB", v.BorrowedAsset, "rawAsset not returned: the paid asset")
	})

	t.Run("converted: paid in BNB, borrowed interest estimated", func(t *testing.T) {
		v := valueInterest(row("BNB", "BTC", "PERIODIC_CONVERTED"), pricePair{symbol: "BNBUSDT"}, closes, now)
		assert.True(t, v.Converted)
		assert.Equal(t, "BNB", v.PaidAsset)
		assert.Equal(t, "BTC", v.BorrowedAsset)
		// 0.5 x 0.0024 / 24
		assert.InDelta(t, 0.00005, v.BorrowedInterest.Float64(), fixedTol)
		assert.InDelta(t, 0.06, v.InterestUSDT.Float64(), 1e-12, "valued in the paid asset")
	})

	t.Run("converted without rawAsset: borrowed asset unknown", func(t *testing.T) {
		m := row("BNB", "", "PERIODIC_CONVERTED")
		m.IsolatedSymbol = "BTCUSDT"
		v := valueInterest(m, pricePair{symbol: "BNBUSDT"}, closes, now)
		assert.Equal(t, "", v.BorrowedAsset)
		assert.Nil(t, v.BorrowedInterest)
		assert.NotNil(t, v.InterestUSDT, "still priced in the paid asset")
	})

	t.Run("usdt is 1 without a kline", func(t *testing.T) {
		v := valueInterest(row("USDT", "USDT", "PERIODIC"), pricePair{}, nil, now)
		assert.Equal(t, "1", v.PriceInUSDT.String())
		assert.Equal(t, "0.0001", v.InterestUSDT.String())
		assert.True(t, v.PriceKLineStart.Time().IsZero())
	})

	t.Run("inverse pair", func(t *testing.T) {
		v := valueInterest(row("TRY", "TRY", "PERIODIC"), pricePair{symbol: "USDTTRY", inverse: true},
			map[int64]fixedpoint.Value{h9.UnixMilli(): Number(40)}, now)
		assert.InDelta(t, 0.025, v.PriceInUSDT.Float64(), 1e-12)
	})

	t.Run("no market or no kline: unpriced", func(t *testing.T) {
		v := valueInterest(row("DOGE", "DOGE", "PERIODIC"), pricePair{}, nil, now)
		assert.Nil(t, v.PriceInUSDT)
		assert.Equal(t, "", v.PriceSymbol)

		v = valueInterest(row("BNB", "BNB", "PERIODIC"), pricePair{symbol: "BNBUSDT"}, map[int64]fixedpoint.Value{}, now)
		assert.Nil(t, v.PriceInUSDT)
		assert.Equal(t, "BNBUSDT", v.PriceSymbol)
	})
}

func TestFindPricePair(t *testing.T) {
	markets := types.MarketMap{"BNBUSDT": {}, "USDTTRY": {}}
	assert.Equal(t, pricePair{symbol: "BNBUSDT"}, findPricePair("BNB", markets))
	assert.Equal(t, pricePair{symbol: "USDTTRY", inverse: true}, findPricePair("TRY", markets))
	assert.Equal(t, pricePair{}, findPricePair("DOGE", markets))
	assert.Equal(t, pricePair{}, findPricePair("USDT", markets))
}

// fakeKLineExchange is a margin exchange that serves its markets and the 1h
// klines closeAt has, like the Binance kline API
type fakeKLineExchange struct {
	fakeMarginExchange
	markets types.MarketMap

	// closeAt returns the close of the 1h kline starting at start
	closeAt    func(symbol string, start time.Time) (fixedpoint.Value, bool)
	klineCalls map[string]int
}

func (f *fakeKLineExchange) QueryMarkets(context.Context) (types.MarketMap, error) {
	return f.markets, nil
}

func (f *fakeKLineExchange) QueryKLines(
	_ context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions,
) ([]types.KLine, error) {
	f.klineCalls[symbol]++

	var ks []types.KLine
	for h := options.StartTime.UTC().Truncate(time.Hour); !h.After(*options.EndTime) && len(ks) < 1000; h = h.Add(time.Hour) {
		if h.Before(*options.StartTime) {
			continue
		}
		closePrice, ok := f.closeAt(symbol, h)
		if !ok {
			continue
		}
		ks = append(ks, types.KLine{
			Exchange: types.ExchangeBinance, Symbol: symbol, Interval: interval,
			StartTime: types.Time(h), EndTime: types.Time(h.Add(time.Hour - time.Millisecond)),
			Open: closePrice, High: closePrice, Low: closePrice, Close: closePrice, Closed: true,
		})
	}
	return ks, nil
}

func TestMarginInterestValuationService_Sync(t *testing.T) {
	t.Setenv("USE_MARKETS_CACHE_IN_MEMORY", "true")
	ctx := context.Background()
	margin := newTestMarginService(t)
	s := &MarginInterestValuationService{DB: margin.DB}

	h := time.Now().UTC().Truncate(time.Hour).Add(-5 * time.Hour)
	ethListed := false
	ex := &fakeKLineExchange{
		markets:    types.MarketMap{"BNBUSDT": {}, "ETHUSDT": {}, "USDTTRY": {}},
		klineCalls: map[string]int{},
		closeAt: func(symbol string, start time.Time) (fixedpoint.Value, bool) {
			switch symbol {
			case "BNBUSDT":
				return Number(600), true
			case "USDTTRY":
				return Number(40), true
			case "ETHUSDT":
				return Number(3000), ethListed
			}
			return fixedpoint.Zero, false
		},
	}

	id := uint64(0)
	insert := func(session, asset, rawAsset, typ, interest string) {
		id++
		require.NoError(t, insertMarginInterest(margin.DB, types.MarginInterest{
			Exchange: types.ExchangeBinance, Session: session, TxID: txID(id), Time: types.Time(h),
			Asset: asset, RawAsset: rawAsset, InterestType: typ,
			Principal: Number("0.5"), Interest: Number(interest), InterestRate: Number("0.0024"),
		}))
	}
	insert("a", "USDT", "USDT", "PERIODIC", "0.1")
	insert("a", "BNB", "BTC", "PERIODIC_CONVERTED", "0.0001")
	insert("a", "TRY", "TRY", "PERIODIC", "4")
	insert("a", "DOGE", "DOGE", "PERIODIC", "1")
	insert("a", "ETH", "ETH", "PERIODIC", "0.001")
	insert("b", "BNB", "BNB", "PERIODIC", "0.0001")

	// ETH has no kline yet: its row is stored unpriced and the sync reports it
	require.NoError(t, s.Sync(ctx, ex, "a", h.Add(-time.Hour)))

	query := func(session string) map[string]types.MarginInterestValuation {
		vs, err := s.Query(ctx, QueryMarginInterestValuationOptions{
			Exchange: types.ExchangeBinance, Session: session, Since: h, Until: h.Add(time.Hour),
		})
		require.NoError(t, err)
		out := map[string]types.MarginInterestValuation{}
		for _, v := range vs {
			out[v.PaidAsset] = v
		}
		return out
	}

	vs := query("a")
	require.Len(t, vs, 5)
	assert.InDelta(t, 0.1, vs["USDT"].InterestUSDT.Float64(), 1e-9)
	assert.InDelta(t, 0.06, vs["BNB"].InterestUSDT.Float64(), 1e-9)
	assert.Equal(t, "BTC", vs["BNB"].BorrowedAsset)
	assert.True(t, vs["BNB"].Converted)
	assert.InDelta(t, 0.00005, vs["BNB"].BorrowedInterest.Float64(), fixedTol)
	assert.Equal(t, h.Add(-time.Hour), vs["BNB"].PriceKLineStart.Time().UTC())
	assert.InDelta(t, 0.1, vs["TRY"].InterestUSDT.Float64(), 1e-9, "4 TRY at USDTTRY 40")
	assert.Nil(t, vs["DOGE"].InterestUSDT, "no USDT market")
	assert.Nil(t, vs["ETH"].InterestUSDT, "no kline yet")
	assert.Equal(t, "ETHUSDT", vs["ETH"].PriceSymbol)
	assert.Empty(t, query("b"), "another session is not valued")
	assert.Zero(t, ex.klineCalls["DOGE"]+ex.klineCalls["USDT"])

	// the next sync prices the row once its kline is published, without a second row
	ethListed = true
	require.NoError(t, s.Sync(ctx, ex, "a", h.Add(-time.Hour)))
	vs = query("a")
	require.Len(t, vs, 5)
	require.NotNil(t, vs["ETH"].InterestUSDT)
	assert.InDelta(t, 3.0, vs["ETH"].InterestUSDT.Float64(), 1e-9)

	// priced rows are not valued again
	bnbCalls := ex.klineCalls["BNBUSDT"]
	require.NoError(t, s.Sync(ctx, ex, "a", h.Add(-time.Hour)))
	assert.Equal(t, bnbCalls, ex.klineCalls["BNBUSDT"])
	assert.Len(t, query("a"), 5)
}

func TestMarginInterestValuationService_QueryHourBoundaries(t *testing.T) {
	s := &MarginInterestValuationService{DB: newTestMarginService(t).DB}
	h := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	_, err := s.Query(context.Background(), QueryMarginInterestValuationOptions{Since: h.Add(time.Minute), Until: h.Add(time.Hour)})
	assert.Error(t, err)
}

func TestSpotKLineExchange_HidesMarginSettings(t *testing.T) {
	ex := &fakeKLineExchange{}
	// an isolated margin session would make BacktestService sync the isolated symbol
	assert.Equal(t, "binance_klines", targetKlineTable(spotKLineExchange{ex}))
	_, isMargin := interface{}(spotKLineExchange{ex}).(types.MarginExchange)
	assert.False(t, isMargin)
}

func TestSyncService_SyncMarginHistoryValuesInterests(t *testing.T) {
	t.Setenv("USE_MARKETS_CACHE_IN_MEMORY", "true")
	ctx := context.Background()
	margin := newTestMarginService(t)
	valuations := &MarginInterestValuationService{DB: margin.DB}
	sync := &SyncService{MarginService: margin, MarginInterestValuationService: valuations}

	h := time.Now().UTC().Truncate(time.Hour).Add(-3 * time.Hour)
	ex := &fakeKLineExchange{
		markets:    types.MarketMap{"BNBUSDT": {}},
		klineCalls: map[string]int{},
		closeAt: func(symbol string, start time.Time) (fixedpoint.Value, bool) {
			return Number(600), symbol == "BNBUSDT"
		},
	}
	ex.interests = []types.MarginInterest{{
		Exchange: types.ExchangeBinance, TxID: txID(1), Asset: "BNB", RawAsset: "BTC", InterestType: "PERIODIC_CONVERTED",
		Principal: Number("0.5"), Interest: Number("0.0001"), InterestRate: Number("0.0024"), Time: types.Time(h),
	}}

	require.NoError(t, sync.SyncMarginHistory(ctx, ex, "a", h.Add(-time.Hour), "BNB"))

	vs, err := valuations.Query(ctx, QueryMarginInterestValuationOptions{
		Exchange: types.ExchangeBinance, Session: "a", Since: h, Until: h.Add(time.Hour),
	})
	require.NoError(t, err)
	require.Len(t, vs, 1)
	require.NotNil(t, vs[0].InterestUSDT)
	assert.InDelta(t, 0.06, vs[0].InterestUSDT.Float64(), 1e-9)
	assert.Equal(t, "BTC", vs[0].BorrowedAsset)
}
