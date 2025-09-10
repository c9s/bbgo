package xmaker

import (
	"context"
	"time"

	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

var repayRateLimiter = rate.NewLimiter(rate.Every(30*time.Second), 1)

func tryToRepayDebts(ctx context.Context, session *bbgo.ExchangeSession) bool {
	if !repayRateLimiter.Allow() {
		return false
	}

	hedgeBalances := session.GetAccount().Balances()
	debts := hedgeBalances.Debts()

	log.Infof("trying to repay debts %+v on hedge exchange %s", debts, session.Exchange.Name())

	repayables := make(map[string]fixedpoint.Value)
	for asset, bal := range debts {
		if bal.Borrowed.IsZero() || bal.Available.IsZero() {
			continue
		}

		repayables[asset] = bal.Available
	}

	marginService, ok := session.Exchange.(types.MarginBorrowRepayService)
	if !ok {
		return false
	}

	repaid := false
	for asset, amount := range repayables {
		if amount.IsZero() {
			continue
		}

		if err := marginService.RepayMarginAsset(ctx, asset, amount); err != nil {
			log.WithError(err).Errorf("unable to repay %s asset", asset)
			continue
		}

		repaid = true
	}

	if repaid {
		if _, err := session.UpdateAccount(ctx); err != nil {
			log.WithError(err).Errorf("unable to update account after repay")
		}
	}

	return repaid
}
