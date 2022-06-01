package copytrader

import (
	"context"
	"fmt"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

const ID = "copytrader"

const stateKey = "state-v1"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	*bbgo.Graceful
	*bbgo.Notifiability
	*bbgo.Persistence
	Environment *bbgo.Environment

	Symbol string `json:"symbol"`

	Exchange string `json:"exchange,omitempty"`

	SourceSession   *bbgo.ExchangeSession
	FollowerSession map[string]*bbgo.ExchangeSession

	FollowerMakerOrders map[string]*bbgo.LocalActiveOrderBook

	Market types.Market
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	//log.Info(sessions)

	//sourceSession, ok := sessions["binance-master"]
	//if !ok {
	//	panic(fmt.Errorf("source session %s is not defined", sourceSession.Name))
	//} else {
	//	sourceSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	//	//sourceSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
	//}

	//s.SourceSession = sourceSession
	//
	////s.FollowerSession = make(map[string]*bbgo.ExchangeSession, len(sessions)-1)
	//s.FollowerMakerOrders = make(map[string]*bbgo.LocalActiveOrderBook, len(sessions)-1)
	//for k, v := range sessions {
	//	// do not follower yourself
	//	if k != "binance-master" {
	//		followerSession, ok := sessions[k]
	//		if !ok {
	//			panic(fmt.Errorf("follower session %s is not defined", followerSession.Name))
	//		} else {
	//			//followerSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	//			//followerSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
	//			//s.FollowerSession[k] = followerSession
	//
	//			s.FollowerMakerOrders[k] = bbgo.NewLocalActiveOrderBook(s.Symbol)
	//			s.FollowerMakerOrders[k].BindStream(sessions[k].UserDataStream)
	//
	//			log.Infof("subscribe follower session %s: %s, from env var: %s", k, v.Name, v.EnvVarPrefix)
	//		}
	//		//if !ok {
	//		//	panic(fmt.Errorf("follower session %s is not defined", v))
	//		//}
	//		//s.FollowerSession[k].Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	//	}
	//}
	//for k, v := range sessions {
	//	// do not follower yourself
	//	if k != "binance-master" {
	//		s.FollowerSession[k] = sessions[k]
	//		log.Infof("subscribe follower session %s: %s, from env var: %s", k, v.Name, v.EnvVarPrefix)
	//		//if !ok {
	//		//	panic(fmt.Errorf("follower session %s is not defined", v))
	//		//}
	//		//s.FollowerSession[k].Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	//	}
	//}

	//s.SourceSession = sessions["binance-master"]
	////if !ok {
	////	panic(fmt.Errorf("source session %s is not defined", s.SourceExchange))
	////}
	//
	//s.SourceSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
	//s.SourceSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	//
	//for k, v := range sessions {
	//	// do not follower yourself
	//	if k != "binance-master" {
	//		s.FollowerSession[k] = sessions[k]
	//		log.Infof("subscribe follower session %s: %s, from env var: %s", k, v.Name, v.EnvVarPrefix)
	//		//if !ok {
	//		//	panic(fmt.Errorf("follower session %s is not defined", v))
	//		//}
	//		s.FollowerSession[k].Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	//	}
	//}

}

