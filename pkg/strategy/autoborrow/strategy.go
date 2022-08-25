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

    # minMarginLevel for triggering auto borrow
    minMarginLevel: 1.5
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
	MinDebtRatio         fixedpoint.Value `json:"debtRatio"`
}

type Strategy struct {
	Interval             types.Interval   `json:"interval"`
	MinMarginLevel       fixedpoint.Value `json:"minMarginLevel"`
	MaxMarginLevel       fixedpoint.Value `json:"maxMarginLevel"`
	AutoRepayWhenDeposit bool             `json:"autoRepayWhenDeposit"`

	Assets []MarginAsset `json:"assets"`

	ExchangeSession *bbgo.ExchangeSession

	marginBorrowRepay types.MarginBorrowRepayService
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) tryToRepayAnyDebt(ctx context.Context) {
	log.Infof("trying to repay any debt...")

	account, err := s.ExchangeSession.UpdateAccount(ctx)
	if err != nil {
		log.WithError(err).Errorf("can not update account")
		return
	}

	minMarginLevel := s.MinMarginLevel
	curMarginLevel := account.MarginLevel

	balances := account.Balances()
	for _, b := range balances {
		if b.Borrowed.Sign() <= 0 {
			continue
		}

		if b.Available.IsZero() {
			continue
		}

		toRepay := fixedpoint.Min(b.Available, b.Debt())
		bbgo.Notify(&MarginAction{
			Exchange:       s.ExchangeSession.ExchangeName,
			Action:         "Repay",
			Asset:          b.Currency,
			Amount:         toRepay,
			MarginLevel:    curMarginLevel,
			MinMarginLevel: minMarginLevel,
		})

		log.Infof("repaying %f %s", toRepay.Float64(), b.Currency)
		if err := s.marginBorrowRepay.RepayMarginAsset(context.Background(), b.Currency, toRepay); err != nil {
			log.WithError(err).Errorf("margin repay error")
		}
	}
}

func (s *Strategy) reBalanceDebt(ctx context.Context) {
	account, err := s.ExchangeSession.UpdateAccount(ctx)
	if err != nil {
		log.WithError(err).Errorf("can not update account")
		return
	}

	minMarginLevel := s.MinMarginLevel
	curMarginLevel := account.MarginLevel

	balances := account.Balances()
	if len(balances) == 0 {
		log.Warn("balance is empty, skip autoborrow")
		return
	}

	for _, marginAsset := range s.Assets {
		b, ok := balances[marginAsset.Asset]
		if !ok {
			continue
		}

		// debt / total
		debtRatio := b.Debt().Div(b.Total())
		if marginAsset.MinDebtRatio.IsZero() {
			marginAsset.MinDebtRatio = fixedpoint.One
		}

		if b.Total().Compare(marginAsset.Low) <= 0 {
			continue
		}

		log.Infof("checking debtRatio: session = %s asset = %s, debtRatio = %f", s.ExchangeSession.Name, marginAsset.Asset, debtRatio.Float64())

		// if debt is greater than total, skip repay
		if b.Debt().Compare(b.Total()) > 0 {
			log.Infof("%s debt %f is less than total %f", marginAsset.Asset, b.Debt().Float64(), b.Total().Float64())
			continue
		}

		// the current debt ratio is less than the minimal ratio,
		// we need to repay and reduce the debt
		if debtRatio.Compare(marginAsset.MinDebtRatio) > 0 {
			log.Infof("%s debt ratio %f is less than min debt ratio %f, skip", marginAsset.Asset, debtRatio.Float64(), marginAsset.MinDebtRatio.Float64())
			continue
		}

		toRepay := fixedpoint.Min(b.Borrowed, b.Available)
		toRepay = toRepay.Sub(marginAsset.Low)

		if toRepay.Sign() <= 0 {
			log.Warnf("%s repay amount = 0, can not repay", marginAsset.Asset)
			continue
		}

		bbgo.Notify(&MarginAction{
			Exchange:       s.ExchangeSession.ExchangeName,
			Action:         fmt.Sprintf("Repay for Debt Ratio %f", debtRatio.Float64()),
			Asset:          b.Currency,
			Amount:         toRepay,
			MarginLevel:    curMarginLevel,
			MinMarginLevel: minMarginLevel,
		})

		if err := s.marginBorrowRepay.RepayMarginAsset(context.Background(), b.Currency, toRepay); err != nil {
			log.WithError(err).Errorf("margin repay error")
		}
	}
}

