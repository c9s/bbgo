package autoborrow

import (
	"context"
	"fmt"
	"strings"
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

/*
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

// MarginAlert is used to send the slack mention alerts when the current margin is less than the required margin level
type MarginAlert struct {
	CurrentMarginLevel fixedpoint.Value
	MinimalMarginLevel fixedpoint.Value
	SlackMentions      []string
	SessionName        string
}

func (m *MarginAlert) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Color: "red",
		Title: fmt.Sprintf("Margin Level Alert: %s session - current margin level %f < required margin level %f",
			m.SessionName, m.CurrentMarginLevel.Float64(), m.MinimalMarginLevel.Float64()),
		Text: strings.Join(m.SlackMentions, " "),
		Fields: []slack.AttachmentField{
			{
				Title: "Session",
				Value: m.SessionName,
				Short: true,
			},
			{
				Title: "Current Margin Level",
				Value: m.CurrentMarginLevel.String(),
				Short: true,
			},
			{
				Title: "Minimal Margin Level",
				Value: m.MinimalMarginLevel.String(),
				Short: true,
			},
		},
		// Footer:     "",
		// FooterIcon: "",
	}
}

// RepaidAlert
type RepaidAlert struct {
	SessionName   string
	Asset         string
	Amount        fixedpoint.Value
	SlackMentions []string
}

func (m *RepaidAlert) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Color: "red",
		Title: fmt.Sprintf("Margin Repaid on %s session", m.SessionName),
		Text:  strings.Join(m.SlackMentions, " "),
		Fields: []slack.AttachmentField{
			{
				Title: "Session",
				Value: m.SessionName,
				Short: true,
			},
			{
				Title: "Asset",
				Value: m.Amount.String() + " " + m.Asset,
				Short: true,
			},
		},
		// Footer:     "",
		// FooterIcon: "",
	}
}

type MarginAsset struct {
	Asset                string           `json:"asset"`
	Low                  fixedpoint.Value `json:"low"`
	MaxTotalBorrow       fixedpoint.Value `json:"maxTotalBorrow"`
	MaxQuantityPerBorrow fixedpoint.Value `json:"maxQuantityPerBorrow"`
	MinQuantityPerBorrow fixedpoint.Value `json:"minQuantityPerBorrow"`
	DebtRatio            fixedpoint.Value `json:"debtRatio"`
}

type MarginLevelAlert struct {
	Interval      types.Duration   `json:"interval"`
	MinMargin     fixedpoint.Value `json:"minMargin"`
	SlackMentions []string         `json:"slackMentions"`
}

type MarginRepayAlert struct {
	SlackMentions []string `json:"slackMentions"`
}

type Strategy struct {
	Interval             types.Interval   `json:"interval"`
	MinMarginLevel       fixedpoint.Value `json:"minMarginLevel"`
	MaxMarginLevel       fixedpoint.Value `json:"maxMarginLevel"`
	AutoRepayWhenDeposit bool             `json:"autoRepayWhenDeposit"`

	MarginLevelAlert *MarginLevelAlert `json:"marginLevelAlert"`
	MarginRepayAlert *MarginRepayAlert `json:"marginRepayAlert"`

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
		debt := b.Debt()

		if debt.Sign() <= 0 {
			continue
		}

		if b.Available.IsZero() {
			continue
		}

		toRepay := fixedpoint.Min(b.Available, debt)
		if toRepay.IsZero() {
			continue
		}

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

		if s.MarginRepayAlert != nil {
			bbgo.Notify(&RepaidAlert{
				SessionName:   s.ExchangeSession.Name,
				Asset:         b.Currency,
				Amount:        toRepay,
				SlackMentions: s.MarginRepayAlert.SlackMentions,
			})
		}

		return
	}
}

func (s *Strategy) reBalanceDebt(ctx context.Context) {
	log.Infof("rebalancing debt...")

	account, err := s.ExchangeSession.UpdateAccount(ctx)
	if err != nil {
		log.WithError(err).Errorf("can not update account")
		return
	}

	minMarginLevel := s.MinMarginLevel

	balances := account.Balances().NotZero()
	if len(balances) == 0 {
		log.Warn("balance is empty, skip repay")
		return
	}

	log.Infof("non-zero balances: %+v", balances)

	for _, marginAsset := range s.Assets {
		b, ok := balances[marginAsset.Asset]
		if !ok {
			continue
		}

		// debt / total
		debt := b.Debt()
		total := b.Total()

		debtRatio := debt.Div(total)

		if marginAsset.DebtRatio.IsZero() {
			marginAsset.DebtRatio = fixedpoint.One
		}

		if total.Compare(marginAsset.Low) <= 0 {
			log.Infof("%s total %f is less than margin asset low %f, skip early repay", marginAsset.Asset, total.Float64(), marginAsset.Low.Float64())
			continue
		}

		log.Infof("checking debtRatio: session = %s asset = %s, debt = %f, total = %f, debtRatio = %f", s.ExchangeSession.Name, marginAsset.Asset, debt.Float64(), total.Float64(), debtRatio.Float64())

		// if debt is greater than total, skip repay
		if debt.Compare(total) > 0 {
			log.Infof("%s debt %f is greater than total %f, skip early repay", marginAsset.Asset, debt.Float64(), total.Float64())
			continue
		}

		// if debtRatio is lesser, means that we have more spot, we should try to repay as much as we can
		if debtRatio.Compare(marginAsset.DebtRatio) > 0 {
			log.Infof("%s debt ratio %f is greater than min debt ratio %f, skip", marginAsset.Asset, debtRatio.Float64(), marginAsset.DebtRatio.Float64())
			continue
		}

		log.Infof("checking repayable balance: %+v", b)

		toRepay := debt

		if b.Available.IsZero() {
			log.Errorf("%s available balance is 0, can not repay, balance = %+v", marginAsset.Asset, b)
			continue
		}

		toRepay = fixedpoint.Min(toRepay, b.Available)

		if !marginAsset.Low.IsZero() {
			extra := b.Available.Sub(marginAsset.Low)
			if extra.Sign() > 0 {
				toRepay = fixedpoint.Min(extra, toRepay)
			}
		}

		if toRepay.Sign() <= 0 {
			log.Warnf("%s repay = %f, available = %f, borrowed = %f, can not repay",
				marginAsset.Asset,
				toRepay.Float64(),
				b.Available.Float64(),
				b.Borrowed.Float64())
			continue
		}

		log.Infof("%s repay %f", marginAsset.Asset, toRepay.Float64())

		bbgo.Notify(&MarginAction{
			Exchange:       s.ExchangeSession.ExchangeName,
			Action:         fmt.Sprintf("Repay for Debt Ratio %f < Minimal Debt Ratio %f", debtRatio.Float64(), marginAsset.DebtRatio.Float64()),
			Asset:          b.Currency,
			Amount:         toRepay,
			MarginLevel:    account.MarginLevel,
			MinMarginLevel: minMarginLevel,
		})

		if err := s.marginBorrowRepay.RepayMarginAsset(context.Background(), b.Currency, toRepay); err != nil {
			log.WithError(err).Errorf("margin repay error")
		}

		if s.MarginRepayAlert != nil {
			bbgo.Notify(&RepaidAlert{
				SessionName:   s.ExchangeSession.Name,
				Asset:         b.Currency,
				Amount:        toRepay,
				SlackMentions: s.MarginRepayAlert.SlackMentions,
			})
		}

		if accountUpdate, err2 := s.ExchangeSession.UpdateAccount(ctx); err2 != nil {
			log.WithError(err).Errorf("unable to update account")
		} else {
			account = accountUpdate
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
	for maxTries := 5; account.MarginLevel.Compare(minMarginLevel) < 0 && maxTries > 0; maxTries-- {
		log.Infof("current margin level %f < min margin level %f, skip autoborrow", account.MarginLevel.Float64(), minMarginLevel.Float64())

		bbgo.Notify("Warning!!! %s Current Margin Level %f < Minimal Margin Level %f",
			s.ExchangeSession.Name,
			account.MarginLevel.Float64(),
			minMarginLevel.Float64(),
			account.Balances().Debts(),
		)

		s.tryToRepayAnyDebt(ctx)

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 5):
		}

		// update account info after the repay
		account, err = s.ExchangeSession.UpdateAccount(ctx)
		if err != nil {
			log.WithError(err).Errorf("can not update account")
			return
		}
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
				newTotalBorrow := toBorrow.Add(b.Borrowed)
				if newTotalBorrow.Compare(marginAsset.MaxTotalBorrow) > 0 {
					toBorrow = toBorrow.Sub(newTotalBorrow.Sub(marginAsset.MaxTotalBorrow))
					if toBorrow.Sign() < 0 {
						log.Warnf("margin asset %s is over borrowed, skip", marginAsset.Asset)
						continue
					}
				}
			}

			maxBorrowable, err2 := s.marginBorrowRepay.QueryMarginAssetMaxBorrowable(ctx, marginAsset.Asset)
			if err2 != nil {
				log.WithError(err).Errorf("max borrowable query error")
				continue
			}

			if toBorrow.Compare(maxBorrowable) > 0 {
				bbgo.Notify("Trying to borrow %f %s, which is greater than the max borrowable amount %f, will adjust borrow amount to %f",
					toBorrow.Float64(),
					marginAsset.Asset,
					maxBorrowable.Float64(),
					maxBorrowable.Float64())

				toBorrow = fixedpoint.Min(maxBorrowable, toBorrow)
			}

			if toBorrow.IsZero() {
				continue
			}

			bbgo.Notify(&MarginAction{
				Exchange:       s.ExchangeSession.ExchangeName,
				Action:         "Borrow",
				Asset:          marginAsset.Asset,
				Amount:         toBorrow,
				MarginLevel:    account.MarginLevel,
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
	bbgo.Notify(event)

	account := s.ExchangeSession.GetAccount()

	delta := event.Delta

	// ignore outflow
	if delta.Sign() < 0 {
		return
	}

	minMarginLevel := s.MinMarginLevel
	curMarginLevel := account.MarginLevel

	// margin repay/borrow also trigger this update event
	if curMarginLevel.Compare(minMarginLevel) > 0 {
		return
	}

	if b, ok := account.Balance(event.Asset); ok {
		if b.Available.IsZero() {
			return
		}

		debt := b.Debt()
		if debt.IsZero() {
			return
		}

		toRepay := fixedpoint.Min(debt, b.Available)
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

	if s.MarginLevelAlert != nil && !s.MarginLevelAlert.MinMargin.IsZero() {
		alertInterval := time.Minute * 5
		if s.MarginLevelAlert.Interval > 0 {
			alertInterval = s.MarginLevelAlert.Interval.Duration()
		}

		go s.marginAlertWorker(ctx, alertInterval)
	}

	go s.run(ctx, s.Interval.Duration())
	return nil
}

func (s *Strategy) marginAlertWorker(ctx context.Context, alertInterval time.Duration) {
	go func() {
		ticker := time.NewTicker(alertInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				account := s.ExchangeSession.GetAccount()
				if s.MarginLevelAlert != nil && account.MarginLevel.Compare(s.MarginLevelAlert.MinMargin) <= 0 {
					bbgo.Notify(&MarginAlert{
						CurrentMarginLevel: account.MarginLevel,
						MinimalMarginLevel: s.MarginLevelAlert.MinMargin,
						SlackMentions:      s.MarginLevelAlert.SlackMentions,
						SessionName:        s.ExchangeSession.Name,
					})
					bbgo.Notify(account.Balances().Debts())
				}
			}
		}
	}()
}
