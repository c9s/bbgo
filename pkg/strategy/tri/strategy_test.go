//go:build !dnum

package tri

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var markets = make(types.MarketMap)

func init() {
	if err := util.ReadJsonFile("../../../testdata/binance-markets.json", &markets); err != nil {
		panic(err)
	}
}

func loadMarket(symbol string) types.Market {
	if market, ok := markets[symbol]; ok {
		return market
	}

	panic(fmt.Errorf("market %s not found", symbol))
}

func newArbMarket(symbol, base, quote string, askPrice, askVolume, bidPrice, bidVolume float64) *ArbMarket {
	market := loadMarket(symbol)
	return &ArbMarket{
		Symbol:        symbol,
		BaseCurrency:  base,
		QuoteCurrency: quote,
		market:        market,
		book:          nil,
		bestBid: types.PriceVolume{
			Price:  fixedpoint.NewFromFloat(bidPrice),
			Volume: fixedpoint.NewFromFloat(bidVolume),
		},
		bestAsk: types.PriceVolume{
			Price:  fixedpoint.NewFromFloat(askPrice),
			Volume: fixedpoint.NewFromFloat(askVolume),
		},
		buyRate:               1.0 / askPrice,
		sellRate:              bidPrice,
		truncateBaseQuantity:  createBaseQuantityTruncator(market),
		truncateQuoteQuantity: createPricePrecisionBasedQuoteQuantityTruncator(market),
	}

}

func TestPath_calculateBackwardRatio(t *testing.T) {
	// BTCUSDT 22800.0 22700.0
	// ETHBTC  0.074, 0.073
	// ETHUSDT 1630.0 1620.0

	// sell BTCUSDT @ 22700 ( 0.1 BTC => 2270 USDT)
	// buy ETHUSDT @ 1630 ( 2270 USDT => 1.3926380368 ETH)
	// sell ETHBTC @ 0.073  (1.3926380368 ETH => 0.1016625767 BTC)
	marketA := newArbMarket("BTCUSDT", "BTC", "USDT", 22800.0, 1.0, 22700.0, 1.0)
	marketB := newArbMarket("ETHBTC", "ETH", "BTC", 0.074, 2.0, 0.073, 2.0)
	marketC := newArbMarket("ETHUSDT", "ETH", "USDT", 1630.0, 2.0, 1620.0, 2.0)
	path := &Path{
		marketA: marketA,
		marketB: marketB,
		marketC: marketC,
		dirA:    -1,
		dirB:    -1,
		dirC:    1,
	}
	ratio := calculateForwardRatio(path)
	assert.Equal(t, 0.9601706970128022, ratio)

	ratio = calculateBackwardRate(path)
	assert.Equal(t, 1.0166257668711656, ratio)
}

func TestPath_CalculateForwardRatio(t *testing.T) {
	// BTCUSDT 22800.0 22700.0
	// ETHBTC  0.070, 0.069
	// ETHUSDT 1630.0 1620.0

	// buy BTCUSDT @ 22800 ( 2280 usdt => 0.1 BTC)
	// buy ETHBTC @ 0.070  ( 0.1 BTC => 1.4285714286 ETH)
	// sell ETHUSDT @ 1620 ( 1.4285714286 ETH => 2,314.285714332 USDT)
	marketA := newArbMarket("BTCUSDT", "BTC", "USDT", 22800.0, 1.0, 22700.0, 1.0)
	marketB := newArbMarket("ETHBTC", "ETH", "BTC", 0.070, 2.0, 0.069, 2.0)
	marketC := newArbMarket("ETHUSDT", "ETH", "USDT", 1630.0, 2.0, 1620.0, 2.0)
	path := &Path{
		marketA: marketA,
		marketB: marketB,
		marketC: marketC,
		dirA:    -1,
		dirB:    -1,
		dirC:    1,
	}
	ratio := calculateForwardRatio(path)
	assert.Equal(t, 1.015037593984962, ratio)

	ratio = calculateBackwardRate(path)
	assert.Equal(t, 0.9609202453987732, ratio)
}

