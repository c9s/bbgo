package xmaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func TestStrategy_getLayerPrice(t *testing.T) {
	symbol := "BTCUSDT"
	market := Market(symbol)

	s := &Strategy{
		UseDepthPrice: true,
		DepthQuantity: Number(3.0),
		makerMarket:   market,
	}

	sourceBook := types.NewStreamBook(symbol, types.ExchangeBinance)
	sourceBook.Load(types.SliceOrderBook{
		Symbol: symbol,
		Bids: PriceVolumeSlice(
			Number(1300.0), Number(1.0),
			Number(1200.0), Number(2.0),
			Number(1100.0), Number(3.0),
		),
		Asks: PriceVolumeSlice(
			Number(1301.0), Number(1.0),
			Number(1400.0), Number(2.0),
			Number(1500.0), Number(3.0),
		),
		Time:         time.Time{},
		LastUpdateId: 1,
	})

	quote := &Quote{
		BestBidPrice: Number(1300.0),
		BestAskPrice: Number(1301.0),
		BidMargin:    Number(0.001),
		AskMargin:    Number(0.001),
		BidLayerPips: Number(100.0),
		AskLayerPips: Number(100.0),
	}

	t.Run("depthPrice bid price at 0", func(t *testing.T) {
		price := s.getLayerPrice(0, types.SideTypeBuy, sourceBook, quote, s.DepthQuantity)

		// (1300 + 1200*2)/3 * (1 - 0.001)
		assert.InDelta(t, 1232.10, price.Float64(), 0.01)
	})

	t.Run("depthPrice bid price at 1", func(t *testing.T) {
		price := s.getLayerPrice(1, types.SideTypeBuy, sourceBook, quote, s.DepthQuantity)

		// (1300 + 1200*2)/3 * (1 - 0.001) - 100 * 0.01
		assert.InDelta(t, 1231.10, price.Float64(), 0.01)
	})

	t.Run("depthPrice ask price at 0", func(t *testing.T) {
		price := s.getLayerPrice(0, types.SideTypeSell, sourceBook, quote, s.DepthQuantity)

		// (1301 + 1400*2)/3 * (1 + 0.001)
		assert.InDelta(t, 1368.367, price.Float64(), 0.01)
	})

	t.Run("depthPrice ask price at 1", func(t *testing.T) {
		price := s.getLayerPrice(1, types.SideTypeSell, sourceBook, quote, s.DepthQuantity)

		// (1301 + 1400*2)/3 * (1 + 0.001) + 100 * 0.01
		assert.InDelta(t, 1369.367, price.Float64(), 0.01)
	})

}

func Test_aggregatePrice(t *testing.T) {
	bids := PriceVolumeSliceFromText(`
	1000.0, 1.0
	1200.0, 1.0
	1400.0, 1.0
`)

	aggregatedPrice1 := aggregatePrice(bids, fixedpoint.NewFromFloat(0.5))
	assert.Equal(t, fixedpoint.NewFromFloat(1000.0), aggregatedPrice1)

	aggregatedPrice2 := aggregatePrice(bids, fixedpoint.NewFromInt(1))
	assert.Equal(t, fixedpoint.NewFromFloat(1000.0), aggregatedPrice2)

	aggregatedPrice3 := aggregatePrice(bids, fixedpoint.NewFromInt(2))
	assert.Equal(t, fixedpoint.NewFromFloat(1100.0), aggregatedPrice3)

}
