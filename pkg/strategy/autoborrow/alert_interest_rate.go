package autoborrow

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/livenote"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/currency"
)

type MarginHighInterestRateAlert struct {
	AlertID      string
	AccountLabel string

	SessionName string
	Exchange    types.ExchangeName

	HighRateAssets types.MarginNextHourlyInterestRateMap

	NextTotalInterestValueInUSD fixedpoint.Value
	Debts                       types.BalanceMap
}

func (a *MarginHighInterestRateAlert) ObjectID() string {
	return a.AlertID
}

func (a *MarginHighInterestRateAlert) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField

	if len(a.AccountLabel) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Account",
			Value: a.AccountLabel,
		})
	}

	if len(a.SessionName) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Session",
			Value: a.SessionName,
		})
	}

	if len(a.Exchange) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Exchange",
			Value: string(a.Exchange),
		})
	}

	isRisky := len(a.HighRateAssets) > 0

	color := "good"
	text := "✅ No high interest rate assets found"

	if isRisky {
		color = "warning"
		text = fmt.Sprintf("💹 %d high interest rate assets found", len(a.HighRateAssets))
	}

	for asset, rate := range a.HighRateAssets {
		desc := "APY: " + rate.AnnualizedRate.FormatPercentage(2)
		if debt, ok := a.Debts[asset]; ok {
			desc += " Debt: " + debt.Debt().String()
		}

		fields = append(fields, slack.AttachmentField{
			Title: asset,
			Value: desc,
		})
	}

	if a.NextTotalInterestValueInUSD.Sign() > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Total Interest Value In USD",
			Value: a.NextTotalInterestValueInUSD.String(),
		})
	}

	return slack.Attachment{
		Color:      color,
		Title:      "High Interest Rate Alert",
		Text:       text,
		Fields:     fields,
		MarkdownIn: []string{"text"},
	}
}

type marginFutureInterestQueryService interface {
	QueryMarginFutureHourlyInterestRate(ctx context.Context, assets []string) (rates types.MarginNextHourlyInterestRateMap, err error)
}

type MarginHighInterestRateWorker struct {
	strategy *Strategy
	session  *bbgo.ExchangeSession
	config   *MarginHighInterestRateAlertConfig

	service marginFutureInterestQueryService
}

func newMarginHighInterestRateWorker(strategy *Strategy, config *MarginHighInterestRateAlertConfig) *MarginHighInterestRateWorker {
	session := strategy.ExchangeSession
	service, support := session.Exchange.(marginFutureInterestQueryService)
	if !support {
		log.Warnf("exchange %T does not support margin future interest rate query", session.Exchange)
	}

	return &MarginHighInterestRateWorker{
		strategy: strategy,
		session:  session,
		config:   config,
		service:  service,
	}
}

func (w *MarginHighInterestRateWorker) findMarginHighInterestRateAssets(
	debts types.BalanceMap,
	rateMap types.MarginNextHourlyInterestRateMap,
	minAnnualRate float64,
) (highRates types.MarginNextHourlyInterestRateMap, err error) {
	highRates = make(types.MarginNextHourlyInterestRateMap)
	for asset, rate := range rateMap {
		bal, ok := debts[asset]
		if !ok {
			continue
		}

		debt := bal.Debt()
		if debt.IsZero() {
			continue
		}

		if rate.AnnualizedRate.IsZero() {
			log.Warnf("annualized rate is zero for %s", asset)
		}

		if rate.AnnualizedRate.Float64() >= minAnnualRate {
			highRates[asset] = rate
		}
	}

	return highRates, nil
}

func (w *MarginHighInterestRateWorker) tryFix(ctx context.Context, asset string) error {
	log.Infof("trying to repay %s to fix high interest rate alert", asset)

	bal, ok := w.session.Account.Balance(asset)
	if !ok {
		return fmt.Errorf("balance of %s not found", asset)
	}

	if bal.Available.IsZero() {
		return fmt.Errorf("available balance of %s is zero", asset)
	}

	repayService, support := w.session.Exchange.(types.MarginBorrowRepayService)
	if !support {
		return fmt.Errorf("exchange %s does not support margin borrow repay service", w.session.ExchangeName)
	}

	log.Infof("repaying %s %s", bal.Debt().String(), asset)

	repay := fixedpoint.Min(bal.Available, bal.Debt())
	return repayService.RepayMarginAsset(ctx, asset, repay)
}

