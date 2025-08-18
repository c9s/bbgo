package okex

import (
	"context"
	"fmt"
	"strings"

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

func (e *Exchange) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if e.IsFutures {
		mgnMode := okexapi.MarginModeCross
		if e.IsIsolatedFutures {
			mgnMode = okexapi.MarginModeIsolated
		}

		_, err := e.client.NewSetAccountLeverageRequest().
			InstrumentId(toLocalSymbol(symbol, okexapi.InstrumentTypeSwap)).
			Leverage(leverage).
			MarginMode(okexapi.MarginMode(mgnMode)).
			Do(ctx)
		return err
	}
	return fmt.Errorf("not supported set leverage")
}

func (e *Exchange) QueryPositionRisk(ctx context.Context, symbol ...string) ([]types.PositionRisk, error) {
	var (
		positions []okexapi.Position
		err       error
	)

	if e.IsFutures {
		req := e.client.NewGetAccountPositionsRequest().
			InstType(okexapi.InstrumentTypeSwap)
		if len(symbol) > 0 {
			symbols := make([]string, len(symbol))
			for i, s := range symbol {
				symbols[i] = toLocalSymbol(s, okexapi.InstrumentTypeSwap)
			}
			req.InstId(strings.Join(symbols, ","))
		}
		if positions, err = req.Do(ctx); err != nil {
			return nil, err
		}
	}

	return toGlobalPositionRisk(positions), nil
}
