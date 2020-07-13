package slack

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/bbgo/types"
	"github.com/c9s/bbgo/pkg/slack/slackstyle"
	"github.com/c9s/bbgo/pkg/util"
)

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

func (t *SlackNotifier) ReportTrade(trade *types.Trade) {
	_, _, err := t.Slack.PostMessageContext(context.Background(), t.TradingChannel,
		slack.MsgOptionText(util.Render(`:handshake: trade execution @ {{ .Price  }}`, trade), true),
		slack.MsgOptionAttachments(trade.SlackAttachment()))

	if err != nil {
		log.WithError(err).Error("slack send error")
	}
}

func (t *SlackNotifier) ReportPnL(report *bbgo.ProfitAndLossReport) {
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

