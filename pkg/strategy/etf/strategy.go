package etf

import (
	"context"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "etf"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Market types.Market

	Notifiability *bbgo.Notifiability


	TotalAmount fixedpoint.Value `json:"totalAmount,omitempty"`

	// Interval is the period that you want to submit order
	Duration types.Duration `json:"duration"`

	Index map[string]fixedpoint.Value `json:"index"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
}

func (s *Strategy) Validate() error {
	if s.TotalAmount == 0 {
		return errors.New("amount can not be empty")
	}

	return nil
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	go func() {
		ticker := time.NewTicker(s.Duration.Duration())
		defer ticker.Stop()

		s.Notifiability.Notify("ETF orders will be executed every %s", s.Duration.Duration().String())

		for {
			select {
			case <-ctx.Done():
				return

			case <-ticker.C:
				totalAmount := s.TotalAmount
				for symbol, ratio := range s.Index {
					amount := totalAmount.Mul(ratio)

					ticker, err := session.Exchange.QueryTicker(ctx, symbol)
					if err != nil {
						log.WithError(err).Error("query ticker error")
					}

					askPrice := fixedpoint.NewFromFloat(ticker.Sell)
					quantity := askPrice.Div(amount)

					// execute orders
					quoteBalance, ok := session.Account.Balance(s.Market.QuoteCurrency)
					if !ok {
						return
					}
					if quoteBalance.Available < amount {
						s.Notifiability.Notify("Quote balance %s is not enough: %f < %f", s.Market.QuoteCurrency, quoteBalance.Available.Float64(), amount.Float64())
						return
					}

					s.Notifiability.Notify("Submitting etf order %s quantity %f at price %f (index ratio %f %%)",
						symbol,
						quantity.Float64(),
						askPrice.Float64(),
						ratio.Float64()*100.0)
					_, err = orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
						Symbol:   symbol,
						Side:     types.SideTypeBuy,
						Type:     types.OrderTypeMarket,
						Quantity: quantity.Float64(),
					})

					if err != nil {
						log.WithError(err).Error("submit order error")
					}

				}
			}
		}
	}()

	return nil
}
