package okex

import (
	"strings"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalSymbol(symbol string) string {
	return strings.ReplaceAll(symbol, "-", "")
}

////go:generate sh -c "echo \"package okex\nvar spotSymbolMap = map[string]string{\n\" $(curl -s -L 'https://okex.com/api/v5/public/instruments?instType=SPOT' | jq -r '.data[] | \"\\(.instId | sub(\"-\" ; \"\") | tojson ): \\( .instId | tojson),\n\"') \"\n}\" > symbols.go"
//go:generate go run gensymbols.go
func toLocalSymbol(symbol string) string {
	if s, ok := spotSymbolMap[symbol]; ok {
		return s
	}

	log.Errorf("failed to look up local symbol from %s", symbol)
	return symbol
}

func toGlobalTicker(marketTicker okexapi.MarketTicker) *types.Ticker {
	return &types.Ticker{
		Time:   marketTicker.Timestamp.Time(),
		Volume: marketTicker.Volume24H.Float64(),
		Last:   marketTicker.Last.Float64(),
		Open:   marketTicker.Open24H.Float64(),
		High:   marketTicker.High24H.Float64(),
		Low:    marketTicker.Low24H.Float64(),
		Buy:    marketTicker.BidPrice.Float64(),
		Sell:   marketTicker.AskPrice.Float64(),
	}
}

func toGlobalBalance(balanceSummaries []okexapi.BalanceSummary) types.BalanceMap {
	var balanceMap = types.BalanceMap{}
	for _, balanceSummary := range balanceSummaries {
		for _, balanceDetail := range balanceSummary.Details {
			balanceMap[balanceDetail.Currency] = types.Balance{
				Currency:  balanceDetail.Currency,
				Available: balanceDetail.CashBalance,
				Locked:    balanceDetail.Frozen,
			}
		}
	}
	return balanceMap
}
