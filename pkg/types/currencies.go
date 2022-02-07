package types

import "math/big"

import "github.com/leekchan/accounting"

import "github.com/c9s/bbgo/pkg/fixedpoint"

type Acc = accounting.Accounting

type wrapper struct {
	Acc
}

func (w *wrapper) FormatMoney(v fixedpoint.Value) string {
	f := new(big.Float)
	f.SetString(v.String())
	return w.Acc.FormatMoneyBigFloat(f)
}

var USD = wrapper{accounting.Accounting{Symbol: "$ ", Precision: 2}}
var BTC = wrapper{accounting.Accounting{Symbol: "BTC ", Precision: 8}}
var BNB = wrapper{accounting.Accounting{Symbol: "BNB ", Precision: 4}}

var FiatCurrencies = []string{"USDC", "USDT", "USD", "TWD", "EUR", "GBP", "BUSD"}

func IsFiatCurrency(currency string) bool {
	for _, c := range FiatCurrencies {
		if c == currency {
			return true
		}
	}
	return false
}

