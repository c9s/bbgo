package xmaker

import (
	"context"
	"reflect"
	"testing"
	"unsafe"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

// newSyntheticHedgeTestSession builds a minimal session for the borrowable-check tests: margin
// flag and a real *types.Account so balance fallbacks can be exercised via UpdateBalances.
func newSyntheticHedgeTestSession(name string, margin bool) *bbgo.ExchangeSession {
	return &bbgo.ExchangeSession{
		ExchangeSessionConfig: bbgo.ExchangeSessionConfig{Name: name, Margin: margin},
		Account:               types.NewAccount(),
	}
}

// mockMarginBorrowRepayService is a stub implementation of
// types.MarginBorrowRepayService that returns preconfigured max borrowable
// amounts, letting us populate a MarginInfoUpdater without hitting an exchange.
type mockMarginBorrowRepayService struct {
	maxBorrowable map[string]fixedpoint.Value
}

func (m *mockMarginBorrowRepayService) RepayMarginAsset(
	_ context.Context, _ string, _ fixedpoint.Value,
) error {
	return nil
}

func (m *mockMarginBorrowRepayService) BorrowMarginAsset(
	_ context.Context, _ string, _ fixedpoint.Value,
) error {
	return nil
}

func (m *mockMarginBorrowRepayService) QueryMarginAssetMaxBorrowable(
	_ context.Context, asset string,
) (fixedpoint.Value, error) {
	return m.maxBorrowable[asset], nil
}

// setMarginInfoUpdater assigns the unexported marginInfoUpdater field on the
// session via unsafe, since bbgo exposes only a getter.
func setMarginInfoUpdater(session *bbgo.ExchangeSession, updater *bbgo.MarginInfoUpdater) {
	field := reflect.ValueOf(session).Elem().FieldByName("marginInfoUpdater")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(updater))
}

// newBorrowableUpdater builds a MarginInfoUpdater whose GetMaxBorrowable reports
// the given per-asset amounts (assets present in the map are "found").
func newBorrowableUpdater(values map[string]fixedpoint.Value) *bbgo.MarginInfoUpdater {
	svc := &mockMarginBorrowRepayService{maxBorrowable: values}
	config := bbgo.MarginInfoUpdaterConfig{
		BatchSize: len(values),
	}
	updater := bbgo.NewMarginInfoUpdater(svc, config)

	assets := make([]string, 0, len(values))
	for asset := range values {
		assets = append(assets, asset)
	}
	updater.AddBorrowableAssets(assets...)
	updater.Update(context.Background())
	return updater
}

func Test_getBorrowableAssetResult(t *testing.T) {
	market := Market("BTCUSDT") // base: BTC, quote: USDT
	logger := logrus.New()

	t.Run("nil updater disables the check", func(t *testing.T) {
		// a non-margin session has no updater -> returns nil
		session := &bbgo.ExchangeSession{}
		assert.Nil(t, getBorrowableAssetResult(session, market, logger))
	})

	t.Run("missing asset disables the check", func(t *testing.T) {
		// only the base currency is tracked, the quote lookup is "not found"
		updater := newBorrowableUpdater(map[string]fixedpoint.Value{
			"BTC": Number(1),
		})
		session := &bbgo.ExchangeSession{}
		setMarginInfoUpdater(session, updater)

		assert.Nil(t, getBorrowableAssetResult(session, market, logger))
	})

	t.Run("both assets missing disables the check", func(t *testing.T) {
		updater := newBorrowableUpdater(map[string]fixedpoint.Value{
			"ETH": Number(1),
		})
		session := &bbgo.ExchangeSession{}
		setMarginInfoUpdater(session, updater)

		assert.Nil(t, getBorrowableAssetResult(session, market, logger))
	})

	testCases := []struct {
		name            string
		base            fixedpoint.Value
		quote           fixedpoint.Value
		wantBaseBorrow  bool
		wantQuoteBorrow bool
	}{
		{
			name:            "both borrowable",
			base:            Number(1.5),
			quote:           Number(1000),
			wantBaseBorrow:  true,
			wantQuoteBorrow: true,
		},
		{
			name:            "only base borrowable",
			base:            Number(1.5),
			quote:           fixedpoint.Zero,
			wantBaseBorrow:  true,
			wantQuoteBorrow: false,
		},
		{
			name:            "only quote borrowable",
			base:            fixedpoint.Zero,
			quote:           Number(1000),
			wantBaseBorrow:  false,
			wantQuoteBorrow: true,
		},
		{
			name:            "neither borrowable",
			base:            fixedpoint.Zero,
			quote:           fixedpoint.Zero,
			wantBaseBorrow:  false,
			wantQuoteBorrow: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			updater := newBorrowableUpdater(map[string]fixedpoint.Value{
				market.BaseCurrency:  tc.base,
				market.QuoteCurrency: tc.quote,
			})
			session := &bbgo.ExchangeSession{}
			setMarginInfoUpdater(session, updater)

			result := getBorrowableAssetResult(session, market, logger)
			if assert.NotNil(t, result) {
				assert.Equal(t, tc.wantBaseBorrow, result.BaseBorrowable, "base borrowable")
				assert.Equal(t, tc.wantQuoteBorrow, result.QuoteBorrowable, "quote borrowable")
			}
		})
	}
}

