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

func (s *Strategy) transferOut(ctx context.Context, ex FuturesTransfer, trade types.Trade) error {
	currency := s.spotMarket.BaseCurrency

	// base asset needs BUY trades
	if trade.Side == types.SideTypeBuy {
		return nil
	}

	balances, err := ex.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	b, ok := balances[currency]
	if !ok {
		return fmt.Errorf("%s balance not found", currency)
	}

	// TODO: according to the fee, we might not be able to get enough balance greater than the trade quantity, we can adjust the quantity here
	if b.Available.Compare(trade.Quantity) < 0 {
		log.Infof("adding to pending base transfer: %s %s", trade.Quantity, currency)
		s.State.PendingBaseTransfer = s.State.PendingBaseTransfer.Add(trade.Quantity)
		return nil
	}

	amount := s.State.PendingBaseTransfer.Add(trade.Quantity)

	pos := s.SpotPosition.GetBase()
	rest := pos.Sub(s.State.TotalBaseTransfer)

	if rest.Sign() < 0 {
		return nil
	}

	amount = fixedpoint.Min(rest, amount)

	log.Infof("transfering out futures account asset %s %s", amount, currency)
	if err := ex.TransferFuturesAccountAsset(ctx, currency, amount, types.TransferOut); err != nil {
		return err
	}

	// reset pending transfer
	s.State.PendingBaseTransfer = fixedpoint.Zero

	// record the transfer in the total base transfer
	s.State.TotalBaseTransfer = s.State.TotalBaseTransfer.Add(amount)
	return nil
}

func (s *Strategy) transferIn(ctx context.Context, ex FuturesTransfer, trade types.Trade) error {
	currency := s.spotMarket.BaseCurrency

	// base asset needs BUY trades
	if trade.Side == types.SideTypeSell {
		return nil
	}

	balances, err := ex.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	b, ok := balances[currency]
	if !ok {
		return fmt.Errorf("%s balance not found", currency)
	}

	// TODO: according to the fee, we might not be able to get enough balance greater than the trade quantity, we can adjust the quantity here
	if b.Available.Compare(trade.Quantity) < 0 {
		log.Infof("adding to pending base transfer: %s %s", trade.Quantity, currency)
		s.State.PendingBaseTransfer = s.State.PendingBaseTransfer.Add(trade.Quantity)
		return nil
	}

	amount := s.State.PendingBaseTransfer.Add(trade.Quantity)

	pos := s.SpotPosition.GetBase()
	rest := pos.Sub(s.State.TotalBaseTransfer)

	if rest.Sign() < 0 {
		return nil
	}

	amount = fixedpoint.Min(rest, amount)

	log.Infof("transfering in futures account asset %s %s", amount, currency)
	if err := ex.TransferFuturesAccountAsset(ctx, currency, amount, types.TransferIn); err != nil {
		return err
	}

	// reset pending transfer
	s.State.PendingBaseTransfer = fixedpoint.Zero

	// record the transfer in the total base transfer
	s.State.TotalBaseTransfer = s.State.TotalBaseTransfer.Add(amount)
	return nil
}
