package trinity

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func newArbMarket(symbol, base, quote string, askPrice, askVolume, bidPrice, bidVolume float64) *ArbMarket {
	return &ArbMarket{
		Symbol:        symbol,
		BaseCurrency:  base,
		QuoteCurrency: quote,
		market:        types.Market{Symbol: symbol, BaseCurrency: base, QuoteCurrency: quote},
		book:          nil,
		bestBid: types.PriceVolume{
			Price:  fixedpoint.NewFromFloat(bidPrice),
			Volume: fixedpoint.NewFromFloat(bidVolume),
		},
		bestAsk: types.PriceVolume{
			Price:  fixedpoint.NewFromFloat(askPrice),
			Volume: fixedpoint.NewFromFloat(askVolume),
		},
		buyRate:  1.0 / askPrice,
		sellRate: bidPrice,
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

func TestPath_NewForwardOrders(t *testing.T) {
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
	orders := path.newForwardOrders(types.BalanceMap{
		"USDT": {
			Currency:  "USDT",
			Available: fixedpoint.NewFromFloat(2280.0),
			Locked:    fixedpoint.Zero,
			Borrowed:  fixedpoint.Zero,
			Interest:  fixedpoint.Zero,
			NetAsset:  fixedpoint.Zero,
		},
	})
	for i, order := range orders {
		t.Logf("order #%d: %+v", i, order.String())
	}

	assert.True(t, orders[2].Price.Mul(orders[2].Quantity).Compare(fixedpoint.NewFromFloat(2314.28)) > 0)
}

func Test_fitQuantityByQuote(t *testing.T) {
	type args struct {
		price        float64
		quantity     float64
		quoteBalance float64
	}
	tests := []struct {
		name  string
		args  args
		want  float64
		want1 float64
	}{
		{
			name: "simple",
			args: args{
				price:        1630.0,
				quantity:     2.0,
				quoteBalance: 1000,
			},
			want:  0.6134969325153374,
			want1: 0.3067484662576687,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := fitQuantityByQuote(tt.args.price, tt.args.quantity, tt.args.quoteBalance)
			if !assert.Equal(t, got, tt.want) {
				t.Errorf("fitQuantityByQuote() got = %v, want %v", got, tt.want)
			}
			if !assert.Equal(t, got1, tt.want1) {
				t.Errorf("fitQuantityByQuote() got = %v, want %v", got, tt.want1)
			}
		})
	}
}
