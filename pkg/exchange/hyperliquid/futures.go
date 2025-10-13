package hyperliquid

import (
	"context"
	"math"
	"strconv"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const QuoteCurrency = "USDC"

func (e *Exchange) queryFuturesMarkets(ctx context.Context) (types.MarketMap, error) {
	meta, err := e.client.NewFuturesGetMetaRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for i, u := range meta.Universe {
		stepSize := fixedpoint.NewFromFloat(1 / math.Pow10(u.SzDecimals))
		tickSize := fixedpoint.NewFromFloat(1 / math.Pow10(8))
		markets.Add(types.Market{
			Exchange:        types.ExchangeHyperliquid,
			Symbol:          u.Name + QuoteCurrency,
			LocalSymbol:     strconv.Itoa(i),
			BaseCurrency:    u.Name,
			QuoteCurrency:   QuoteCurrency,
			PricePrecision:  8,
			VolumePrecision: u.SzDecimals,
			StepSize:        stepSize,
			TickSize:        tickSize,
			MinNotional:     stepSize.Mul(tickSize),
			MinAmount:       stepSize,
			MinQuantity:     stepSize,
			MaxQuantity:     fixedpoint.NewFromFloat(1e9),
		})
	}

	return markets, nil
}