func Test_hedgeMarketLegAvailability(t *testing.T) {
	market := Market("BTCUSDT") // base: BTC, quote: USDT
	logger := logrus.New()

	t.Run("non-margin session is always available", func(t *testing.T) {
		hm := &HedgeMarket{market: market, session: newSyntheticHedgeTestSession("binance", false)}
		baseOK, quoteOK := hedgeMarketLegAvailability(hm, logger)
		assert.True(t, baseOK)
		assert.True(t, quoteOK)
	})

	t.Run("margin session without updater fails open", func(t *testing.T) {
		hm := &HedgeMarket{market: market, session: newSyntheticHedgeTestSession("binance", true)}
		baseOK, quoteOK := hedgeMarketLegAvailability(hm, logger)
		assert.True(t, baseOK)
		assert.True(t, quoteOK)
	})

	t.Run("margin session with missing asset data fails open", func(t *testing.T) {
		session := newSyntheticHedgeTestSession("binance", true)
		updater := newBorrowableUpdater(map[string]fixedpoint.Value{
			"ETH": Number(1),
		})
		setMarginInfoUpdater(session, updater)

		hm := &HedgeMarket{market: market, session: session}
		baseOK, quoteOK := hedgeMarketLegAvailability(hm, logger)
		assert.True(t, baseOK)
		assert.True(t, quoteOK)
	})

	testCases := []struct {
		name           string
		base, quote    fixedpoint.Value
		baseBalance    fixedpoint.Value
		quoteBalance   fixedpoint.Value
		wantBaseAvail  bool
		wantQuoteAvail bool
	}{
		{
			name:           "both borrowable",
			base:           Number(1.5),
			quote:          Number(1000),
			wantBaseAvail:  true,
			wantQuoteAvail: true,
		},
		{
			name:           "neither borrowable, zero balances",
			base:           fixedpoint.Zero,
			quote:          fixedpoint.Zero,
			wantBaseAvail:  false,
			wantQuoteAvail: false,
		},
		{
			name:           "base not borrowable but balance available",
			base:           fixedpoint.Zero,
			quote:          fixedpoint.Zero,
			baseBalance:    Number(2),
			wantBaseAvail:  true,
			wantQuoteAvail: false,
		},
		{
			name:           "quote not borrowable but balance available",
			base:           fixedpoint.Zero,
			quote:          fixedpoint.Zero,
			quoteBalance:   Number(500),
			wantBaseAvail:  false,
			wantQuoteAvail: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			session := newSyntheticHedgeTestSession("binance", true)
			updater := newBorrowableUpdater(map[string]fixedpoint.Value{
				market.BaseCurrency:  tc.base,
				market.QuoteCurrency: tc.quote,
			})
			setMarginInfoUpdater(session, updater)
			session.Account.UpdateBalances(types.BalanceMap{
				market.BaseCurrency:  types.NewBalance(market.BaseCurrency, tc.baseBalance),
				market.QuoteCurrency: types.NewBalance(market.QuoteCurrency, tc.quoteBalance),
			})

			hm := &HedgeMarket{market: market, session: session}
			baseOK, quoteOK := hedgeMarketLegAvailability(hm, logger)
			assert.Equal(t, tc.wantBaseAvail, baseOK, "base available")
			assert.Equal(t, tc.wantQuoteAvail, quoteOK, "quote available")
		})
	}
}

