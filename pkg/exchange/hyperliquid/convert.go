package hyperliquid

import (
	"math"
	"strconv"
	"strings"

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
		LocalSymbol:     "@" + strconv.Itoa(s.Index),
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

func toLocalSpotAsset(symbol string) string {
	if s, ok := spotSymbolSyncMap.Load(symbol); ok {
		if localSymbol, ok := s.(string); ok {
			at := strings.LastIndexByte(localSymbol, '@')
			if at < 0 || at+1 >= len(localSymbol) {
				log.Errorf("invalid local symbol format %q for %s", localSymbol, symbol)
				return symbol
			}

			if asset, err := strconv.Atoi(localSymbol[at+1:]); err == nil {
				return strconv.Itoa(asset + 1000)
			}
		}

		log.Errorf("failed to convert symbol %s to local asset, but found in spotSymbolSyncMap", symbol)
	}

	log.Errorf("failed to look up local asset from %s", symbol)
	return symbol
}

func toLocalFuturesAsset(symbol string) string {
	if s, ok := futuresSymbolSyncMap.Load(symbol); ok {
		if localSymbol, ok := s.(string); ok {
			at := strings.LastIndexByte(localSymbol, '@')
			if at < 0 || at+1 >= len(localSymbol) {
				log.Errorf("invalid local symbol format %q for %s", localSymbol, symbol)
				return symbol
			}

			return localSymbol[at+1:]
		}

		log.Errorf("failed to convert symbol %s to local asset, but found in futuresSymbolSyncMaps", symbol)
	}

	log.Errorf("failed to look up local asset from %s", symbol)
	return symbol
}

func toGlobalBalance(account *hyperapi.Account) types.BalanceMap {
	balances := make(types.BalanceMap)
	for _, b := range account.Balances {
		available := b.Total.Sub(b.Hold)
		balances[b.Coin] = types.Balance{
			Currency:          b.Coin,
			Available:         available,
			Locked:            b.Hold,
			NetAsset:          b.Total,
			MaxWithdrawAmount: available,
		}
	}
	return balances
}

func toGlobalFuturesAccountInfo(account *hyperapi.FuturesAccount) *types.FuturesAccount {
	futuresAccount := &types.FuturesAccount{
		Assets:    make(types.FuturesAssetMap),
		Positions: make(types.FuturesPositionMap),
	}
	// TODO implement the conversion
	return futuresAccount
}
