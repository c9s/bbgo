package deposit2transfer

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "deposit2transfer"

var log = logrus.WithField("strategy", ID)

var errNotBinanceExchange = errors.New("not binance exchange, currently only support binance exchange")

func init() {
	// Register the pointer of the strategy struct,
	// so that bbgo knows what struct to be used to unmarshal the configs (YAML or JSON)
	// Note: built-in strategies need to imported manually in the bbgo cmd package.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment

	Interval types.Interval `json:"interval"`

	SpotSession    string `json:"spotSession"`
	FuturesSession string `json:"futuresSession"`

	spotSession, futuresSession *bbgo.ExchangeSession

	binanceFutures, binanceSpot *binance.Exchange
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {}

func (s *Strategy) Defaults() error {
	if s.Interval == "" {
		s.Interval = types.Interval1m
	}

	return nil
}

func (s *Strategy) Validate() error {
	if len(s.SpotSession) == 0 {
		return errors.New("spotSession name is required")
	}

	if len(s.FuturesSession) == 0 {
		return errors.New("futuresSession name is required")
	}

	return nil
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s", ID)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	var ok bool
	s.binanceFutures, ok = session.Exchange.(*binance.Exchange)
	if !ok {
		return errNotBinanceExchange
	}

	s.binanceSpot, ok = session.Exchange.(*binance.Exchange)
	if !ok {
		return errNotBinanceExchange
	}

	// instanceID := s.InstanceID()
	/*
		if err := s.transferIn(ctx, s.binanceSpot, s.spotMarket.BaseCurrency, fixedpoint.Zero); err != nil {
			log.WithError(err).Errorf("futures asset transfer in error")
		}

		if err := s.transferOut(ctx, s.binanceSpot, s.spotMarket.BaseCurrency, fixedpoint.Zero); err != nil {
			log.WithError(err).Errorf("futures asset transfer out error")
		}

		if err := backoff.RetryGeneral(ctx, func() error {
			return s.transferIn(ctx, s.binanceSpot, s.spotMarket.BaseCurrency, trade.Quantity)
		}); err != nil {
			log.WithError(err).Errorf("spot-to-futures transfer in retry failed")
			return err
		}

		if err := backoff.RetryGeneral(ctx, func() error {
			return s.transferOut(ctx, s.binanceSpot, s.spotMarket.BaseCurrency, quantity)
		}); err != nil {
			log.WithError(err).Errorf("spot-to-futures transfer in retry failed")
			return err
		}
	*/

	if binanceStream, ok := s.futuresSession.UserDataStream.(*binance.Stream); ok {
		binanceStream.OnAccountUpdateEvent(func(e *binance.AccountUpdateEvent) {
			s.handleAccountUpdate(ctx, e)
		})
	}

	return nil
}

func (s *Strategy) handleAccountUpdate(ctx context.Context, e *binance.AccountUpdateEvent) {
	switch e.AccountUpdate.EventReasonType {
	case binance.AccountUpdateEventReasonDeposit:
	case binance.AccountUpdateEventReasonWithdraw:
	case binance.AccountUpdateEventReasonFundingFee:
		//  EventBase:{
		// 		Event:ACCOUNT_UPDATE
		// 		Time:1679760000932
		// 	}
		// 	Transaction:1679760000927
		// 	AccountUpdate:{
		// 			EventReasonType:FUNDING_FEE
		// 			Balances:[{
		// 					Asset:USDT
		// 					WalletBalance:56.64251742
		// 					CrossWalletBalance:56.64251742
		// 					BalanceChange:-0.00037648
		// 			}]
		// 		}
		// 	}
	}
}