func Test_anyHedgeMarketLegAvailable(t *testing.T) {
	market := Market("BTCUSDT")
	logger := logrus.New()

	newMarket := func(name string, base, quote fixedpoint.Value) *HedgeMarket {
		session := newSyntheticHedgeTestSession(name, true)
		updater := newBorrowableUpdater(map[string]fixedpoint.Value{
			market.BaseCurrency:  base,
			market.QuoteCurrency: quote,
		})
		setMarginInfoUpdater(session, updater)
		return &HedgeMarket{market: market, session: session}
	}

	t.Run("empty slice is never available", func(t *testing.T) {
		anyBaseOK, anyQuoteOK := anyHedgeMarketLegAvailable(nil, logger)
		assert.False(t, anyBaseOK)
		assert.False(t, anyQuoteOK)
	})

	t.Run("single element degenerates to hedgeMarketLegAvailability", func(t *testing.T) {
		hm := newMarket("binance", Number(1), fixedpoint.Zero)
		wantBaseOK, wantQuoteOK := hedgeMarketLegAvailability(hm, logger)
		anyBaseOK, anyQuoteOK := anyHedgeMarketLegAvailable([]*HedgeMarket{hm}, logger)
		assert.Equal(t, wantBaseOK, anyBaseOK)
		assert.Equal(t, wantQuoteOK, anyQuoteOK)
	})

	t.Run("at least one market covers the leg", func(t *testing.T) {
		hm1 := newMarket("binance", fixedpoint.Zero, fixedpoint.Zero)
		hm2 := newMarket("okex", Number(1), fixedpoint.Zero)
		anyBaseOK, anyQuoteOK := anyHedgeMarketLegAvailable([]*HedgeMarket{hm1, hm2}, logger)
		assert.True(t, anyBaseOK)
		assert.False(t, anyQuoteOK)
	})
}

