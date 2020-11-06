package types

import (
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/util"
)

type Balance struct {
	Currency  string  `json:"currency"`
	Available float64 `json:"available"`
	Locked    float64 `json:"locked"`
}

type BalanceMap map[string]Balance

type Account struct {
	sync.Mutex

	MakerCommission int `json:"makerCommission"`
	TakerCommission int `json:"takerCommission"`
	AccountType     string `json:"accountType"`

	balances BalanceMap
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

func (a *Account) UpdateBalances(balances map[string]Balance) {
	a.Lock()
	defer a.Unlock()

	if a.balances == nil {
		a.balances = make(BalanceMap)
	}

	for _, balance := range balances {
		a.balances[balance.Currency] = balance
	}
}

func (a *Account) BindStream(stream Stream) {
	stream.OnBalanceUpdate(a.UpdateBalances)
	stream.OnBalanceSnapshot(a.UpdateBalances)
}

func (a *Account) Print() {
	a.Lock()
	defer a.Unlock()

	for _, balance := range a.balances {
		if util.NotZero(balance.Available) {
			logrus.Infof("account balance %s %f", balance.Currency, balance.Available)
		}
	}
}
