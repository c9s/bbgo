package xfunding

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

	s.State.PendingBaseTransfer = fixedpoint.Zero
	s.State.TotalBaseTransfer = fixedpoint.Zero
	return nil
}

func (s *Strategy) transferOut(ctx context.Context, ex FuturesTransfer, asset string, quantity fixedpoint.Value) error {
	// if transfer done
	// TotalBaseTransfer here is the rest quantity we need to transfer
	// (total spot -> futures transfer amount) is recorded in this variable.
	//
	// TotalBaseTransfer == 0 means we have nothing to transfer.
	if s.State.TotalBaseTransfer.IsZero() {
		return nil
	}

	quantity = quantity.Add(s.State.PendingBaseTransfer)

	// A simple protection here -- we can only transfer the rest quota (total base transfer) back to spot
	quantity = fixedpoint.Min(s.State.TotalBaseTransfer, quantity)

	available, pending, err := s.queryAvailableTransfer(ctx, s.futuresSession.Exchange, asset, quantity)
	if err != nil {
		s.State.PendingBaseTransfer = quantity
		return err
	}

	s.State.PendingBaseTransfer = pending

	log.Infof("transfering out futures account asset %f %s", available.Float64(), asset)
	if err := ex.TransferFuturesAccountAsset(ctx, asset, available, types.TransferOut); err != nil {
		s.State.PendingBaseTransfer = s.State.PendingBaseTransfer.Add(available)
		return err
	}

	// reduce the transfer in the total base transfer
	s.State.TotalBaseTransfer = s.State.TotalBaseTransfer.Sub(available)
	return nil
}

// transferIn transfers the asset from the spot account to the futures account
func (s *Strategy) transferIn(ctx context.Context, ex FuturesTransfer, asset string, quantity fixedpoint.Value) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// add the pending transfer and reset the pending transfer
	quantity = s.State.PendingBaseTransfer.Add(quantity)

	available, pending, err := s.queryAvailableTransfer(ctx, s.spotSession.Exchange, asset, quantity)
	if err != nil {
		s.State.PendingBaseTransfer = quantity
		return err
	}

	s.State.PendingBaseTransfer = pending

	if available.IsZero() {
		return fmt.Errorf("unable to transfer zero %s from spot wallet to futures wallet", asset)
	}

	log.Infof("transfering %f %s from the spot wallet into futures wallet...", available.Float64(), asset)
	if err := ex.TransferFuturesAccountAsset(ctx, asset, available, types.TransferIn); err != nil {
		s.State.PendingBaseTransfer = s.State.PendingBaseTransfer.Add(available)
		return err
	}

	// record the transfer in the total base transfer
	s.State.TotalBaseTransfer = s.State.TotalBaseTransfer.Add(available)
	return nil
}

func (s *Strategy) queryAvailableTransfer(
	ctx context.Context, ex types.Exchange, asset string, quantity fixedpoint.Value,
) (available, pending fixedpoint.Value, err error) {
	available = fixedpoint.Zero
	pending = fixedpoint.Zero

	// query spot balances to validate the quantity
	balances, err := ex.QueryAccountBalances(ctx)
	if err != nil {
		return available, pending, err
	}

	b, ok := balances[asset]
	if !ok {
		return available, pending, fmt.Errorf("%s balance not found", asset)
	}

	log.Infof("loaded %s balance: %+v", asset, b)

	// if quantity = 0, we will transfer all available balance into the futures wallet
	if quantity.IsZero() {
		quantity = b.Available
	}

	limit := b.Available
	if b.MaxWithdrawAmount.Sign() > 0 {
		limit = fixedpoint.Min(b.MaxWithdrawAmount, limit)
	}

	if limit.Compare(quantity) < 0 {
		log.Infof("%s available balance is not enough for transfer (%f < %f)",
			asset,
			b.Available.Float64(),
			quantity.Float64())

		available = fixedpoint.Min(limit, quantity)
		pending = quantity.Sub(available)
		log.Infof("adjusted transfer quantity from %f to %f", quantity.Float64(), available.Float64())
		return available, pending, nil
	}

	available = quantity
	pending = fixedpoint.Zero
	return available, pending, nil
}
