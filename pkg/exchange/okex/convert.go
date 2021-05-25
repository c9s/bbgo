package okex

import "strings"

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
