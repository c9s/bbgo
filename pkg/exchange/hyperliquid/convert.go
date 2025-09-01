package hyperliquid

import (
	"math"
	"strconv"

	"github.com/c9s/bbgo/pkg/exchange/hyperliquid/hyperapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalSpotMarket(s hyperapi.UniverseMeta, tokens []hyperapi.TokenMeta) types.Market {
	base, quote := tokens[s.Tokens[0]], tokens[s.Tokens[1]]
	tickSize := fixedpoint.NewFromFloat(math.Pow10(-quote.SzDecimals))
	stepSize := fixedpoint.NewFromFloat(math.Pow10(-base.SzDecimals))

	return types.Market{
		Exchange:        types.ExchangeHyperliquid,
		Symbol:          base.Name + quote.Name,
		LocalSymbol:     strconv.Itoa(s.Index + 1000),
		BaseCurrency:    base.Name,
		QuoteCurrency:   quote.Name,
		TickSize:        tickSize,
		StepSize:        stepSize,
		MinPrice:        fixedpoint.Zero, // not used
		MaxPrice:        fixedpoint.Zero, // not used
		MinNotional:     stepSize.Mul(tickSize),
		MinAmount:       stepSize,
		MinQuantity:     stepSize,
		MaxQuantity:     fixedpoint.NewFromFloat(1e9),
		PricePrecision:  quote.SzDecimals,
		VolumePrecision: base.SzDecimals,
	}
}
