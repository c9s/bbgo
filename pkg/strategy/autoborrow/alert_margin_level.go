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
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/currency"
)

// MarginLevelAlert is used to send the slack mention alerts when the current margin is less than the required margin level
type MarginLevelAlert struct {
	AlertID             string
	AccountLabel        string
	CurrentMarginLevel  fixedpoint.Value
	MinimalMarginLevel  fixedpoint.Value
	SessionName         string
	Exchange            types.ExchangeName
	TotalDebtValueInUSD fixedpoint.Value
	Debts               types.BalanceMap
}

func (m *MarginLevelAlert) ObjectID() (str string) {
	str = m.AccountLabel
	if m.AlertID != "" {
		str += "-" + m.AlertID
	}

	return str
}

func (m *MarginLevelAlert) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField

	if len(m.AccountLabel) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Account",
			Value: m.AccountLabel,
		})
	}

	if len(m.Exchange) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Exchange",
			Value: string(m.Exchange),
		})
	}

	fields = append(fields, []slack.AttachmentField{
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
	}...)

	if !m.TotalDebtValueInUSD.IsZero() {
		fields = append(fields, slack.AttachmentField{
			Title: "Total Debt Value In USD",
			Value: m.TotalDebtValueInUSD.String(),
		})
	}

	// collect the current debts into the alert fields
	if m.Debts != nil && len(m.Debts) > 0 {
		fields = append(fields, m.Debts.SlackAttachment().Fields...)
	}

	color := "good"
	isDanger := m.CurrentMarginLevel.Compare(m.MinimalMarginLevel) <= 0
	if isDanger {
		color = "danger"
	}

	isSolved := !isDanger
	if isSolved {
		fields = append(fields, slack.AttachmentField{
			Title: "Status",
			Value: "✅ Solved",
		})
	}

	footer := fmt.Sprintf("%s - %s", m.Exchange, time.Now().String())

	return slack.Attachment{
		Color: color,
		Title: fmt.Sprintf("Margin Level Alert: %s session",
			m.SessionName,
		),
		Text: fmt.Sprintf("The current margin level %f is less than required margin level %f",
			m.CurrentMarginLevel.Float64(),
			m.MinimalMarginLevel.Float64(),
		),
		Fields:     fields,
		Footer:     footer,
		FooterIcon: types.ExchangeFooterIcon(m.Exchange),
	}
}

type MarginLevelAlertConfig struct {
	Slack     *slackalert.SlackAlert `json:"slack"`
	Interval  types.Duration         `json:"interval"`
	MinMargin fixedpoint.Value       `json:"minMargin"`
}

func (s *Strategy) marginLevelAlertWorker(ctx context.Context, config *MarginLevelAlertConfig) {
	alertInterval := time.Minute * 5
	if config.Interval > 0 {
		alertInterval = config.Interval.Duration()
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
			account, err := s.ExchangeSession.UpdateAccount(ctx)
			if err != nil {
				log.WithError(err).Errorf("unable to update account")
				continue
			}

			// either danger or margin level is less than the minimal margin level
			// if the previous danger is set to true, we should send the alert again to
			// update the previous danger margin alert message
			if danger || account.MarginLevel.Compare(config.MinMargin) <= 0 {
				// calculate the debt value by the price solver
				totalDebtValueInUSDT := fixedpoint.Zero
				debts := account.Balances().Debts()
				for cur, bal := range debts {
					price, ok := s.priceSolver.ResolvePrice(cur, currency.USDT)
					if !ok {
						log.Warnf("unable to resolve price for %s", cur)
						continue
					}

					debtValue := bal.Debt().Mul(price)
					totalDebtValueInUSDT = totalDebtValueInUSDT.Add(debtValue)
				}

				alert := &MarginLevelAlert{
					AlertID:             alertId,
					AccountLabel:        s.ExchangeSession.GetAccountLabel(),
					Exchange:            s.ExchangeSession.ExchangeName,
					CurrentMarginLevel:  account.MarginLevel,
					MinimalMarginLevel:  config.MinMargin,
					SessionName:         s.ExchangeSession.Name,
					TotalDebtValueInUSD: totalDebtValueInUSDT,
					Debts:               account.Balances().Debts(),
				}

				bbgo.PostLiveNote(alert,
					livenote.Channel(config.Slack.Channel),
					livenote.OneTimeMention(config.Slack.Mentions...),
					livenote.CompareObject(true),
				)

				// if the previous danger flag is not set, we should send the alert at the first time
				if !danger {
					s.postLiveNoteMessage(alert, config.Slack, "⚠️ The current margin level %f is less than the minimal margin level %f, please repay the debt",
						account.MarginLevel.Float64(),
						config.MinMargin.Float64())

				}

				// update danger flag
				danger = account.MarginLevel.Compare(config.MinMargin) <= 0

				// if it's not in danger anymore, send a solved message
				if !danger {
					alertId = uuid.New().String()
					s.postLiveNoteMessage(alert, config.Slack, "✅ The current margin level %f is safe now", account.MarginLevel.Float64())
				}
			}
		}
	}
}