func (w *MarginHighInterestRateWorker) Run(ctx context.Context) {
	alertInterval := time.Minute * 5
	if w.config.Interval > 0 {
		alertInterval = w.config.Interval.Duration()
	}

	if w.service == nil {
		log.Warnf("exchange %T does not support margin future interest rate query", w.session.Exchange)
		return
	}

	ticker := time.NewTicker(alertInterval)
	defer ticker.Stop()

	danger := false

	// alertId is used to identify the alert message when the alert is solved, we
	// should send a new alert message instead of replacing the previous one, so the
	// alertId will be updated to a new uuid once the alert is solved
	alertId := uuid.New().String()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			assets := w.strategy.getAssetStringSlice()

			rateMap, err := w.service.QueryMarginFutureHourlyInterestRate(ctx, assets)
			if err != nil {
				log.WithError(err).Errorf("unable to query the next future hourly interest rate")
				continue
			}

			log.Infof("rates: %+v", rateMap)

			debts := w.session.Account.Balances().Debts()

			highRateAssets, err := w.findMarginHighInterestRateAssets(debts, rateMap, w.config.MinAnnualInterestRate.Float64())
			if err != nil {
				log.WithError(err).Errorf("unable to query the next future hourly interest rate")
				continue
			}

			if len(highRateAssets) > 0 {
				log.Infof("found high interest rate assets: %+v", highRateAssets)
			}

			nextTotalInterestValue := fixedpoint.Zero
			exceededDebtAmount := false
			for cur, bal := range debts {
				price, ok := w.session.GetPriceSolver().ResolvePrice(cur, currency.USDT)
				if !ok {
					log.Warnf("unable to resolve price for %s", cur)
					continue
				}

				rate := rateMap[cur]
				debtValue := bal.Debt().Mul(price)
				nextTotalInterestValue = nextTotalInterestValue.Add(
					debtValue.Mul(rate.HourlyRate))

				if w.config.MinDebtAmount.Sign() > 0 &&
					debtValue.Compare(w.config.MinDebtAmount) > 0 {
					exceededDebtAmount = true
				}
			}

			shouldAlert := func() bool {
				return len(highRateAssets) > 0 && exceededDebtAmount
			}

			// either danger or margin level is less than the minimal margin level
			// if the previous danger is set to true, we should send the alert again to
			// update the previous danger margin alert message
			if danger || shouldAlert() {
				allRepaid := 0
				for asset := range highRateAssets {
					if err := w.tryFix(ctx, asset); err != nil {
						log.WithError(err).Errorf("unable to fix high interest rate alert")
					} else {
						allRepaid++
					}
				}

				// if the previous danger flag is not set, and all high interest rate assets are repaid
				// we should not send the alert
				if !danger && allRepaid == len(highRateAssets) {
					log.Infof("all high interest rate assets are repaid")
					continue
				}

				alert := &MarginHighInterestRateAlert{
					AlertID:        alertId,
					AccountLabel:   w.session.GetAccountLabel(),
					Exchange:       w.session.ExchangeName,
					SessionName:    w.session.Name,
					Debts:          debts,
					HighRateAssets: highRateAssets,
				}

				bbgo.PostLiveNote(alert,
					livenote.Channel(w.config.Slack.Channel),
					livenote.OneTimeMention(w.config.Slack.Mentions...),
					livenote.CompareObject(true),
				)

				// if the previous danger flag is not set, we should send the alert at the first time
				if !danger {
					w.strategy.postLiveNoteMessage(alert, w.config.Slack, "⚠️ High interest rate assets found, please repay the debt")
				}

				// update danger flag
				danger = shouldAlert()

				// if it's not in danger anymore, send a solved message
				if !danger {
					alertId = uuid.New().String()
					w.strategy.postLiveNoteMessage(alert, w.config.Slack, "✅ High interest rate alert is solved")
				}
			}
		}
	}
}
