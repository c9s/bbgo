package currency

import (
	"math/big"
	"slices"

	"github.com/leekchan/accounting"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type wrapper struct {
	accounting.Accounting
}

func (w *wrapper) FormatMoney(v fixedpoint.Value) string {
	f := new(big.Float)
	f.SetString(v.String())
	return w.Accounting.FormatMoneyBigFloat(f)
}

var USD = wrapper{accounting.Accounting{Symbol: "$ ", Precision: 2}}
var BTC = wrapper{accounting.Accounting{Symbol: "BTC ", Precision: 8}}
var BNB = wrapper{accounting.Accounting{Symbol: "BNB ", Precision: 4}}

const (
	USDT = "USDT"
	USDC = "USDC"
	BUSD = "BUSD"
)

var FiatCurrencies = []string{"USDC", "USDT", "USD", "TWD", "EUR", "GBP", "BUSD"}

// USDFiatCurrencies lists the USD stable coins
var USDFiatCurrencies = []string{"USDT", "USDC", "USD", "BUSD"}

func IsUSDFiatCurrency(currency string) bool {
	return slices.Contains(USDFiatCurrencies, currency)
}

func IsFiatCurrency(currency string) bool {
	return slices.Contains(FiatCurrencies, currency)
}
