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
	rateMap types.MarginNextHourlyInterestRateMap,
	minAnnualRate float64,
) (highRates types.MarginNextHourlyInterestRateMap, err error) {
	highRates = make(types.MarginNextHourlyInterestRateMap)
	for asset, rate := range rateMap {
		if rate.AnnualizedRate.IsZero() {
			log.Warnf("annualized rate is zero for %s", asset)
		}

		if rate.AnnualizedRate.Float64() >= minAnnualRate {
			highRates[asset] = rate
		}
	}

	return highRates, nil
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

			highRateAssets, err := w.findMarginHighInterestRateAssets(rateMap, w.config.MinAnnualInterestRate.Float64())
			if err != nil {
				log.WithError(err).Errorf("unable to query the next future hourly interest rate")
				continue
			}

			log.Infof("found high interest rate assets: %+v", highRateAssets)

			totalDebtValue := fixedpoint.Zero
			nextTotalInterestValue := fixedpoint.Zero
			debts := w.session.Account.Balances().Debts()
			for cur, bal := range debts {
				price, ok := w.session.GetPriceSolver().ResolvePrice(cur, currency.USDT)
				if !ok {
					log.Warnf("unable to resolve price for %s", cur)
					continue
				}

				rate := rateMap[cur]
				nextTotalInterestValue = nextTotalInterestValue.Add(
					bal.Debt().Mul(rate.HourlyRate).Mul(price))

				totalDebtValue = totalDebtValue.Add(bal.Debt().Mul(price))
			}

			shouldAlert := func() bool {
				return len(highRateAssets) > 0 &&
					w.config.MinDebtAmount.Sign() > 0 &&
					totalDebtValue.Compare(w.config.MinDebtAmount) > 0
			}

			// either danger or margin level is less than the minimal margin level
			// if the previous danger is set to true, we should send the alert again to
			// update the previous danger margin alert message
			if danger || shouldAlert() {
				// calculate the debt value by the price solver

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
