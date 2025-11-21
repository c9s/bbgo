package xpremium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func newTicker(buy, sell, last float64) *types.Ticker {
	return &types.Ticker{Buy: Number(buy), Sell: Number(sell), Last: Number(last)}
}

func newStrategyWithMockExchange(t *testing.T, price float64) (*Strategy, *gomock.Controller, *mocks.MockExchange) {
	ctrl := gomock.NewController(t)
	mockEx := mocks.NewMockExchange(ctrl)

	// NewExchangeSession will call NewStream twice and Name once
	mockEx.EXPECT().NewStream().Times(2).DoAndReturn(func() types.Stream {
		s := types.NewStandardStream()
		return &s
	})
	mockEx.EXPECT().Name().AnyTimes().Return(types.ExchangeName("mock"))
	mockEx.EXPECT().PlatformFeeCurrency().AnyTimes().Return("")

	sess := bbgo.NewExchangeSession("test", mockEx)
	// default spot
	sess.Futures = false
	// set account with balances via UpdateBalances
	acct := &types.Account{}
	acct.UpdateBalances(types.BalanceMap{
		"USDT": {Currency: "USDT", Available: Number(1000)},
		"BTC":  {Currency: "BTC", Available: Number(1)},
	})
	sess.Account = acct

	// QueryTicker stub returning consistent maker price around price
	mockEx.EXPECT().QueryTicker(gomock.Any(), "BTCUSDT").AnyTimes().DoAndReturn(func(ctx context.Context, symbol string) (*types.Ticker, error) {
		p := price
		return newTicker(p-0.5, p+0.5, p), nil
	})

	mkt := Market("BTCUSDT")

	s := &Strategy{
		TradingSession: "test",
		TradingSymbol:  "BTCUSDT",
		PriceType:      types.PriceTypeMaker,
		MaxLeverage:    5,

		logger: log.WithField("strategy", ID),
	}
	// bind runtime session fields
	s.tradingSession = sess
	s.tradingMarket = mkt

	return s, ctrl, mockEx
}

func TestComputeRiskPerUnit_LongShort(t *testing.T) {
	s, ctrl, _ := newStrategyWithMockExchange(t, 100.0)
	defer ctrl.Finish()

	cp := Number(100.0)

	t.Run("long_valid", func(t *testing.T) {
		risk, err := s.computeRiskPerUnit(types.SideTypeBuy, cp, Number(98))
		require.NoError(t, err)
		assert.Equal(t, fixedpoint.NewFromFloat(2), risk)
	})

	t.Run("long_invalid_stop_above", func(t *testing.T) {
		_, err := s.computeRiskPerUnit(types.SideTypeBuy, cp, Number(101))
		assert.Error(t, err)
	})

	t.Run("short_valid", func(t *testing.T) {
		risk, err := s.computeRiskPerUnit(types.SideTypeSell, cp, Number(102))
		require.NoError(t, err)
		assert.Equal(t, fixedpoint.NewFromFloat(2), risk)
	})

	t.Run("short_invalid_stop_below", func(t *testing.T) {
		_, err := s.computeRiskPerUnit(types.SideTypeSell, cp, Number(99))
		assert.Error(t, err)
	})
}

func TestDetermineDesiredQuantity(t *testing.T) {
	s, ctrl, _ := newStrategyWithMockExchange(t, 100.0)
	defer ctrl.Finish()
	cp := Number(100)

	assert.NotNil(t, s.tradingMarket)

	t.Run("quantity_provided", func(t *testing.T) {
		s.Quantity = Number(0.123456)
		dq, rpu, err := s.determineDesiredQuantity(types.SideTypeBuy, cp, Number(95))
		require.NoError(t, err)
		assert.True(t, rpu.IsZero())
		assert.Equal(t, s.Quantity, dq)
	})

	t.Run("risk_based_with_max_loss", func(t *testing.T) {
		s.Quantity = fixedpoint.Zero
		s.MaxLossLimit = Number(20)                                                   // $20
		dq, rpu, err := s.determineDesiredQuantity(types.SideTypeBuy, cp, Number(98)) // rpu=2
		require.NoError(t, err)
		assert.Equal(t, fixedpoint.NewFromFloat(2), rpu)
		assert.Equal(t, fixedpoint.NewFromFloat(10), dq) // 20/2
	})

	t.Run("min_quantity_when_no_stop", func(t *testing.T) {
		dq, rpu, err := s.determineDesiredQuantity(types.SideTypeBuy, cp, fixedpoint.Zero)
		require.NoError(t, err)
		assert.True(t, rpu.IsZero())
		assert.Equal(t, s.tradingMarket.MinQuantity, dq)
	})
}

