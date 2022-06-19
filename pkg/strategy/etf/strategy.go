package etf

import (
	"context"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/fixedpoint"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "etf"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Market types.Market

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
	if s.TotalAmount.IsZero() {
		return errors.New("amount can not be empty")
	}

	return nil
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	go func() {
		ticker := time.NewTicker(s.Duration.Duration())
		defer ticker.Stop()

		bbgo.Notify("ETF orders will be executed every %s", s.Duration.Duration().String())

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
						bbgo.Notify("query ticker error: %s", err.Error())
						log.WithError(err).Error("query ticker error")
						break
					}

					askPrice := ticker.Sell
					quantity := askPrice.Div(amount)

					// execute orders
					quoteBalance, ok := session.GetAccount().Balance(s.Market.QuoteCurrency)
					if !ok {
						break
					}
					if quoteBalance.Available.Compare(amount) < 0 {
						bbgo.Notify("Quote balance %s is not enough: %s < %s", s.Market.QuoteCurrency, quoteBalance.Available.String(), amount.String())
						break
					}

					bbgo.Notify("Submitting etf order %s quantity %s at price %s (index ratio %s)",
						symbol,
						quantity.String(),
						askPrice.String(),
						ratio.Percentage())
					_, err = orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
						Symbol:   symbol,
						Side:     types.SideTypeBuy,
						Type:     types.OrderTypeMarket,
						Quantity: quantity,
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
