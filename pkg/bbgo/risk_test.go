package bbgo

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

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

		session := NewExchangeSession("test", mockEx)
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

		session := NewExchangeSession("test", mockEx)
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

	session := NewExchangeSession("test", mockEx)
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

func number(n float64) fixedpoint.Value {
	return fixedpoint.NewFromFloat(n)
}

func Test_aggregateUsdValue(t *testing.T) {
	type args struct {
		balances types.BalanceMap
	}
	tests := []struct {
		name string
		args args
		want fixedpoint.Value
	}{
		{
			name: "mixed",
			args: args{
				balances: types.BalanceMap{
					"USDC": types.Balance{Currency: "USDC", Available: number(70.0)},
					"USDT": types.Balance{Currency: "USDT", Available: number(100.0)},
					"BUSD": types.Balance{Currency: "BUSD", Available: number(80.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: number(0.01)},
				},
			},
			want: number(250.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, aggregateUsdNetValue(tt.args.balances), "aggregateUsdNetValue(%v)", tt.args.balances)
		})
	}
}

func Test_usdFiatBalances(t *testing.T) {
	type args struct {
		balances types.BalanceMap
	}
	tests := []struct {
		name      string
		args      args
		wantFiats types.BalanceMap
		wantRest  types.BalanceMap
	}{
		{
			args: args{
				balances: types.BalanceMap{
					"USDC": types.Balance{Currency: "USDC", Available: number(70.0)},
					"USDT": types.Balance{Currency: "USDT", Available: number(100.0)},
					"BUSD": types.Balance{Currency: "BUSD", Available: number(80.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: number(0.01)},
				},
			},
			wantFiats: types.BalanceMap{
				"USDC": types.Balance{Currency: "USDC", Available: number(70.0)},
				"USDT": types.Balance{Currency: "USDT", Available: number(100.0)},
				"BUSD": types.Balance{Currency: "BUSD", Available: number(80.0)},
			},
			wantRest: types.BalanceMap{
				"BTC": types.Balance{Currency: "BTC", Available: number(0.01)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFiats, gotRest := usdFiatBalances(tt.args.balances)
			assert.Equalf(t, tt.wantFiats, gotFiats, "usdFiatBalances(%v)", tt.args.balances)
			assert.Equalf(t, tt.wantRest, gotRest, "usdFiatBalances(%v)", tt.args.balances)
		})
	}
}

func Test_calculateNetValueInQuote(t *testing.T) {
	type args struct {
		balances      types.BalanceMap
		prices        types.PriceMap
		quoteCurrency string
	}
	tests := []struct {
		name             string
		args             args
		wantAccountValue fixedpoint.Value
	}{
		{
			name: "positive asset",
			args: args{
				balances: types.BalanceMap{
					"USDC": types.Balance{Currency: "USDC", Available: number(70.0)},
					"USDT": types.Balance{Currency: "USDT", Available: number(100.0)},
					"BUSD": types.Balance{Currency: "BUSD", Available: number(80.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: number(0.01)},
				},
				prices: types.PriceMap{
					"USDCUSDT": number(1.0),
					"BUSDUSDT": number(1.0),
					"BTCUSDT":  number(19000.0),
				},
				quoteCurrency: "USDT",
			},
			wantAccountValue: number(19000.0*0.01 + 100.0 + 80.0 + 70.0),
		},
		{
			name: "reversed usdt price",
			args: args{
				balances: types.BalanceMap{
					"USDC": types.Balance{Currency: "USDC", Available: number(70.0)},
					"TWD":  types.Balance{Currency: "TWD", Available: number(3000.0)},
					"USDT": types.Balance{Currency: "USDT", Available: number(100.0)},
					"BUSD": types.Balance{Currency: "BUSD", Available: number(80.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: number(0.01)},
				},
				prices: types.PriceMap{
					"USDTTWD":  number(30.0),
					"USDCUSDT": number(1.0),
					"BUSDUSDT": number(1.0),
					"BTCUSDT":  number(19000.0),
				},
				quoteCurrency: "USDT",
			},
			wantAccountValue: number(19000.0*0.01 + 100.0 + 80.0 + 70.0 + (3000.0 / 30.0)),
		},
		{
			name: "borrow base asset",
			args: args{
				balances: types.BalanceMap{
					"USDT": types.Balance{Currency: "USDT", Available: number(20000.0 * 2)},
					"USDC": types.Balance{Currency: "USDC", Available: number(70.0)},
					"BUSD": types.Balance{Currency: "BUSD", Available: number(80.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: number(0), Borrowed: number(2.0)},
				},
				prices: types.PriceMap{
					"USDCUSDT": number(1.0),
					"BUSDUSDT": number(1.0),
					"BTCUSDT":  number(19000.0),
				},
				quoteCurrency: "USDT",
			},
			wantAccountValue: number(19000.0*-2.0 + 20000.0*2 + 80.0 + 70.0),
		},
		{
			name: "multi base asset",
			args: args{
				balances: types.BalanceMap{
					"USDT": types.Balance{Currency: "USDT", Available: number(20000.0 * 2)},
					"USDC": types.Balance{Currency: "USDC", Available: number(70.0)},
					"BUSD": types.Balance{Currency: "BUSD", Available: number(80.0)},
					"ETH":  types.Balance{Currency: "ETH", Available: number(10.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: number(0), Borrowed: number(2.0)},
				},
				prices: types.PriceMap{
					"USDCUSDT": number(1.0),
					"BUSDUSDT": number(1.0),
					"ETHUSDT":  number(1700.0),
					"BTCUSDT":  number(19000.0),
				},
				quoteCurrency: "USDT",
			},
			wantAccountValue: number(19000.0*-2.0 + 1700.0*10.0 + 20000.0*2 + 80.0 + 70.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.wantAccountValue, calculateNetValueInQuote(tt.args.balances, tt.args.prices, tt.args.quoteCurrency), "calculateNetValueInQuote(%v, %v, %v)", tt.args.balances, tt.args.prices, tt.args.quoteCurrency)
		})
	}
}
