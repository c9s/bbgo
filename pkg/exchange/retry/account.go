package retry

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

func QueryAccountUntilSuccessful(
	ctx context.Context, ex types.ExchangeAccountService,
) (account *types.Account, err error) {
	var op = func() (err2 error) {
		account, err2 = ex.QueryAccount(ctx)
		if err2 != nil {
			log.WithError(err2).Errorf("failed to query account")
		}
	
		return err2
	}

	err = GeneralBackoff(ctx, op)
	return account, err
}

func QueryAccountBalancesUntilSuccessful(
	ctx context.Context, ex types.ExchangeAccountService,
) (bals types.BalanceMap, err error) {
	var op = func() (err2 error) {
		bals, err2 = ex.QueryAccountBalances(ctx)
		return err2
	}

	err = GeneralBackoff(ctx, op)
	return bals, err
}

func QueryAccountBalancesUntilSuccessfulLite(
	ctx context.Context, ex types.ExchangeAccountService,
) (bals types.BalanceMap, err error) {
	var op = func() (err2 error) {
		bals, err2 = ex.QueryAccountBalances(ctx)
		return err2
	}

	err = GeneralLiteBackoff(ctx, op)
	return bals, err
}
