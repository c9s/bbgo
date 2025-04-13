package okex

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/types"
)

func (e *Exchange) QueryFuturesAccount(ctx context.Context) (*types.Account, error) {
	accounts, err := e.queryAccountBalance(ctx)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("account balance is empty")
	}

	positions, err := e.client.NewGetAccountPositionsRequest().
		InstType(okexapi.InstrumentTypeSwap).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	balances := toGlobalBalance(&accounts[0])
	account := types.NewAccount()
	account.UpdateBalances(balances)
	account.AccountType = types.AccountTypeFutures
	account.FuturesInfo = toGlobalFuturesAccountInfo(&accounts[0], positions)

	return account, nil
}
