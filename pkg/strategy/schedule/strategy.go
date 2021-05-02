package schedule

import (
	"context"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "schedule"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Market types.Market
	Notifiability *bbgo.Notifiability

	// Interval is the period that you want to submit order
	Interval types.Interval `json:"interval"`

	// Symbol is the symbol of the market
	Symbol string `json:"symbol"`

	// Side is the order side type, which can be buy or sell
	Side types.SideType `json:"side"`

	// Quantity is the quantity of the submit order
	Quantity fixedpoint.Value `json:"quantity,omitempty"`

	Amount fixedpoint.Value `json:"amount,omitempty"`

}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval.String()})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.Stream.OnKLineClosed(func(kline types.KLine) {
		if kline.Symbol != s.Symbol {
			return
		}


		closePrice := kline.Close
		quantity := s.Quantity.Float64()
		if s.Amount > 0 {
			quantity = s.Amount.Float64() / closePrice
		}
		quoteQuantity := quantity * closePrice

		switch s.Side {
		case types.SideTypeBuy:
			quoteBalance, ok := session.Account.Balance(s.Market.QuoteCurrency)
			if !ok {
				return
			}
			if quoteBalance.Available.Float64() < quoteQuantity {
				s.Notifiability.Notify("quote balance %s is not enough: %f < %f", s.Market.QuoteCurrency, quoteBalance.Available.Float64(), quoteQuantity)
				return
			}

		case types.SideTypeSell:
			baseBalance, ok := session.Account.Balance(s.Market.BaseCurrency)
			if !ok {
				return
			}
			if baseBalance.Available.Float64() < quantity {
				s.Notifiability.Notify("base balance %s is not enough: %f < %f", s.Market.QuoteCurrency, baseBalance.Available.Float64(), quantity)
				return
			}

		}

		_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:   s.Symbol,
			Side:     s.Side,
			Type:     types.OrderTypeMarket,
			Quantity: quantity,
		})

		if err != nil {
			log.WithError(err).Error("submit order error")
		}
	})

	return nil
}
