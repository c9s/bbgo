//go:build !dnum

package xdepthmaker

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
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
		logger: logrus.New(),
	}

	pricingBook := types.NewStreamBook("BTCUSDT", types.ExchangeBinance)
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
		{Side: types.SideTypeSell, Price: Number("25233.34"), Quantity: Number("0.2772")},
		{Side: types.SideTypeSell, Price: Number("25300"), Quantity: Number("0.275845")},
	}, orders)
}

func TestPriceVolumeSlice_AverageDepthPrice(t *testing.T) {
	book := types.NewSliceOrderBook("BTCUSDT")
	book.Update(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Asks: PriceVolumeSliceFromText(`
			59400,0.5123
			59402,0.0244
			59405,0.0413
			59450,0.02821
			60000,3
		`),
		Bids: PriceVolumeSliceFromText(`
			59399,0.3441 // = 20,439.1959
			59398,0.221  // = 13,126.958
			59395,0.000123 // = 7.305585
			59390,0.03312 // = 1,966.9968
			59000,3 // = 177,000
			// sum = 212,540.456285
		`),
		Time:         time.Now(),
		LastUpdateId: 0,
	})

	t.Run("test average price by base quantity", func(t *testing.T) {
		// Test buying 2 BTC
		buyPrice := book.Asks.AverageDepthPrice(fixedpoint.NewFromFloat(2))
		assert.InDelta(t, 59818.9699, 0.001, buyPrice.Float64())

		// Test selling 2 BTC
		sellPrice := book.Bids.AverageDepthPrice(fixedpoint.NewFromFloat(2))
		assert.InDelta(t, 59119.1096, 0.001, sellPrice.Float64())
	})

	t.Run("test average price by quote quantity", func(t *testing.T) {
		// Test buying with ~119637.9398 quote
		buyPrice := book.Asks.AverageDepthPriceByQuote(fixedpoint.NewFromFloat(119637.9398), 0)
		assert.InDelta(t, 59899.6009, buyPrice.Float64(), 0.001)

		// Test selling with ~118238.219281 quote
		sellPrice := book.Bids.AverageDepthPriceByQuote(fixedpoint.NewFromFloat(118238.219281), 0)
		assert.InDelta(t, 59066.2024, sellPrice.Float64(), 0.001)

		assert.Less(t, sellPrice.Float64(), buyPrice.Float64(), "the sell price should be lower than the buy price")
	})
}
