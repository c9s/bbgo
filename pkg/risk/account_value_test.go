package risk

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func newTestTicker() types.Ticker {
	return types.Ticker{
		Time:   time.Now(),
		Volume: fixedpoint.Zero,
		Last:   fixedpoint.NewFromFloat(19000.0),
		Open:   fixedpoint.NewFromFloat(19500.0),
		High:   fixedpoint.NewFromFloat(19900.0),
		Low:    fixedpoint.NewFromFloat(18800.0),
		Buy:    fixedpoint.NewFromFloat(19500.0),
		Sell:   fixedpoint.NewFromFloat(18900.0),
	}
}

func TestAccountValueCalculator_NetValue(t *testing.T) {

	t.Run("borrow and available", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockEx := mocks.NewMockExchange(mockCtrl)
		// for market data stream and user data stream
		mockEx.EXPECT().NewStream().Return(&types.StandardStream{}).Times(2)
		mockEx.EXPECT().QueryTickers(gomock.Any(), []string{"BTCUSDT"}).Return(map[string]types.Ticker{
			"BTCUSDT": newTestTicker(),
		}, nil)

		session := bbgo.NewExchangeSession("test", mockEx)
		session.Account.UpdateBalances(types.BalanceMap{
			"BTC": {
				Currency:  "BTC",
				Available: fixedpoint.NewFromFloat(2.0),
				Locked:    fixedpoint.Zero,
				Borrowed:  fixedpoint.NewFromFloat(1.0),
				Interest:  fixedpoint.Zero,
				NetAsset:  fixedpoint.Zero,
			},
			"USDT": {
				Currency:  "USDT",
				Available: fixedpoint.NewFromFloat(1000.0),
				Locked:    fixedpoint.Zero,
				Borrowed:  fixedpoint.Zero,
				Interest:  fixedpoint.Zero,
				NetAsset:  fixedpoint.Zero,
			},
		})
		assert.NotNil(t, session)

		cal := NewAccountValueCalculator(session, "USDT")
		assert.NotNil(t, cal)

		ctx := context.Background()
		netValue, err := cal.NetValue(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "20000", netValue.String())
	})

	t.Run("borrowed and sold", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockEx := mocks.NewMockExchange(mockCtrl)
		// for market data stream and user data stream
		mockEx.EXPECT().NewStream().Return(&types.StandardStream{}).Times(2)
		mockEx.EXPECT().QueryTickers(gomock.Any(), []string{"BTCUSDT"}).Return(map[string]types.Ticker{
			"BTCUSDT": newTestTicker(),
		}, nil)

		session := bbgo.NewExchangeSession("test", mockEx)
		session.Account.UpdateBalances(types.BalanceMap{
			"BTC": {
				Currency:  "BTC",
				Available: fixedpoint.Zero,
				Locked:    fixedpoint.Zero,
				Borrowed:  fixedpoint.NewFromFloat(1.0),
				Interest:  fixedpoint.Zero,
				NetAsset:  fixedpoint.Zero,
			},
			"USDT": {
				Currency:  "USDT",
				Available: fixedpoint.NewFromFloat(21000.0),
				Locked:    fixedpoint.Zero,
				Borrowed:  fixedpoint.Zero,
				Interest:  fixedpoint.Zero,
				NetAsset:  fixedpoint.Zero,
			},
		})
		assert.NotNil(t, session)

		cal := NewAccountValueCalculator(session, "USDT")
		assert.NotNil(t, cal)

		ctx := context.Background()
		netValue, err := cal.NetValue(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "2000", netValue.String()) // 21000-19000
	})
}

func TestNewAccountValueCalculator_MarginLevel(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockEx := mocks.NewMockExchange(mockCtrl)
	// for market data stream and user data stream
	mockEx.EXPECT().NewStream().Return(&types.StandardStream{}).Times(2)
	mockEx.EXPECT().QueryTickers(gomock.Any(), []string{"BTCUSDT"}).Return(map[string]types.Ticker{
		"BTCUSDT": newTestTicker(),
	}, nil)

	session := bbgo.NewExchangeSession("test", mockEx)
	session.Account.UpdateBalances(types.BalanceMap{
		"BTC": {
			Currency:  "BTC",
			Available: fixedpoint.Zero,
			Locked:    fixedpoint.Zero,
			Borrowed:  fixedpoint.NewFromFloat(1.0),
			Interest:  fixedpoint.NewFromFloat(0.003),
			NetAsset:  fixedpoint.Zero,
		},
		"USDT": {
			Currency:  "USDT",
			Available: fixedpoint.NewFromFloat(21000.0),
			Locked:    fixedpoint.Zero,
			Borrowed:  fixedpoint.Zero,
			Interest:  fixedpoint.Zero,
			NetAsset:  fixedpoint.Zero,
		},
	})
	assert.NotNil(t, session)

	cal := NewAccountValueCalculator(session, "USDT")
	assert.NotNil(t, cal)

	ctx := context.Background()
	marginLevel, err := cal.MarginLevel(ctx)
	assert.NoError(t, err)

	// expected (21000 / 19000 * 1.003)
	assert.Equal(t,
		fixedpoint.NewFromFloat(21000.0).Div(fixedpoint.NewFromFloat(19000.0).Mul(fixedpoint.NewFromFloat(1.003))).FormatString(6),
		marginLevel.FormatString(6))
}