func (s *Strategy) CrossRun(ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	//_ = s.Persistence.Sync(s)
	// configure sessions
	//sourceSession, ok := sessions[s.SourceExchange]
	//if !ok {
	//	return fmt.Errorf("source exchange session %s is not defined", s.SourceExchange)
	//}

	//s.sourceSession = sourceSession

	//for k, v := range s.FollowerExchange {
	//	followerSession, ok := sessions[k]
	//	if !ok {
	//		panic(fmt.Errorf("maker exchange session %s is not defined", v))
	//	}
	//	s.followerSession[k] = followerSession
	//
	//}

	log.Info(sessions)

	//sourceSession, _ := sessions["binance-master"]
	//s.SourceSession = sourceSession
	//
	//for k, v := range sessions {
	//	// do not follower yourself
	//	if k != "binance-master" {
	//		s.FollowerSession[k], _ = sessions[k]
	//		log.Infof("subscribe follower session %s: %s, from env var: %s", k, v.Name, v.EnvVarPrefix)
	//		//if !ok {
	//		//	panic(fmt.Errorf("follower session %s is not defined", v))
	//		//}
	//		//s.FollowerSession[k].Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
	//	}
	//}

	//if !ok {
	//	return fmt.Errorf("source session market %s is not defined", s.Symbol)
	//}

	//s.followerMarket, ok = s.sourceSession.Market(s.Symbol)
	//if !ok {
	//	return fmt.Errorf("maker session market %s is not defined", s.Symbol)
	//}

	// restore state
	//instanceID := s.InstanceID()
	//s.groupID = util.FNV32(instanceID)
	//log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	//s.book = types.NewStreamBook(s.Symbol)
	//s.book.BindStream(s.sourceSession.MarketDataStream)

	//for k, _ := range s.FollowerExchange {
	//	s.activeFollowerOrders[k] = bbgo.NewLocalActiveOrderBook(s.Symbol)
	//	s.activeFollowerOrders[k].BindStream(s.followerSession[k].UserDataStream)
	//	s.followerOrderStore[k] = bbgo.NewOrderStore(s.Symbol)
	//	s.followerOrderStore[k].BindStream(s.followerSession[k].UserDataStream)
	//}
	//
	//s.sourceOrderStore = bbgo.NewOrderStore(s.Symbol)
	//s.sourceOrderStore.BindStream(s.sourceSession.UserDataStream)
	// If position is nil, we need to allocate a new position for calculation
	//if s.Position == nil {
	//	s.Position = types.NewPositionFromMarket(s.Market)
	//}

	//log.Infof("===")
	//log.Info(s.SourceSession)
	//log.Infof("===")
	//log.Info(s.FollowerSession)
	sourceSession, ok := sessions["binance-master"]
	if !ok {
		panic(fmt.Errorf("source session %s is not defined", sourceSession.Name))
	}
	s.Market, _ = sourceSession.Market(s.Symbol)

	s.FollowerMakerOrders = make(map[string]*bbgo.LocalActiveOrderBook)
	for k, _ := range sessions {
		s.FollowerMakerOrders[k] = bbgo.NewLocalActiveOrderBook(s.Symbol)
		s.FollowerMakerOrders[k].BindStream(sessions[k].UserDataStream)
	}

	//go func() {
	//cnt := 0
	//log.Info(sourceSession)

	sourceSession.UserDataStream.OnOrderUpdate(func(order types.Order) {
		utility := fixedpoint.Zero
		account := sourceSession.GetAccount()
		if quote, ok := account.Balance(s.Market.QuoteCurrency); ok {
			utility = order.Quantity.Mul(order.Price).Div(quote.Available)
		}
		log.Infof("source order: %v", order)
		log.Info(utility)
		if order.Status == types.OrderStatusNew {

			for k, v := range sessions {
				//log.Error(cnt)
				if k != sourceSession.Name {
					// cancel all follower's open orders from source
					//if err := s.FollowerMakerOrders[k].GracefulCancel(context.Background(),
					//	sessions[k].Exchange); err != nil {
					//	log.WithError(err).Errorf("can not cancel %s orders", s.Symbol)
					//}
					//log.Infof("submitted order: %s for follower index key %s, %s", copyOrder, k, sessions[k].Name)
					//cnt++
					followerAccount := v.GetAccount()
					orderAmount := fixedpoint.Zero
					if followerQuote, ok := followerAccount.Balance(s.Market.QuoteCurrency); ok {
						orderAmount = followerQuote.Available.Mul(utility)
					}
					copyOrder := types.SubmitOrder{
						Symbol:   order.Symbol,
						Side:     order.Side,
						Price:    order.Price,
						Type:     order.Type,
						Quantity: orderAmount.Div(order.Price),
					}
					log.Infof("copy order: %s", copyOrder)

					//createdOrders, err := orderExecutionRouter.SubmitOrdersTo(ctx, sessions[k].Name, copyOrder)
					//// createdOrders, err := v.Exchange.SubmitOrders(ctx, copyOrder)
					//if err != nil {
					//	log.WithError(err).Errorf("can not place order")
					//} else {
					//	s.FollowerMakerOrders[k].Add(createdOrders...)
					//	log.Infof("submit order success: %s for follower index key %s", createdOrders, k)
					//}
				}
			}
		} else if order.Status == types.OrderStatusCanceled {
			for k, _ := range sessions {
				//log.Error(cnt)
				if k != sourceSession.Name {
					// cancel all follower's open orders from source
					if err := s.FollowerMakerOrders[k].GracefulCancel(context.Background(),
						sessions[k].Exchange); err != nil {
						log.WithError(err).Errorf("can not cancel %s orders", s.Symbol)
					} else {
						log.Infof("cancel order success: %d for follower index key %s", order.OrderID, k)
					}
				}
			}
		}
	})
	//}()

	//s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.Position, s.orderStore)

	//if s.NotifyTrade {
	//	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
	//		s.Notifiability.Notify(trade)
	//	})
	//}

	//s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
	//	c := trade.PositionChange()
	//	if trade.Exchange == s.sourceSession.ExchangeName {
	//		s.CoveredPosition = s.CoveredPosition.Add(c)
	//	}
	//
	//	s.ProfitStats.AddTrade(trade)
	//
	//	if profit.Compare(fixedpoint.Zero) == 0 {
	//		s.Environment.RecordPosition(s.Position, trade, nil)
	//	} else {
	//		log.Infof("%s generated profit: %v", s.Symbol, profit)
	//
	//		p := s.Position.NewProfit(trade, profit, netProfit)
	//		p.Strategy = ID
	//		p.StrategyInstanceID = instanceID
	//		s.Notify(&p)
	//		s.ProfitStats.AddProfit(p)
	//
	//		s.Environment.RecordPosition(s.Position, trade, &p)
	//	}
	//})

	//s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
	//	s.Notifiability.Notify(position)
	//})
	//s.tradeCollector.OnRecover(func(trade types.Trade) {
	//	s.Notifiability.Notify("Recover trade", trade)
	//})
	//s.tradeCollector.BindStream(s.sourceSession.UserDataStream)
	//s.tradeCollector.BindStream(s.makerSession.UserDataStream)

	//go func() {

	//defer func() {
	//	if err := s.activeFollowerOrders.GracefulCancel(context.Background(),
	//		s.makerSession.Exchange); err != nil {
	//		log.WithError(err).Errorf("can not cancel %s orders", s.Symbol)
	//	}
	//}()

	//	for {
	//		select {
	//
	//		//case <-s.stopC:
	//		//	log.Warnf("%s maker goroutine stopped, due to the stop signal", s.Symbol)
	//		//	return
	//
	//		case <-ctx.Done():
	//			log.Warnf("%s maker goroutine stopped, due to the cancelled context", s.Symbol)
	//			return
	//
	//		case <-quoteTicker.C:
	//			s.updateQuote(ctx, orderExecutionRouter)
	//
	//		case <-reportTicker.C:
	//			s.Notifiability.Notify(&s.ProfitStats)
	//
	//		case <-tradeScanTicker.C:
	//			log.Infof("scanning trades from %s ago...", tradeScanInterval)
	//			startTime := time.Now().Add(-tradeScanInterval)
	//			if err := s.tradeCollector.Recover(ctx, s.sourceSession.Exchange.(types.ExchangeTradeHistoryService), s.Symbol, startTime); err != nil {
	//				log.WithError(err).Errorf("query trades error")
	//			}
	//
	//		case <-posTicker.C:
	//			// For positive position and positive covered position:
	//			// uncover position = +5 - +3 (covered position) = 2
	//			//
	//			// For positive position and negative covered position:
	//			// uncover position = +5 - (-3) (covered position) = 8
	//			//
	//			// meaning we bought 5 on MAX and sent buy order with 3 on binance
	//			//
	//			// For negative position:
	//			// uncover position = -5 - -3 (covered position) = -2
	//			s.tradeCollector.Process()
	//
	//			position := s.Position.GetBase()
	//
	//			uncoverPosition := position.Sub(s.CoveredPosition)
	//			absPos := uncoverPosition.Abs()
	//			if !s.DisableHedge && absPos.Compare(s.sourceMarket.MinQuantity) > 0 {
	//				log.Infof("%s base position %v coveredPosition: %v uncoverPosition: %v",
	//					s.Symbol,
	//					position,
	//					s.CoveredPosition,
	//					uncoverPosition,
	//				)
	//
	//				s.Hedge(ctx, uncoverPosition.Neg())
	//			}
	//		}
	//	}
	//}()

	//s.Graceful.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
	//	defer wg.Done()
	//
	//	close(s.stopC)
	//
	//	// wait for the quoter to stop
	//	time.Sleep(s.UpdateInterval.Duration())
	//
	//	shutdownCtx, cancelShutdown := context.WithTimeout(context.TODO(), time.Minute)
	//	defer cancelShutdown()
	//
	//	if err := s.activeMakerOrders.GracefulCancel(shutdownCtx, s.makerSession.Exchange); err != nil {
	//		log.WithError(err).Errorf("graceful cancel error")
	//	}
	//
	//	s.Notify("%s: %s position", ID, s.Symbol, s.Position)
	//})

	return nil
}
