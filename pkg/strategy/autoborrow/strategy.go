package autoborrow

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/binance"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "autoborrow"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

/**
- on: binance
  autoborrow:
    interval: 30m
    repayWhenDeposit: true

    # minMarginRatio for triggering auto borrow
    minMarginRatio: 1.5
    assets:
    - asset: ETH
      low: 3.0
      maxQuantityPerBorrow: 1.0
      maxTotalBorrow: 10.0
    - asset: USDT
      low: 1000.0
      maxQuantityPerBorrow: 100.0
      maxTotalBorrow: 10.0
*/

type MarginAsset struct {
	Asset                string           `json:"asset"`
	Low                  fixedpoint.Value `json:"low"`
	MaxTotalBorrow       fixedpoint.Value `json:"maxTotalBorrow"`
	MaxQuantityPerBorrow fixedpoint.Value `json:"maxQuantityPerBorrow"`
	MinQuantityPerBorrow fixedpoint.Value `json:"minQuantityPerBorrow"`
}

type Strategy struct {
	*bbgo.Notifiability

	Interval             types.Interval   `json:"interval"`
	MinMarginRatio       fixedpoint.Value `json:"minMarginRatio"`
	MaxMarginRatio       fixedpoint.Value `json:"maxMarginRatio"`
	AutoRepayWhenDeposit bool             `json:"autoRepayWhenDeposit"`

	Assets []MarginAsset `json:"assets"`

	ExchangeSession *bbgo.ExchangeSession

	marginBorrowRepay types.MarginBorrowRepay
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) checkAndBorrow(ctx context.Context) {
	if s.MinMarginRatio.IsZero() {
		return
	}

	if err := s.ExchangeSession.UpdateAccount(ctx); err != nil {
		log.WithError(err).Errorf("can not update account")
		return
	}

	minMarginRatio := s.MinMarginRatio
	curMarginRatio := s.ExchangeSession.GetAccount().MarginRatio

	// if margin ratio is too low, do not borrow
	if curMarginRatio.Compare(minMarginRatio) < 0 {
		log.Infof("current margin ratio %f < min margin ratio %f, skip autoborrow", curMarginRatio.Float64(), minMarginRatio.Float64())
		return
	}


	balances := s.ExchangeSession.GetAccount().Balances()
	if len(balances) == 0 {
		log.Warn("balance is empty, skip autoborrow")
		return
	}

	for _, marginAsset := range s.Assets {
		if marginAsset.Low.IsZero() {
			log.Warnf("margin asset low balance is not set: %+v", marginAsset)
			continue
		}

		b, ok := balances[marginAsset.Asset]
		if ok {
			toBorrow := marginAsset.Low.Sub(b.Total())
			if toBorrow.Sign() < 0 {
				log.Debugf("balance %f > low %f. no need to borrow asset %+v",
					b.Total().Float64(),
					marginAsset.Low.Float64(),
					marginAsset)
				continue
			}

			if !marginAsset.MaxQuantityPerBorrow.IsZero() {
				toBorrow = fixedpoint.Min(toBorrow, marginAsset.MaxQuantityPerBorrow)
			}

			if !marginAsset.MaxTotalBorrow.IsZero() {
				// check if we over borrow
				if toBorrow.Add(b.Borrowed).Compare(marginAsset.MaxTotalBorrow) > 0 {
					toBorrow = toBorrow.Sub(toBorrow.Add(b.Borrowed).Sub(marginAsset.MaxTotalBorrow))
					if toBorrow.Sign() < 0 {
						log.Warnf("margin asset %s is over borrowed, skip", marginAsset.Asset)
						continue
					}
				}
				toBorrow = fixedpoint.Min(toBorrow.Add(b.Borrowed), marginAsset.MaxTotalBorrow)
			}

			s.Notifiability.Notify(&MarginAction{
				Action:         "Borrow",
				Asset:          marginAsset.Asset,
				Amount:         toBorrow,
				MarginRatio:    curMarginRatio,
				MinMarginRatio: minMarginRatio,
			})
			log.Infof("sending borrow request %f %s", toBorrow.Float64(), marginAsset.Asset)
			s.marginBorrowRepay.BorrowMarginAsset(ctx, marginAsset.Asset, toBorrow)
		} else {
			// available balance is less than marginAsset.Low, we should trigger borrow
			toBorrow := marginAsset.Low

			if !marginAsset.MaxQuantityPerBorrow.IsZero() {
				toBorrow = fixedpoint.Min(toBorrow, marginAsset.MaxQuantityPerBorrow)
			}

			s.Notifiability.Notify(&MarginAction{
				Action:         "Borrow",
				Asset:          marginAsset.Asset,
				Amount:         toBorrow,
				MarginRatio:    curMarginRatio,
				MinMarginRatio: minMarginRatio,
			})

			log.Infof("sending borrow request %f %s", toBorrow.Float64(), marginAsset.Asset)
			s.marginBorrowRepay.BorrowMarginAsset(ctx, marginAsset.Asset, toBorrow)
		}
	}
}

