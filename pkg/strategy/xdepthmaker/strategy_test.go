//go:build !dnum

package xdepthmaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func newTestBTCUSDTMarket() types.Market {
	return types.Market{
		BaseCurrency:    "BTC",
		QuoteCurrency:   "USDT",
		TickSize:        Number(0.01),
		StepSize:        Number(0.000001),
		PricePrecision:  2,
		VolumePrecision: 8,
		MinNotional:     Number(8.0),
		MinQuantity:     Number(0.0003),
	}
}

func TestStrategy_generateMakerOrders(t *testing.T) {
	s := &Strategy{
		Symbol:    "BTCUSDT",
		NumLayers: 3,
		DepthScale: &bbgo.LayerScale{
			LayerRule: &bbgo.SlideRule{
				LinearScale: &bbgo.LinearScale{
					Domain: [2]float64{1.0, 3.0},
					Range:  [2]float64{1000.0, 15000.0},
				},
			},
		},
		CrossExchangeMarketMakingStrategy: &CrossExchangeMarketMakingStrategy{
			makerMarket: newTestBTCUSDTMarket(),
		},
	}

	pricingBook := types.NewStreamBook("BTCUSDT")
	pricingBook.Load(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids: types.PriceVolumeSlice{
			{Price: Number("25000.00"), Volume: Number("0.1")},
			{Price: Number("24900.00"), Volume: Number("0.2")},
			{Price: Number("24800.00"), Volume: Number("0.3")},
			{Price: Number("24700.00"), Volume: Number("0.4")},
		},
		Asks: types.PriceVolumeSlice{
			{Price: Number("25100.00"), Volume: Number("0.1")},
			{Price: Number("25200.00"), Volume: Number("0.2")},
			{Price: Number("25300.00"), Volume: Number("0.3")},
			{Price: Number("25400.00"), Volume: Number("0.4")},
		},
		Time: time.Now(),
	})

	orders, err := s.generateMakerOrders(pricingBook, 0, fixedpoint.PosInf, fixedpoint.PosInf)
	assert.NoError(t, err)
	AssertOrdersPriceSideQuantity(t, []PriceSideQuantityAssert{
		{Side: types.SideTypeBuy, Price: Number("25000"), Quantity: Number("0.04")},        // =~ $1000.00
		{Side: types.SideTypeBuy, Price: Number("24866.66"), Quantity: Number("0.281715")}, // =~ $7005.3111219, accumulated amount =~ $1000.00 + $7005.3111219 = $8005.3111219
		{Side: types.SideTypeBuy, Price: Number("24800"), Quantity: Number("0.283123")},    // =~ $7021.4504, accumulated amount =~ $1000.00 + $7005.3111219 + $7021.4504 = $8005.3111219 + $7021.4504 =~ $15026.7615219
		{Side: types.SideTypeSell, Price: Number("25100"), Quantity: Number("0.03984")},
		{Side: types.SideTypeSell, Price: Number("25233.33"), Quantity: Number("0.2772")},
		{Side: types.SideTypeSell, Price: Number("25233.33"), Quantity: Number("0.277411")},
	}, orders)
}
