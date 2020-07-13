package bbgo

import (
	"context"
	"fmt"
	"time"

	"github.com/leekchan/accounting"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/bbgo/exchange/binance"
	"github.com/c9s/bbgo/pkg/bbgo/types"
	"github.com/c9s/bbgo/pkg/slack/slackstyle"
	"github.com/c9s/bbgo/pkg/util"
)

var USD = accounting.Accounting{Symbol: "$ ", Precision: 2}
var BTC = accounting.Accounting{Symbol: "BTC ", Precision: 8}

type SlackLogHook struct {
	Slack *slack.Client
	ErrorChannel   string
}

func (t *SlackLogHook) Levels() []log.Level {
	return []log.Level{
		// log.InfoLevel,
		log.ErrorLevel,
		log.PanicLevel,
		// log.WarnLevel,
	}
}

func (t *SlackLogHook) Fire(e *log.Entry) error {
	var color = "#F0F0F0"

	switch e.Level {
	case log.DebugLevel:
		color = "#9B30FF"
	case log.InfoLevel:
		color = "good"
	case log.ErrorLevel, log.FatalLevel, log.PanicLevel:
		color = "danger"
	default:
		color = "warning"
	}

	var slackAttachments []slack.Attachment = nil

	logerr, ok := e.Data["err"]
	if ok {
		slackAttachments = append(slackAttachments, slack.Attachment{
			Color: color,
			Title: "Error",
			Fields: []slack.AttachmentField{
				{Title: "Error", Value: logerr.(error).Error()},
			},
		})
	}

	_, _, err := t.Slack.PostMessageContext(context.Background(), t.ErrorChannel,
		slack.MsgOptionText(e.Message, true),
		slack.MsgOptionAttachments(slackAttachments...))

	return err
}


type SlackNotifier struct {
	Slack *slack.Client

	TradingChannel string
	ErrorChannel   string
	InfoChannel    string
}


func (t *SlackNotifier) Infof(format string, args ...interface{}) {
	var slackAttachments []slack.Attachment = nil
	var slackArgsStartIdx = -1
	for idx, arg := range args {
		switch a := arg.(type) {

		// concrete type assert first
		case slack.Attachment:
			if slackArgsStartIdx == -1 {
				slackArgsStartIdx = idx
			}
			slackAttachments = append(slackAttachments, a)

		case slackstyle.SlackAttachmentCreator:
			if slackArgsStartIdx == -1 {
				slackArgsStartIdx = idx
			}
			slackAttachments = append(slackAttachments, a.SlackAttachment())

		}
	}

	var nonSlackArgs = []interface{}{}
	if slackArgsStartIdx > 0 {
		nonSlackArgs = args[:slackArgsStartIdx]
	}

	log.Infof(format, nonSlackArgs...)

	_, _, err := t.Slack.PostMessageContext(context.Background(), t.InfoChannel,
		slack.MsgOptionText(fmt.Sprintf(format, nonSlackArgs...), true),
		slack.MsgOptionAttachments(slackAttachments...))
	if err != nil {
		log.WithError(err).Errorf("slack error: %s", err.Error())
	}
}

func (t *SlackNotifier) Errorf(err error, format string, args ...interface{}) {
	log.WithError(err).Errorf(format, args...)
	_, _, err2 := t.Slack.PostMessageContext(context.Background(), t.ErrorChannel,
		slack.MsgOptionText("ERROR: "+err.Error()+" "+fmt.Sprintf(format, args...), true))
	if err2 != nil {
		log.WithError(err2).Error("slack error:", err2)
	}
}

func (t *SlackNotifier) ReportTrade(trade *types.Trade) {
	_, _, err := t.Slack.PostMessageContext(context.Background(), t.TradingChannel,
		slack.MsgOptionText(util.Render(`:handshake: trade execution @ {{ .Price  }}`, trade), true),
		slack.MsgOptionAttachments(trade.SlackAttachment()))

	if err != nil {
		log.WithError(err).Error("slack send error")
	}
}

func (t *SlackNotifier) ReportPnL(report *ProfitAndLossReport) {
	attachment := report.SlackAttachment()

	_, _, err := t.Slack.PostMessageContext(context.Background(), t.TradingChannel,
		slack.MsgOptionText(util.Render(
			`:heavy_dollar_sign: Here is your *{{ .symbol }}* PnL report collected since *{{ .startTime }}*`,
			map[string]interface{}{
				"symbol":    report.Symbol,
				"startTime": report.StartTime.Format(time.RFC822),
			}), true),
		slack.MsgOptionAttachments(attachment))

	if err != nil {
		log.WithError(err).Errorf("slack send error")
	}
}

type Trader struct {
	Notifier *SlackNotifier

	// Context is trading Context
	Context *TradingContext

	Exchange *binance.Exchange

	Slack *slack.Client

	TradingChannel string
	ErrorChannel   string
	InfoChannel    string
}

func (t *Trader) Infof(format string, args ...interface{}) {
	t.Notifier.Infof(format, args...)
}

func (t *Trader) ReportTrade(trade *types.Trade) {
	t.Notifier.ReportTrade(trade)
}

func (t *Trader) ReportPnL() {
	report := t.Context.ProfitAndLossCalculator.Calculate()
	report.Print()
	t.Notifier.ReportPnL(report)
}

func (t *Trader) SubmitOrder(ctx context.Context, order *types.Order) {
	t.Infof(":memo: Submitting %s order on side %s with volume: %s", order.Type, order.Side, order.VolumeStr, order.SlackAttachment())

	err := t.Exchange.SubmitOrder(ctx, order)
	if err != nil {
		log.WithError(err).Errorf("order create error: side %s volume: %s", order.Side, order.VolumeStr)
		return
	}
}
