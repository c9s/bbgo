package types

import (
	"fmt"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var debugBalance = false

func init() {
	debugBalance = viper.GetBool("debug-balance")
}

type Balance struct {
	Currency  string           `json:"currency"`
	Available fixedpoint.Value `json:"available"`
	Locked    fixedpoint.Value `json:"locked"`
}

func (b Balance) Total() fixedpoint.Value {
	return b.Available + b.Locked
}

func (b Balance) String() string {
	if b.Locked > 0 {
		return fmt.Sprintf("%s: %f (locked %f)", b.Currency, b.Available.Float64(), b.Locked.Float64())
	}

	return fmt.Sprintf("%s: %f", b.Currency, b.Available.Float64())
}

type Asset struct {
	Currency string           `json:"currency"`
	Total    fixedpoint.Value `json:"total"`
	InUSD    fixedpoint.Value `json:"inUSD"`
	InBTC    fixedpoint.Value `json:"inBTC"`
}

type AssetMap map[string]Asset

type BalanceMap map[string]Balance

func (m BalanceMap) String() string {
	var ss []string
	for _, b := range m {
		ss = append(ss, b.String())
	}

	return "BalanceMap[" + strings.Join(ss, ", ") + "]"
}

func (m BalanceMap) Assets(prices map[string]float64) AssetMap {
	assets := make(AssetMap)

	for currency, b := range m {
		if b.Locked == 0 && b.Available == 0 {
			continue
		}

		asset := Asset{
			Currency: currency,
			Total:    b.Available + b.Locked,
		}

		btcusdt, hasBtcPrice := prices["BTCUSDT"]

		usdMarkets := []string{currency + "USDT", currency + "USDC", currency + "USD", "USDT" + currency}
		for _, market := range usdMarkets {
			if val, ok := prices[market]; ok {

				if strings.HasPrefix(market, "USD") {
					asset.InUSD = fixedpoint.NewFromFloat(asset.Total.Float64() / val)
				} else {
					asset.InUSD = asset.Total.MulFloat64(val)
				}

				if hasBtcPrice {
					asset.InBTC = fixedpoint.NewFromFloat(asset.InUSD.Float64() / btcusdt)
				}
			}
		}

		assets[currency] = asset
	}

	return assets
}

func (m BalanceMap) Print() {
	for _, balance := range m {
		if balance.Available == 0 && balance.Locked == 0 {
			continue
		}

		if balance.Locked > 0 {
			logrus.Infof(" %s: %f (locked %f)", balance.Currency, balance.Available.Float64(), balance.Locked.Float64())
		} else {
			logrus.Infof(" %s: %f", balance.Currency, balance.Available.Float64())
		}
	}
}

type Account struct {
	sync.Mutex `json:"-"`

	// bps. 0.15% fee will be 15.
	MakerCommission fixedpoint.Value `json:"makerCommission,omitempty"`
	TakerCommission fixedpoint.Value `json:"takerCommission,omitempty"`

	MakerFeeRate fixedpoint.Value `json:"makerFeeRate,omitempty"`
	TakerFeeRate fixedpoint.Value `json:"takerFeeRate,omitempty"`
	AccountType  string           `json:"accountType,omitempty"`

	TotalAccountValue fixedpoint.Value `json:"totalAccountValue,omitempty"`

	balances BalanceMap
}

func NewAccount() *Account {
	return &Account{
		balances: make(BalanceMap),
	}
}

// Balances lock the balances and returned the copied balances
func (a *Account) Balances() BalanceMap {
	d := make(BalanceMap)

	a.Lock()
	for c, b := range a.balances {
		d[c] = b
	}
	a.Unlock()

	return d
}

func (a *Account) Balance(currency string) (balance Balance, ok bool) {
	a.Lock()
	balance, ok = a.balances[currency]
	a.Unlock()
	return balance, ok
}

func (a *Account) AddBalance(currency string, fund fixedpoint.Value) error {
	a.Lock()
	defer a.Unlock()

	balance, ok := a.balances[currency]
	if ok {
		balance.Available += fund
		a.balances[currency] = balance
		return nil
	}

	a.balances[currency] = Balance{
		Currency:  currency,
		Available: fund,
		Locked:    0,
	}
	return nil
}

func (a *Account) UseLockedBalance(currency string, fund fixedpoint.Value) error {
	a.Lock()
	defer a.Unlock()

	balance, ok := a.balances[currency]
	if ok && balance.Locked >= fund {
		balance.Locked -= fund
		a.balances[currency] = balance
		return nil
	}

	return fmt.Errorf("trying to use more than locked: locked %f < want to use %f", balance.Locked.Float64(), fund.Float64())
}

func (a *Account) UnlockBalance(currency string, unlocked fixedpoint.Value) error {
	a.Lock()
	defer a.Unlock()

	balance, ok := a.balances[currency]
	if !ok {
		return fmt.Errorf("trying to unlocked inexisted balance: %s", currency)
	}

	if unlocked > balance.Locked {
		return fmt.Errorf("trying to unlocked more than locked %s: locked %f < want to unlock %f", currency, balance.Locked.Float64(), unlocked.Float64())
	}

	balance.Locked -= unlocked
	balance.Available += unlocked
	a.balances[currency] = balance
	return nil
}

func (a *Account) LockBalance(currency string, locked fixedpoint.Value) error {
	a.Lock()
	defer a.Unlock()

	balance, ok := a.balances[currency]
	if ok && balance.Available >= locked {
		balance.Locked += locked
		balance.Available -= locked
		a.balances[currency] = balance
		return nil
	}

	return fmt.Errorf("insufficient available balance %s for lock: want to lock %f, available %f", currency, locked.Float64(), balance.Available.Float64())
}

func (a *Account) UpdateBalances(balances BalanceMap) {
	a.Lock()
	defer a.Unlock()

	if a.balances == nil {
		a.balances = make(BalanceMap)
	}

	for _, balance := range balances {
		a.balances[balance.Currency] = balance
	}
}

func printBalanceUpdate(balances BalanceMap) {
	logrus.Infof("balance update: %+v", balances)
}

func (a *Account) BindStream(stream Stream) {
	stream.OnBalanceUpdate(a.UpdateBalances)
	stream.OnBalanceSnapshot(a.UpdateBalances)
	if debugBalance {
		stream.OnBalanceUpdate(printBalanceUpdate)
	}
}

func (a *Account) Print() {
	a.Lock()
	defer a.Unlock()

	if a.AccountType != "" {
		logrus.Infof("account type: %s", a.AccountType)
	}

	if a.MakerFeeRate > 0 {
		logrus.Infof("maker fee rate: %f", a.MakerFeeRate.Float64())
	}
	if a.TakerFeeRate > 0 {
		logrus.Infof("taker fee rate: %f", a.TakerFeeRate.Float64())
	}

	a.balances.Print()
}
