package dca2

import (
	"context"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/retry"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util/tradingutil"
)

func (s *Strategy) SetupPriceTriggerMode(ctx context.Context) {
	// register state transition functions
	s.stateMachine.RegisterTransitionFunc(IdleWaiting, OpenPositionReady, s.openPosition)
	s.stateMachine.RegisterTransitionFunc(OpenPositionReady, OpenPositionOrderFilled, s.readyToFinishOpenPositionStage)
	s.stateMachine.RegisterTransitionFunc(OpenPositionOrderFilled, OpenPositionFinished, s.finishOpenPositionStage)
	s.stateMachine.RegisterTransitionFunc(OpenPositionFinished, TakeProfitReady, s.cancelOpenPositionOrdersAndPlaceTakeProfitOrder)
	s.stateMachine.RegisterTransitionFunc(TakeProfitReady, IdleWaiting, s.finishTakeProfitStage)

	// state machine start callback
	s.stateMachine.OnStart(func() {
		var err error
		maxTry := 3
		for try := 1; try <= maxTry; try++ {
			s.logger.Infof("try #%d recover", try)
			err = s.recover(ctx)
			if err == nil {
				break
			}

			s.logger.WithError(err).Warnf("failed to recover at #%d", try)
			// sleep 10 second to retry the recovery
			time.Sleep(10 * time.Second)
		}

		// if recovery failed after maxTry attempts, the state in state machine will be None and there is no transition function will be triggered
		if err != nil {
			s.logger.WithError(err).Errorf("failed to recover after %d attempts, please check it", maxTry)
		}

		s.updateTakeProfitPrice()

		if s.stateMachine.GetState() == IdleWaiting {
			s.stateMachine.EmitNextState(OpenPositionReady)
		}
	})

	// state machine close callback
	s.stateMachine.OnClose(func() {
		s.logger.Infof("state machine closed, will cancel all orders")

		var err error
		if s.UseCancelAllOrdersApiWhenClose {
			err = tradingutil.UniversalCancelAllOrders(ctx, s.ExchangeSession.Exchange, s.Symbol, nil)
		} else {
			err = s.OrderExecutor.GracefulCancel(ctx)
		}

		if err != nil {
			s.logger.WithError(err).Errorf("failed to cancel all orders when closing, please check it")
		}

		bbgo.Sync(ctx, s)
	})

	// order filled callback
	s.OrderExecutor.ActiveMakerOrders().OnFilled(func(o types.Order) {
		s.logger.Infof("FILLED ORDER: %s", o.String())

		switch o.Side {
		case OpenPositionSide:
			s.stateMachine.EmitNextState(OpenPositionOrderFilled)
		case TakeProfitSide:
			s.stateMachine.EmitNextState(IdleWaiting)
		default:
			s.logger.Infof("unsupported side (%s) of order: %s", o.Side, o)
		}

		openOrders, err := retry.QueryOpenOrdersUntilSuccessful(ctx, s.ExchangeSession.Exchange, s.Symbol)
		if err != nil {
			s.logger.WithError(err).Warn("failed to query open orders when order filled")
			return
		}

		// update active orders metrics
		numActiveMakerOrders := s.OrderExecutor.ActiveMakerOrders().NumOfOrders()
		updateNumOfActiveOrdersMetrics(numActiveMakerOrders)

		if len(openOrders) != numActiveMakerOrders {
			s.logger.Warnf("num of open orders (%d) and active orders (%d) is different when order filled, please check it.", len(openOrders), numActiveMakerOrders)
			return
		}

		if o.Side == OpenPositionSide && numActiveMakerOrders == 0 {
			s.stateMachine.EmitNextState(OpenPositionFinished)
		}
	})

	// position update callback
	s.OrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		s.logger.Infof("POSITION UPDATE: %s", s.Position.String())
		bbgo.Sync(ctx, s)

		// update take profit price here
		s.updateTakeProfitPrice()

		// emit position update
		s.EmitPositionUpdate(position)
	})

	// kline callback
	s.ExchangeSession.MarketDataStream.OnKLine(func(kline types.KLine) {
		if s.takeProfitPrice.IsZero() {
			s.logger.Warn("take profit price should not be 0 when there is at least one open-position order filled, please check it")
			return
		}

		if s.stateMachine.GetState() != OpenPositionOrderFilled {
			return
		}

		if kline.Close.Compare(s.takeProfitPrice) >= 0 {
			s.logger.Infof("take profit price (%s) reached, will emit next state", s.takeProfitPrice.String())
			s.stateMachine.EmitNextState(OpenPositionFinished)
		}
	})

	// trigger state transitions periodically
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				s.logger.Info("context done, exiting dca2 strategy")
				return
			case <-ticker.C:
				switch s.stateMachine.GetState() {
				case IdleWaiting:
					s.stateMachine.EmitNextState(OpenPositionReady)
				case OpenPositionFinished:
					s.stateMachine.EmitNextState(TakeProfitReady)
				}
			}
		}
	}()
}
