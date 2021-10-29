package types

import "github.com/leekchan/accounting"

var USD = accounting.Accounting{Symbol: "$ ", Precision: 2}
var BTC = accounting.Accounting{Symbol: "BTC ", Precision: 8}
var BNB = accounting.Accounting{Symbol: "BNB ", Precision: 4}

var FiatCurrencies = []string{"USDC", "USDT", "USD", "TWD", "EUR", "GBP", "BUSD"}

