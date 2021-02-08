package ftx

import "strings"

func toGlobalCurrency(original string) string {
	return strings.ToUpper(strings.TrimSpace(original))
}
