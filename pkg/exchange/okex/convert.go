package okex

import "strings"

func toGlobalSymbol(symbol string) string {
	return strings.ReplaceAll(symbol, "-", "")
}
