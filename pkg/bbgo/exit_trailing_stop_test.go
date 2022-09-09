package bbgo

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

// getTestMarket returns the BTCUSDT market information
// for tests, we always use BTCUSDT
func getTestMarket() types.Market {
	market := types.Market{
		Symbol:          "BTCUSDT",
		PricePrecision:  8,
		VolumePrecision: 8,
		QuoteCurrency:   "USDT",
		BaseCurrency:    "BTC",
		MinNotional:     fixedpoint.MustNewFromString("0.001"),
		MinAmount:       fixedpoint.MustNewFromString("10.0"),
		MinQuantity:     fixedpoint.MustNewFromString("0.001"),
	}
	return market
}

func TestTrailingStop_ShortPosition(t *testing.T) {
	market := getTestMarket()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockEx := mocks.NewMockExchange(mockCtrl)
	mockEx.EXPECT().NewStream().Return(&types.StandardStream{}).Times(2)
	mockEx.EXPECT().SubmitOrder(gomock.Any(), types.SubmitOrder{
		Symbol:           "BTCUSDT",
		Side:             types.SideTypeBuy,
		Type:             types.OrderTypeMarket,
		Market:           market,
		Quantity:         fixedpoint.NewFromFloat(1.0),
		Tag:              "trailingStop",
		MarginSideEffect: types.SideEffectTypeAutoRepay,
	})

	session := NewExchangeSession("test", mockEx)
	assert.NotNil(t, session)

	session.markets[market.Symbol] = market

	position := types.NewPositionFromMarket(market)
	position.AverageCost = fixedpoint.NewFromFloat(20000.0)
	position.Base = fixedpoint.NewFromFloat(-1.0)

	orderExecutor := NewGeneralOrderExecutor(session, "BTCUSDT", "test", "test-01", position)

	activationRatio := fixedpoint.NewFromFloat(0.01)
	callbackRatio := fixedpoint.NewFromFloat(0.01)
	stop := &TrailingStop2{
		Symbol:          "BTCUSDT",
		Interval:        types.Interval1m,
		Side:            types.SideTypeBuy,
		CallbackRate:    callbackRatio,
		ActivationRatio: activationRatio,
	}
	stop.Bind(session, orderExecutor)

	// the same price
	currentPrice := fixedpoint.NewFromFloat(20000.0)
	err := stop.checkStopPrice(currentPrice, position)
	if assert.NoError(t, err) {
		assert.False(t, stop.activated)
	}

	// 20000 - 1% = 19800
	currentPrice = currentPrice.Mul(one.Sub(activationRatio))
	assert.Equal(t, fixedpoint.NewFromFloat(19800.0), currentPrice)

	err = stop.checkStopPrice(currentPrice, position)
	if assert.NoError(t, err) {
		assert.True(t, stop.activated)
		assert.Equal(t, fixedpoint.NewFromFloat(19800.0), stop.latestHigh)
	}

	// 19800 - 1% = 19602
	currentPrice = currentPrice.Mul(one.Sub(callbackRatio))
	assert.Equal(t, fixedpoint.NewFromFloat(19602.0), currentPrice)

	err = stop.checkStopPrice(currentPrice, position)
	if assert.NoError(t, err) {
		assert.Equal(t, fixedpoint.NewFromFloat(19602.0), stop.latestHigh)
		assert.True(t, stop.activated)
	}

	// 19602 + 1% = 19798.02
	currentPrice = currentPrice.Mul(one.Add(callbackRatio))
	assert.Equal(t, fixedpoint.NewFromFloat(19798.02), currentPrice)

	err = stop.checkStopPrice(currentPrice, position)
	if assert.NoError(t, err) {
		assert.Equal(t, fixedpoint.Zero, stop.latestHigh)
		assert.False(t, stop.activated)
	}
}

func TestTrailingStop_LongPosition(t *testing.T) {
	market := getTestMarket()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockEx := mocks.NewMockExchange(mockCtrl)
	mockEx.EXPECT().NewStream().Return(&types.StandardStream{}).Times(2)
	mockEx.EXPECT().SubmitOrder(gomock.Any(), types.SubmitOrder{
		Symbol:           "BTCUSDT",
		Side:             types.SideTypeSell,
		Type:             types.OrderTypeMarket,
		Market:           market,
		Quantity:         fixedpoint.NewFromFloat(1.0),
		Tag:              "trailingStop",
		MarginSideEffect: types.SideEffectTypeAutoRepay,
	})

	session := NewExchangeSession("test", mockEx)
	assert.NotNil(t, session)

	session.markets[market.Symbol] = market

	position := types.NewPositionFromMarket(market)
	position.AverageCost = fixedpoint.NewFromFloat(20000.0)
	position.Base = fixedpoint.NewFromFloat(1.0)

	orderExecutor := NewGeneralOrderExecutor(session, "BTCUSDT", "test", "test-01", position)

	activationRatio := fixedpoint.NewFromFloat(0.01)
	callbackRatio := fixedpoint.NewFromFloat(0.01)
	stop := &TrailingStop2{
		Symbol:          "BTCUSDT",
		Interval:        types.Interval1m,
		Side:            types.SideTypeSell,
		CallbackRate:    callbackRatio,
		ActivationRatio: activationRatio,
	}
	stop.Bind(session, orderExecutor)

	// the same price
	currentPrice := fixedpoint.NewFromFloat(20000.0)
	err := stop.checkStopPrice(currentPrice, position)
	if assert.NoError(t, err) {
		assert.False(t, stop.activated)
	}

	// 20000 + 1% = 20200
	currentPrice = currentPrice.Mul(one.Add(activationRatio))
	assert.Equal(t, fixedpoint.NewFromFloat(20200.0), currentPrice)

	err = stop.checkStopPrice(currentPrice, position)
	if assert.NoError(t, err) {
		assert.True(t, stop.activated)
		assert.Equal(t, fixedpoint.NewFromFloat(20200.0), stop.latestHigh)
	}

	// 20200 + 1% = 20402
	currentPrice = currentPrice.Mul(one.Add(callbackRatio))
	assert.Equal(t, fixedpoint.NewFromFloat(20402.0), currentPrice)

	err = stop.checkStopPrice(currentPrice, position)
	if assert.NoError(t, err) {
		assert.Equal(t, fixedpoint.NewFromFloat(20402.0), stop.latestHigh)
		assert.True(t, stop.activated)
	}

	// 20402 - 1%
	currentPrice = currentPrice.Mul(one.Sub(callbackRatio))
	assert.Equal(t, fixedpoint.NewFromFloat(20197.98), currentPrice)

	err = stop.checkStopPrice(currentPrice, position)
	if assert.NoError(t, err) {
		assert.Equal(t, fixedpoint.Zero, stop.latestHigh)
		assert.False(t, stop.activated)
	}
}
