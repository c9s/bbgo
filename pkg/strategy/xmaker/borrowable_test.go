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
)

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
