package bbgo

import (
	"context"
	"sync"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"

	log "github.com/sirupsen/logrus"
)

type Account struct {
	mu sync.Mutex

	Balances map[string]types.Balance
}

func LoadAccount(ctx context.Context, exchange types.Exchange) (*Account, error) {
	balances, err := exchange.QueryAccountBalances(ctx)
	return &Account{
		Balances: balances,
	}, err
}

func (a *Account) BindPrivateStream(stream types.Stream) {
	stream.OnBalanceSnapshot(func(snapshot map[string]types.Balance) {
		a.mu.Lock()
		defer a.mu.Unlock()

		for _, balance := range snapshot {
			a.Balances[balance.Currency] = balance
		}
	})

}

func (a *Account) Print() {
	for _, balance := range a.Balances {
		if util.NotZero(balance.Available) {
			log.Infof("[trader] balance %s %f", balance.Currency, balance.Available)
		}
	}
}