func TestCapQuantityByMaxLoss(t *testing.T) {
	s, ctrl, _ := newStrategyWithMockExchange(t, 100.0)
	defer ctrl.Finish()
	s.MaxLossLimit = Number(10)

	t.Run("over_limit_should_cap", func(t *testing.T) {
		capped := s.capQuantityByMaxLoss(Number(6), Number(2))
		assert.Equal(t, fixedpoint.NewFromFloat(5), capped)
	})

	t.Run("within_limit_should_keep", func(t *testing.T) {
		same := s.capQuantityByMaxLoss(Number(4), Number(2))
		assert.Equal(t, fixedpoint.NewFromFloat(4), same)
	})
}

func TestComputeMaxQtyByBalance_SpotAndFutures(t *testing.T) {
	s, ctrl, _ := newStrategyWithMockExchange(t, 100.0)
	defer ctrl.Finish()
	ctx := context.Background()

	t.Run("spot_buy", func(t *testing.T) {
		q, err := s.computeMaxQtyByBalance(ctx, types.SideTypeBuy, fixedpoint.NewFromFloat(100))
		require.NoError(t, err)
		assert.Equal(t, fixedpoint.NewFromFloat(10), q)
	})

	t.Run("spot_sell", func(t *testing.T) {
		q, err := s.computeMaxQtyByBalance(ctx, types.SideTypeSell, fixedpoint.NewFromFloat(100))
		require.NoError(t, err)
		assert.Equal(t, fixedpoint.NewFromFloat(1), q)
	})

	t.Run("futures_buy_with_leverage", func(t *testing.T) {
		s.tradingSession.Futures = true
		q, err := s.computeMaxQtyByBalance(ctx, types.SideTypeBuy, fixedpoint.NewFromFloat(100))
		require.NoError(t, err)
		assert.Equal(t, fixedpoint.NewFromFloat(50), q)
	})
}

func TestFinalizeQuantity(t *testing.T) {
	s, ctrl, _ := newStrategyWithMockExchange(t, 100.0)
	defer ctrl.Finish()
	cp := fixedpoint.NewFromFloat(100)

	t.Run("below_minquantity_and_minnotional", func(t *testing.T) {
		q := s.finalizeQuantity(cp, fixedpoint.NewFromFloat(0.0001))
		// BTCUSDT test market has MinNotional=10, so minimal qty = 10/100 = 0.1
		assert.Equal(t, s.tradingMarket.RoundUpByStepSize(fixedpoint.NewFromFloat(0.1)), q)
	})

	t.Run("truncate_large_quantity", func(t *testing.T) {
		q2 := s.finalizeQuantity(cp, fixedpoint.NewFromFloat(1.23456789))
		assert.Equal(t, s.tradingMarket.TruncateQuantity(q2), q2)
	})
}

func TestCalculatePositionSize_Scenarios(t *testing.T) {
	// Price ~100
	s, ctrl, _ := newStrategyWithMockExchange(t, 100.0)
	defer ctrl.Finish()
	ctx := context.Background()

	t.Run("explicit_quantity_no_cap", func(t *testing.T) {
		s.Quantity = fixedpoint.NewFromFloat(10)     // desired 10
		s.MaxLossLimit = fixedpoint.NewFromFloat(15) // $15
		qty, err := s.calculatePositionSize(ctx, types.SideTypeBuy, fixedpoint.NewFromFloat(98))
		require.NoError(t, err)
		expected := s.tradingMarket.TruncateQuantity(fixedpoint.NewFromFloat(10))
		assert.Equal(t, expected, qty)
	})

	t.Run("risk_based_quantity", func(t *testing.T) {
		s.Quantity = fixedpoint.Zero
		s.tradingSession.Futures = false
		qty, err := s.calculatePositionSize(ctx, types.SideTypeBuy, fixedpoint.NewFromFloat(98))
		require.NoError(t, err)
		expected := s.tradingMarket.TruncateQuantity(fixedpoint.NewFromFloat(10))
		assert.Equal(t, expected, qty)
	})

	t.Run("balance_cap_spot", func(t *testing.T) {
		s.MaxLossLimit = fixedpoint.NewFromFloat(1000) // desired 1000/2=500, but balance allows ~1000/99.5
		qty, err := s.calculatePositionSize(ctx, types.SideTypeBuy, fixedpoint.NewFromFloat(98))
		require.NoError(t, err)
		expBal := fixedpoint.NewFromFloat(1000).Div(fixedpoint.NewFromFloat(99.5))
		expBal = s.tradingMarket.TruncateQuantity(expBal)
		assert.Equal(t, expBal, qty)
	})
}
