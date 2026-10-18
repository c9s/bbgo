package bbgo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func TestClosePosition_AvailableBelowMinQuantity(t *testing.T) {
	market := getTestMarket()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockEx := mocks.NewMockExchange(mockCtrl)
	mockEx.EXPECT().Name().Return(types.ExchangeName("test")).AnyTimes()
	mockEx.EXPECT().NewStream().Return(&types.StandardStream{}).Times(2)

	session := NewExchangeSession("test", mockEx)
	assert.NotNil(t, session)

	session.markets[market.Symbol] = market
	session.Account.UpdateBalances(types.BalanceMap{
		"BTC": {
			Currency:  "BTC",
			Available: fixedpoint.MustNewFromString("0.0005"),
		},
	})

	position := types.NewPositionFromMarket(market)
	position.AverageCost = fixedpoint.NewFromFloat(20000.0)
	position.Base = fixedpoint.NewFromFloat(1.0)

	orderExecutor := NewGeneralOrderExecutor(session, "BTCUSDT", "test", "test-01", position)
	err := orderExecutor.ClosePosition(context.Background(), fixedpoint.One)
	assert.NoError(t, err)
}
