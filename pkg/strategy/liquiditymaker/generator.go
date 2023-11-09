package liquiditymaker

import (
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// input: liquidityOrderGenerator(
//
//	totalLiquidityAmount,
//	startPrice,
//	endPrice,
//	numLayers,
//	quantityScale)
//
// when side == sell
//
//	priceAsk1 * scale(1) * f = amount1
//	priceAsk2 * scale(2) * f = amount2
//	priceAsk3 * scale(3) * f = amount3
//
// totalLiquidityAmount = priceAsk1 * scale(1) * f + priceAsk2 * scale(2) * f + priceAsk3 * scale(3) * f + ....
// totalLiquidityAmount = f * (priceAsk1 * scale(1)  + priceAsk2 * scale(2)  + priceAsk3 * scale(3) + ....)
// f = totalLiquidityAmount / (priceAsk1 * scale(1)  + priceAsk2 * scale(2)  + priceAsk3 * scale(3) + ....)
//
// when side == buy
//
//	priceBid1 * scale(1) * f = amount1
type LiquidityOrderGenerator struct {
	Symbol string
	Market types.Market

	logger log.FieldLogger
}

func (g *LiquidityOrderGenerator) Generate(
	side types.SideType, totalAmount, startPrice, endPrice fixedpoint.Value, numLayers int, scale bbgo.Scale,
) (orders []types.SubmitOrder) {

	if g.logger == nil {
		logger := log.New()
		logger.SetLevel(log.ErrorLevel)
		g.logger = logger
	}

	layerSpread := endPrice.Sub(startPrice).Div(fixedpoint.NewFromInt(int64(numLayers - 1)))
	switch side {
	case types.SideTypeSell:
		if layerSpread.Compare(g.Market.TickSize) < 0 {
			layerSpread = g.Market.TickSize
		}

	case types.SideTypeBuy:
		if layerSpread.Compare(g.Market.TickSize.Neg()) > 0 {
			layerSpread = g.Market.TickSize.Neg()
		}
	}

	quantityBase := 0.0
	var layerPrices []fixedpoint.Value
	var layerScales []float64
	for i := 0; i < numLayers; i++ {
		fi := fixedpoint.NewFromInt(int64(i))
		layerPrice := g.Market.TruncatePrice(startPrice.Add(layerSpread.Mul(fi)))
		layerPrices = append(layerPrices, layerPrice)

		layerScale := scale.Call(float64(i + 1))
		layerScales = append(layerScales, layerScale)

		quantityBase += layerPrice.Float64() * layerScale
	}

	factor := totalAmount.Float64() / quantityBase

	g.logger.Infof("liquidity amount base: %f, factor: %f", quantityBase, factor)

	for i := 0; i < numLayers; i++ {
		price := layerPrices[i]
		s := layerScales[i]

		quantity := factor * s
		orders = append(orders, types.SubmitOrder{
			Symbol:   g.Symbol,
			Price:    price,
			Type:     types.OrderTypeLimitMaker,
			Quantity: g.Market.TruncateQuantity(fixedpoint.NewFromFloat(quantity)),
			Side:     side,
			Market:   g.Market,
		})
	}

	return orders
}