func Test_Strategy_syntheticHedgeBorrowableCheck(t *testing.T) {
	sourceMarket := Market("BTCUSDT")
	fiatMarket := Market("USDTTWD")
	logger := logrus.New()

	newSourceMarket := func(name string, base, quote fixedpoint.Value) *HedgeMarket {
		session := newSyntheticHedgeTestSession(name, true)
		updater := newBorrowableUpdater(map[string]fixedpoint.Value{
			sourceMarket.BaseCurrency:  base,
			sourceMarket.QuoteCurrency: quote,
		})
		setMarginInfoUpdater(session, updater)
		return &HedgeMarket{market: sourceMarket, session: session}
	}

	newFiatMarket := func(base, quote fixedpoint.Value) *HedgeMarket {
		session := newSyntheticHedgeTestSession("max", true)
		updater := newBorrowableUpdater(map[string]fixedpoint.Value{
			fiatMarket.BaseCurrency:  base,
			fiatMarket.QuoteCurrency: quote,
		})
		setMarginInfoUpdater(session, updater)
		return &HedgeMarket{market: fiatMarket, session: session}
	}

	t.Run("already disabled short-circuits", func(t *testing.T) {
		s := &Strategy{logger: logger, Symbol: "BTCTWD", SyntheticHedge: &SyntheticHedge{}}
		disableBid, disableAsk := s.syntheticHedgeBorrowableCheck(true, true)
		assert.True(t, disableBid)
		assert.True(t, disableAsk)
	})

	t.Run("non-split forward: everything available", func(t *testing.T) {
		s := &Strategy{
			logger: logger, Symbol: "BTCTWD",
			SyntheticHedge: &SyntheticHedge{
				sourceMarket: newSourceMarket("binance", Number(1), Number(1000)),
				fiatMarket:   newFiatMarket(Number(1000), Number(1000)),
				forward:      true,
			},
		}
		disableBid, disableAsk := s.syntheticHedgeBorrowableCheck(false, false)
		assert.False(t, disableBid)
		assert.False(t, disableAsk)
	})

	t.Run("non-split forward: source base unavailable disables bid", func(t *testing.T) {
		s := &Strategy{
			logger: logger, Symbol: "BTCTWD",
			SyntheticHedge: &SyntheticHedge{
				sourceMarket: newSourceMarket("binance", fixedpoint.Zero, Number(1000)),
				fiatMarket:   newFiatMarket(Number(1000), Number(1000)),
				forward:      true,
			},
		}
		disableBid, disableAsk := s.syntheticHedgeBorrowableCheck(false, false)
		assert.True(t, disableBid)
		assert.False(t, disableAsk)
	})

	t.Run("non-split forward: fiat quote unavailable disables ask", func(t *testing.T) {
		s := &Strategy{
			logger: logger, Symbol: "BTCTWD",
			SyntheticHedge: &SyntheticHedge{
				sourceMarket: newSourceMarket("binance", Number(1), Number(1000)),
				fiatMarket:   newFiatMarket(Number(1000), fixedpoint.Zero),
				forward:      true,
			},
		}
		disableBid, disableAsk := s.syntheticHedgeBorrowableCheck(false, false)
		assert.False(t, disableBid)
		assert.True(t, disableAsk)
	})

	t.Run("non-split reverse: fiat quote unavailable disables bid (flip)", func(t *testing.T) {
		s := &Strategy{
			logger: logger, Symbol: "BTCUSDC",
			SyntheticHedge: &SyntheticHedge{
				sourceMarket: newSourceMarket("binance", Number(1), Number(1000)),
				fiatMarket:   newFiatMarket(Number(1000), fixedpoint.Zero),
				forward:      false,
			},
		}
		disableBid, disableAsk := s.syntheticHedgeBorrowableCheck(false, false)
		assert.True(t, disableBid)
		assert.False(t, disableAsk)
	})

	t.Run("non-split reverse: fiat base unavailable disables ask (flip)", func(t *testing.T) {
		s := &Strategy{
			logger: logger, Symbol: "BTCUSDC",
			SyntheticHedge: &SyntheticHedge{
				sourceMarket: newSourceMarket("binance", Number(1), Number(1000)),
				fiatMarket:   newFiatMarket(fixedpoint.Zero, Number(1000)),
				forward:      false,
			},
		}
		disableBid, disableAsk := s.syntheticHedgeBorrowableCheck(false, false)
		assert.False(t, disableBid)
		assert.True(t, disableAsk)
	})

	t.Run("split-source forward: at-least-one semantics on source leg", func(t *testing.T) {
		split := &SplitHedge{
			Enabled: true,
			hedgeMarketInstances: map[string]*HedgeMarket{
				"binance": newSourceMarket("binance", fixedpoint.Zero, fixedpoint.Zero),
				"okex":    newSourceMarket("okex", Number(1), fixedpoint.Zero),
			},
		}
		s := &Strategy{
			logger: logger, Symbol: "BTCTWD",
			SyntheticHedge: &SyntheticHedge{
				SplitSource: split,
				fiatMarket:  newFiatMarket(Number(1000), fixedpoint.Zero),
				forward:     true,
			},
		}
		disableBid, disableAsk := s.syntheticHedgeBorrowableCheck(false, false)
		// one split market has base available -> bid still allowed
		assert.False(t, disableBid)
		// no split market has quote available -> ask disabled
		assert.True(t, disableAsk)
	})

	t.Run("split-source reverse: fiat-leg flip combined with split aggregation", func(t *testing.T) {
		split := &SplitHedge{
			Enabled: true,
			hedgeMarketInstances: map[string]*HedgeMarket{
				"binance": newSourceMarket("binance", Number(1), fixedpoint.Zero),
				"okex":    newSourceMarket("okex", Number(1), fixedpoint.Zero),
			},
		}
		s := &Strategy{
			logger: logger, Symbol: "BTCUSDC",
			SyntheticHedge: &SyntheticHedge{
				SplitSource: split,
				fiatMarket:  newFiatMarket(fixedpoint.Zero, Number(1000)),
				forward:     false,
			},
		}
		disableBid, disableAsk := s.syntheticHedgeBorrowableCheck(false, false)
		// reverse: bid needs fiat quote, which is available -> bid allowed
		assert.False(t, disableBid)
		// reverse: ask needs fiat base, which is unavailable -> ask disabled
		assert.True(t, disableAsk)
	})
}
