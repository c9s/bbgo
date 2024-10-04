package bbgo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pricesolver"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
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
	symbol := "BTCUSDT"
	markets := AllMarkets()

	t.Run("borrow and available", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		mockEx := mocks.NewMockExchange(mockCtrl)
		// for market data stream and user data stream
		mockEx.EXPECT().NewStream().Return(&types.StandardStream{}).Times(2)
		mockEx.EXPECT().QueryTickers(gomock.Any(), []string{symbol}).Return(map[string]types.Ticker{
			"BTCUSDT": Ticker(symbol),
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

		ctx := context.Background()
		priceSolver := pricesolver.NewSimplePriceResolver(markets)

		cal := NewAccountValueCalculator(session, priceSolver, "USDT")
		assert.NotNil(t, cal)

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
		mockEx.EXPECT().QueryTickers(gomock.Any(), []string{symbol}).Return(map[string]types.Ticker{
			symbol: Ticker(symbol),
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

		ctx := context.Background()
		priceSolver := pricesolver.NewSimplePriceResolver(markets)

		cal := NewAccountValueCalculator(session, priceSolver, "USDT")
		assert.NotNil(t, cal)

		netValue, err := cal.NetValue(ctx)
		assert.NoError(t, err)
		assert.Equal(t, "2000", netValue.String()) // 21000-19000
	})
}

func TestNewAccountValueCalculator_MarginLevel(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ticker := newTestTicker()

	mockEx := mocks.NewMockExchange(mockCtrl)
	// for market data stream and user data stream
	mockEx.EXPECT().NewStream().Return(&types.StandardStream{}).Times(2)
	mockEx.EXPECT().QueryTickers(gomock.Any(), []string{"BTCUSDT"}).Return(map[string]types.Ticker{
		"BTCUSDT": ticker,
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

	ctx := context.Background()
	markets := AllMarkets()
	priceSolver := pricesolver.NewSimplePriceResolver(markets)
	err := priceSolver.UpdateFromTickers(ctx, mockEx, "BTCUSDT")
	assert.NoError(t, err)

	cal := NewAccountValueCalculator(session, priceSolver, "USDT")
	assert.NotNil(t, cal)

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
					"USDC": types.Balance{Currency: "USDC", Available: Number(70.0 + 80.0)},
					"USDT": types.Balance{Currency: "USDT", Available: Number(100.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: Number(0.01)},
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
					"USDC": types.Balance{Currency: "USDC", Available: Number(70.0 + 80.0)},
					"USDT": types.Balance{Currency: "USDT", Available: Number(100.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: Number(0.01)},
				},
			},
			wantFiats: types.BalanceMap{
				"USDC": types.Balance{Currency: "USDC", Available: Number(70.0 + 80.0)},
				"USDT": types.Balance{Currency: "USDT", Available: Number(100.0)},
			},
			wantRest: types.BalanceMap{
				"BTC": types.Balance{Currency: "BTC", Available: Number(0.01)},
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
					"USDC": types.Balance{Currency: "USDC", Available: number(70.0 + 80.0)},
					"USDT": types.Balance{Currency: "USDT", Available: number(100.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: number(0.01)},
				},
				prices: types.PriceMap{
					"USDCUSDT": Number(1.0),
					"BTCUSDT":  Number(19000.0),
				},
				quoteCurrency: "USDT",
			},
			wantAccountValue: Number(19000.0*0.01 + 100.0 + 80.0 + 70.0),
		},
		{
			name: "reversed usdt price",
			args: args{
				balances: types.BalanceMap{
					"USDC": types.Balance{Currency: "USDC", Available: Number(70.0 + 80.0)},
					"TWD":  types.Balance{Currency: "TWD", Available: Number(3000.0)},
					"USDT": types.Balance{Currency: "USDT", Available: Number(100.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: Number(0.01)},
				},
				prices: types.PriceMap{
					"USDTTWD":  Number(30.0),
					"USDCUSDT": Number(1.0),
					"BTCUSDT":  Number(19000.0),
				},
				quoteCurrency: "USDT",
			},
			wantAccountValue: Number(19000.0*0.01 + 100.0 + 80.0 + 70.0 + (3000.0 / 30.0)),
		},
		{
			name: "borrow base asset",
			args: args{
				balances: types.BalanceMap{
					"USDT": types.Balance{Currency: "USDT", Available: Number(20000.0*2 + 80.0)},
					"USDC": types.Balance{Currency: "USDC", Available: Number(70.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: Number(0), Borrowed: Number(2.0)},
				},
				prices: types.PriceMap{
					"USDCUSDT": number(1.0),
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
					"USDT": types.Balance{Currency: "USDT", Available: Number(20000.0*2 + 80.0)},
					"USDC": types.Balance{Currency: "USDC", Available: Number(70.0)},
					"ETH":  types.Balance{Currency: "ETH", Available: Number(10.0)},
					"BTC":  types.Balance{Currency: "BTC", Available: Number(0), Borrowed: Number(2.0)},
				},
				prices: types.PriceMap{
					"USDCUSDT": Number(1.0),
					"BTCUSDT":  Number(19000.0),
					"ETHUSDT":  Number(1700.0),
				},
				quoteCurrency: "USDT",
			},
			wantAccountValue: Number(19000.0*-2.0 + 1700.0*10.0 + 20000.0*2 + 80.0 + 70.0),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markets := AllMarkets()
			priceSolver := pricesolver.NewSimplePriceResolver(markets)

			for symbol, price := range tt.args.prices {
				priceSolver.Update(symbol, price)
			}

			assert.InDeltaf(t,
				tt.wantAccountValue.Float64(),
				calculateNetValueInQuote(tt.args.balances, priceSolver, tt.args.quoteCurrency).Float64(),
				0.01,
				"calculateNetValueInQuote(%v, %v, %v)", tt.args.balances, tt.args.prices, tt.args.quoteCurrency)
		})
	}
}