func (s *Strategy) run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.checkAndBorrow(ctx)
	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.checkAndBorrow(ctx)

		}
	}
}

func (s *Strategy) handleBalanceUpdate(balances types.BalanceMap) {
	if s.MinMarginRatio.IsZero() {
		return
	}

	if s.ExchangeSession.GetAccount().MarginRatio.Compare(s.MinMarginRatio) > 0 {
		return
	}

	for _, b := range balances {
		if b.Available.IsZero() && b.Borrowed.IsZero() {
			continue
		}
	}
}

func (s *Strategy) handleBinanceBalanceUpdateEvent(event *binance.BalanceUpdateEvent) {
	if s.MinMarginRatio.IsZero() {
		return
	}

	if s.ExchangeSession.GetAccount().MarginRatio.Compare(s.MinMarginRatio) > 0 {
		return
	}

	delta := fixedpoint.MustNewFromString(event.Delta)

	// ignore outflow
	if delta.Sign() < 0 {
		return
	}

	minMarginRatio := s.MinMarginRatio
	curMarginRatio := s.ExchangeSession.GetAccount().MarginRatio

	if b, ok := s.ExchangeSession.GetAccount().Balance(event.Asset); ok {
		if b.Available.IsZero() || b.Borrowed.IsZero() {
			return
		}

		toRepay := b.Available
		s.Notifiability.Notify(&MarginAction{
			Action:         "Borrow",
			Asset:          b.Currency,
			Amount:         toRepay,
			MarginRatio:    curMarginRatio,
			MinMarginRatio: minMarginRatio,
		})
		if err := s.marginBorrowRepay.RepayMarginAsset(context.Background(), event.Asset, toRepay); err != nil {
			log.WithError(err).Errorf("margin repay error")
		}
	}
}

type MarginAction struct {
	Action         string
	Asset          string
	Amount         fixedpoint.Value
	MarginRatio    fixedpoint.Value
	MinMarginRatio fixedpoint.Value
}

func (a *MarginAction) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Title: fmt.Sprintf("%s %s %s", a.Action, a.Amount, a.Asset),
		Color: "warning",
		Fields: []slack.AttachmentField{
			{
				Title: "Action",
				Value: a.Action,
				Short: true,
			},
			{
				Title: "Asset",
				Value: a.Asset,
				Short: true,
			},
			{
				Title: "Amount",
				Value: a.Amount.String(),
				Short: true,
			},
			{
				Title: "Current Margin Ratio",
				Value: a.MarginRatio.String(),
				Short: true,
			},
			{
				Title: "Min Margin Ratio",
				Value: a.MinMarginRatio.String(),
				Short: true,
			},
		},
	}
}

// This strategy simply spent all available quote currency to buy the symbol whenever kline gets closed
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.MinMarginRatio.IsZero() {
		log.Warnf("minMarginRatio is 0, you should configure this minimal margin ratio for controlling the liquidation risk")
	}

	s.ExchangeSession = session

	marginBorrowRepay, ok := session.Exchange.(types.MarginBorrowRepay)
	if !ok {
		return fmt.Errorf("exchange %s does not implement types.MarginBorrowRepay", session.ExchangeName)
	}

	s.marginBorrowRepay = marginBorrowRepay

	if s.AutoRepayWhenDeposit {
		binanceStream, ok := session.UserDataStream.(*binance.Stream)
		if ok {
			binanceStream.OnBalanceUpdateEvent(s.handleBinanceBalanceUpdateEvent)
		} else {
			session.UserDataStream.OnBalanceUpdate(s.handleBalanceUpdate)
		}
	}

	go s.run(ctx, s.Interval.Duration())
	return nil
}