func (s *Strategy) checkAndBorrow(ctx context.Context) {
	s.reBalanceDebt(ctx)

	if s.MinMarginLevel.IsZero() {
		return
	}

	account, err := s.ExchangeSession.UpdateAccount(ctx)
	if err != nil {
		log.WithError(err).Errorf("can not update account")
		return
	}

	minMarginLevel := s.MinMarginLevel
	curMarginLevel := account.MarginLevel

	log.Infof("%s: current margin level: %s, margin ratio: %s, margin tolerance: %s",
		s.ExchangeSession.Name,
		account.MarginLevel.String(),
		account.MarginRatio.String(),
		account.MarginTolerance.String(),
	)

	// if margin ratio is too low, do not borrow
	if curMarginLevel.Compare(minMarginLevel) < 0 {
		log.Infof("current margin level %f < min margin level %f, skip autoborrow", curMarginLevel.Float64(), minMarginLevel.Float64())
		bbgo.Notify("Warning!!! %s Current Margin Level %f < Minimal Margin Level %f",
			s.ExchangeSession.Name,
			curMarginLevel.Float64(),
			minMarginLevel.Float64(),
			account.Balances().Debts(),
		)
		s.tryToRepayAnyDebt(ctx)
		return
	}

	balances := account.Balances()
	if len(balances) == 0 {
		log.Warn("balance is empty, skip autoborrow")
		return
	}

	for _, marginAsset := range s.Assets {
		changed := false

		if marginAsset.Low.IsZero() {
			log.Warnf("margin asset low balance is not set: %+v", marginAsset)
			continue
		}

		b, ok := balances[marginAsset.Asset]
		if ok {
			toBorrow := marginAsset.Low.Sub(b.Total())
			if toBorrow.Sign() < 0 {
				log.Infof("balance %f > low %f. no need to borrow asset %+v",
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
				newBorrow := toBorrow.Add(b.Borrowed)
				if newBorrow.Compare(marginAsset.MaxTotalBorrow) > 0 {
					toBorrow = toBorrow.Sub(newBorrow.Sub(marginAsset.MaxTotalBorrow))
					if toBorrow.Sign() < 0 {
						log.Warnf("margin asset %s is over borrowed, skip", marginAsset.Asset)
						continue
					}
				}
			}

			if toBorrow.IsZero() {
				continue
			}

			bbgo.Notify(&MarginAction{
				Exchange:       s.ExchangeSession.ExchangeName,
				Action:         "Borrow",
				Asset:          marginAsset.Asset,
				Amount:         toBorrow,
				MarginLevel:    curMarginLevel,
				MinMarginLevel: minMarginLevel,
			})
			log.Infof("sending borrow request %f %s", toBorrow.Float64(), marginAsset.Asset)
			if err := s.marginBorrowRepay.BorrowMarginAsset(ctx, marginAsset.Asset, toBorrow); err != nil {
				log.WithError(err).Errorf("borrow error")
				continue
			}
			changed = true
		} else {
			// available balance is less than marginAsset.Low, we should trigger borrow
			toBorrow := marginAsset.Low

			if !marginAsset.MaxQuantityPerBorrow.IsZero() {
				toBorrow = fixedpoint.Min(toBorrow, marginAsset.MaxQuantityPerBorrow)
			}

			if toBorrow.IsZero() {
				continue
			}

			bbgo.Notify(&MarginAction{
				Exchange:       s.ExchangeSession.ExchangeName,
				Action:         "Borrow",
				Asset:          marginAsset.Asset,
				Amount:         toBorrow,
				MarginLevel:    curMarginLevel,
				MinMarginLevel: minMarginLevel,
			})

			log.Infof("sending borrow request %f %s", toBorrow.Float64(), marginAsset.Asset)
			if err := s.marginBorrowRepay.BorrowMarginAsset(ctx, marginAsset.Asset, toBorrow); err != nil {
				log.WithError(err).Errorf("borrow error")
				continue
			}

			changed = true
		}

		// if debt is changed, we need to update account
		if changed {
			account, err = s.ExchangeSession.UpdateAccount(ctx)
			if err != nil {
				log.WithError(err).Errorf("can not update account")
				return
			}
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
	if s.MinMarginLevel.IsZero() {
		return
	}

	if s.ExchangeSession.GetAccount().MarginLevel.Compare(s.MinMarginLevel) > 0 {
		return
	}

	for _, b := range balances {
		if b.Available.IsZero() && b.Borrowed.IsZero() {
			continue
		}
	}
}

func (s *Strategy) handleBinanceBalanceUpdateEvent(event *binance.BalanceUpdateEvent) {
	if s.MinMarginLevel.IsZero() {
		return
	}

	account := s.ExchangeSession.GetAccount()
	if account.MarginLevel.Compare(s.MinMarginLevel) > 0 {
		return
	}

	delta := fixedpoint.MustNewFromString(event.Delta)

	// ignore outflow
	if delta.Sign() < 0 {
		return
	}

	minMarginLevel := s.MinMarginLevel
	curMarginLevel := account.MarginLevel

	if b, ok := account.Balance(event.Asset); ok {
		if b.Available.IsZero() || b.Borrowed.IsZero() {
			return
		}

		toRepay := fixedpoint.Min(b.Borrowed, b.Available)
		if toRepay.IsZero() {
			return
		}

		bbgo.Notify(&MarginAction{
			Exchange:       s.ExchangeSession.ExchangeName,
			Action:         "Repay",
			Asset:          b.Currency,
			Amount:         toRepay,
			MarginLevel:    curMarginLevel,
			MinMarginLevel: minMarginLevel,
		})

		if err := s.marginBorrowRepay.RepayMarginAsset(context.Background(), event.Asset, toRepay); err != nil {
			log.WithError(err).Errorf("margin repay error")
		}
	}
}

type MarginAction struct {
	Exchange       types.ExchangeName `json:"exchange"`
	Action         string             `json:"action"`
	Asset          string             `json:"asset"`
	Amount         fixedpoint.Value   `json:"amount"`
	MarginLevel    fixedpoint.Value   `json:"marginLevel"`
	MinMarginLevel fixedpoint.Value   `json:"minMarginLevel"`
}

func (a *MarginAction) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Title: fmt.Sprintf("%s %s %s", a.Action, a.Amount, a.Asset),
		Color: "warning",
		Fields: []slack.AttachmentField{
			{
				Title: "Exchange",
				Value: a.Exchange.String(),
				Short: true,
			},
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
				Title: "Current Margin Level",
				Value: a.MarginLevel.String(),
				Short: true,
			},
			{
				Title: "Min Margin Level",
				Value: a.MinMarginLevel.String(),
				Short: true,
			},
		},
	}
}

// This strategy simply spent all available quote currency to buy the symbol whenever kline gets closed
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.MinMarginLevel.IsZero() {
		log.Warnf("%s: minMarginLevel is 0, you should configure this minimal margin ratio for controlling the liquidation risk", session.Name)
	}

	s.ExchangeSession = session

	marginBorrowRepay, ok := session.Exchange.(types.MarginBorrowRepayService)
	if !ok {
		return fmt.Errorf("exchange %s does not implement types.MarginBorrowRepayService", session.Name)
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
