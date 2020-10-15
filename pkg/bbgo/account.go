package bbgo

import (
	"sync"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"

	log "github.com/sirupsen/logrus"
)

type Account struct {
	sync.Mutex
	balances map[string]types.Balance
}

func (a *Account) Balance(currency string) (balance types.Balance, ok bool) {
	a.Lock()
	balance, ok = a.balances[currency]
	a.Unlock()
	return balance, ok
}

func (a *Account) handleBalanceUpdates(balances map[string]types.Balance) {
	a.Lock()
	defer a.Unlock()

	for _, balance := range balances {
		a.balances[balance.Currency] = balance
	}
}

func (a *Account) BindStream(stream types.Stream) {
	stream.OnBalanceUpdate(a.handleBalanceUpdates)
	stream.OnBalanceSnapshot(a.handleBalanceUpdates)
}

func (a *Account) Print() {
	a.Lock()
	defer a.Unlock()

	for _, balance := range a.balances {
		if util.NotZero(balance.Available) {
			log.Infof("account balance %s %f", balance.Currency, balance.Available)
		}
	}
}
