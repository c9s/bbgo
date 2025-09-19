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

	account, err := session.UpdateAccount(ctx)
	if err != nil {
		log.WithError(err).Errorf("unable to update account for repay")
		return false
	}

	balances := account.Balances()
	debts := balances.Debts()

	log.Infof("trying to repay debts %+v on hedge makerExchange %s", debts, session.Exchange.Name())

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

	// if we have repaid something, update the account again.
	// this is to make sure the account info is up-to-date.
	// we don't want to wait until the next periodic update.
	if repaid {
		if _, err := session.UpdateAccount(ctx); err != nil {
			log.WithError(err).Errorf("unable to update account after repay")
		}
	}

	return repaid
}
