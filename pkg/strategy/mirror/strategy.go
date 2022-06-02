package mirror

import (
	"context"
	"fmt"
	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

const ID = "mirror"

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

	log.Info(sessions)

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

	return nil
}