func TestPath_newForwardOrders(t *testing.T) {
	// BTCUSDT 22800.0 22700.0
	// ETHBTC  0.070, 0.069
	// ETHUSDT 1630.0 1620.0

	// buy BTCUSDT @ 22800 ( 2280 usdt => 0.1 BTC)
	// buy ETHBTC @ 0.070  ( 0.1 BTC => 1.4285714286 ETH)
	// sell ETHUSDT @ 1620 ( 1.4285714286 ETH => 2,314.285714332 USDT)
	marketA := newArbMarket("BTCUSDT", "BTC", "USDT", 22800.0, 1.0, 22700.0, 1.0)
	marketB := newArbMarket("ETHBTC", "ETH", "BTC", 0.070, 2.0, 0.069, 2.0)
	marketC := newArbMarket("ETHUSDT", "ETH", "USDT", 1630.0, 2.0, 1620.0, 2.0)
	path := &Path{
		marketA: marketA,
		marketB: marketB,
		marketC: marketC,
		dirA:    -1,
		dirB:    -1,
		dirC:    1,
	}
	orders := path.newOrders(types.BalanceMap{
		"USDT": {
			Currency:  "USDT",
			Available: fixedpoint.NewFromFloat(2280.0),
			Locked:    fixedpoint.Zero,
			Borrowed:  fixedpoint.Zero,
			Interest:  fixedpoint.Zero,
			NetAsset:  fixedpoint.Zero,
		},
	}, 1)
	for i, order := range orders {
		t.Logf("order #%d: %+v", i, order.String())
	}

	assert.InDelta(t, 2314.17, orders[2].Price.Mul(orders[2].Quantity).Float64(), 0.01)
}

func TestPath_newForwardOrdersWithAdjustRate(t *testing.T) {
	// BTCUSDT 22800.0 22700.0
	// ETHBTC  0.070, 0.069
	// ETHUSDT 1630.0 1620.0

	// buy BTCUSDT @ 22800 (2280 usdt => 0.1 BTC)
	// buy ETHBTC @ 0.070  (0.1 BTC => 1.4285714286 ETH)

	// APPLY ADJUST RATE B: 0.7 = 1 ETH / 1.4285714286 ETH
	// buy BTCUSDT @ 22800 ( 1596 usdt => 0.07 BTC)
	// buy ETHBTC @ 0.070  (0.07 BTC => 1 ETH)
	// sell ETHUSDT @ 1620.0 (1 ETH => 1620 USDT)

	// APPLY ADJUST RATE C: 0.5 = 0.5 ETH / 1 ETH
	// buy BTCUSDT @ 22800 ( 798 usdt => 0.0035 BTC)
	// buy ETHBTC @ 0.070  (0.035 BTC => 0.5 ETH)
	// sell ETHUSDT @ 1620.0 (0.5 ETH => 1620 USDT)

	// sell ETHUSDT @ 1620 ( 1.4285714286 ETH => 2,314.285714332 USDT)
	marketA := newArbMarket("BTCUSDT", "BTC", "USDT", 22800.0, 1.0, 22700.0, 1.0)
	marketB := newArbMarket("ETHBTC", "ETH", "BTC", 0.070, 1.0, 0.069, 2.0)
	marketC := newArbMarket("ETHUSDT", "ETH", "USDT", 1630.0, 0.5, 1620.0, 0.5)
	path := &Path{
		marketA: marketA,
		marketB: marketB,
		marketC: marketC,
		dirA:    -1,
		dirB:    -1,
		dirC:    1,
	}
	orders := path.newOrders(types.BalanceMap{
		"USDT": {
			Currency:  "USDT",
			Available: fixedpoint.NewFromFloat(2280.0),
			Locked:    fixedpoint.Zero,
			Borrowed:  fixedpoint.Zero,
			Interest:  fixedpoint.Zero,
			NetAsset:  fixedpoint.Zero,
		},
	}, 1)
	for i, order := range orders {
		t.Logf("order #%d: %+v", i, order.String())
	}

	assert.Equal(t, "0.03499", orders[0].Quantity.String())
	assert.Equal(t, "0.5", orders[1].Quantity.String())
	assert.Equal(t, "0.5", orders[2].Quantity.String())
}

func Test_fitQuantityByQuote(t *testing.T) {
	type args struct {
		price        float64
		quantity     float64
		quoteBalance float64
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "simple",
			args: args{
				price:        1630.0,
				quantity:     2.0,
				quoteBalance: 1000,
			},
			want: 0.6134969325153374,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := fitQuantityByQuote(tt.args.price, tt.args.quantity, tt.args.quoteBalance)
			if !assert.Equal(t, got, tt.want) {
				t.Errorf("fitQuantityByQuote() got = %v, want %v", got, tt.want)
			}
		})
	}
}
