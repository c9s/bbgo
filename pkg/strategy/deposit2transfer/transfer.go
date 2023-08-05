package deposit2transfer

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type FuturesTransfer interface {
	TransferFuturesAccountAsset(ctx context.Context, asset string, amount fixedpoint.Value, io types.TransferDirection) error
	QueryAccountBalances(ctx context.Context) (types.BalanceMap, error)
}

func (s *Strategy) resetTransfer(ctx context.Context, ex FuturesTransfer, asset string) error {
	balances, err := s.futuresSession.Exchange.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	b, ok := balances[asset]
	if !ok {
		return nil
	}

	amount := b.MaxWithdrawAmount
	if amount.IsZero() {
		return nil
	}

	log.Infof("transfering out futures account asset %s %s", amount, asset)

	err = ex.TransferFuturesAccountAsset(ctx, asset, amount, types.TransferOut)
	if err != nil {
		return err
	}
	return nil
}

func (s *Strategy) transferOut(ctx context.Context, ex FuturesTransfer, asset string, quantity fixedpoint.Value) error {
	balances, err := s.futuresSession.Exchange.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	b, ok := balances[asset]
	if !ok {
		return fmt.Errorf("%s balance not found", asset)
	}

	log.Infof("found futures balance: %+v", b)

	// add the previous pending base transfer and the current trade quantity
	amount := b.MaxWithdrawAmount

	// try to transfer more if we enough balance
	amount = fixedpoint.Min(amount, b.MaxWithdrawAmount)

	// TODO: according to the fee, we might not be able to get enough balance greater than the trade quantity, we can adjust the quantity here
	if amount.IsZero() {
		return nil
	}

	// de-leverage and get the collateral base quantity
	collateralBase := s.FuturesPosition.GetBase().Abs().Div(s.Leverage)
	_ = collateralBase

	// if s.State.TotalBaseTransfer.Compare(collateralBase)

	log.Infof("transfering out futures account asset %s %s", amount, asset)
	if err := ex.TransferFuturesAccountAsset(ctx, asset, amount, types.TransferOut); err != nil {
		return err
	}

	return nil
}

func (s *Strategy) transferIn(ctx context.Context, ex FuturesTransfer, asset string, quantity fixedpoint.Value) error {
	balances, err := s.spotSession.Exchange.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	b, ok := balances[asset]
	if !ok {
		return fmt.Errorf("%s balance not found", asset)
	}

	// TODO: according to the fee, we might not be able to get enough balance greater than the trade quantity, we can adjust the quantity here
	if !quantity.IsZero() && b.Available.Compare(quantity) < 0 {
		log.Infof("adding to pending base transfer: %s %s", quantity, asset)
		return nil
	}

	amount := b.Available

	log.Infof("transfering in futures account asset %s %s", amount, asset)
	if err := ex.TransferFuturesAccountAsset(ctx, asset, amount, types.TransferIn); err != nil {
		return err
	}

	return nil
}
