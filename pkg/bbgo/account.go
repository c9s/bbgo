package bbgo

import (
	"sync"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"

	log "github.com/sirupsen/logrus"
)

type Account struct {
	sync.Mutex
	Balances map[string]types.Balance
}

func (a *Account) handleBalanceUpdates(balances map[string]types.Balance) {
	a.Lock()
	defer a.Unlock()

	for _, balance := range balances {
		a.Balances[balance.Currency] = balance
	}
}

func (a *Account) BindStream(stream types.Stream) {
	stream.OnBalanceUpdate(a.handleBalanceUpdates)
	stream.OnBalanceSnapshot(a.handleBalanceUpdates)
}

func (a *Account) Print() {
	a.Lock()
	defer a.Unlock()

	for _, balance := range a.Balances {
		if util.NotZero(balance.Available) {
			log.Infof("[trader] balance %s %f", balance.Currency, balance.Available)
		}
	}
}
