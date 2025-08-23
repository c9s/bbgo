package xalign

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/livenote"
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	"github.com/c9s/bbgo/pkg/types"
)

var (
	_ livenote.Object = &CriticalBalanceDiscrepancyAlert{}
)

var cbdDateCache = struct {
	notifyDate time.Time
}{}

type CriticalBalanceDiscrepancyAlert struct {
	SlackAlert *slackalert.SlackAlert

	Warning bool

	BaseCurrency      string
	Delta             fixedpoint.Value
	SustainedDuration time.Duration

	QuoteCurrency string
	AlertAmount   fixedpoint.Value

	Side     types.SideType
	Price    fixedpoint.Value
	Quantity fixedpoint.Value
	Amount   fixedpoint.Value
}

func (m *CriticalBalanceDiscrepancyAlert) SlackAttachment() slack.Attachment {
	color := "red"

	if m.Warning {
		color = "yellow"
	}

	titlePrefix := "Critical Balance Discrepancy Alert:"
	if m.Warning {
		titlePrefix = "Critical Balance Discrepancy Warning:"
	}

	if m.Delta.Sign() > 0 {
		m.Side = types.SideTypeBuy
	} else {
		m.Side = types.SideTypeSell
	}

	title := titlePrefix + fmt.Sprintf(" %f %s",
		m.Delta.Float64(),
		m.BaseCurrency,
	)

	if m.SustainedDuration > 0 {
		title += fmt.Sprintf(" sustained for %s", m.SustainedDuration)
	}

	title += fmt.Sprintf(" (~= %f %s > %f %s)",
		m.Amount.Float64(),
		m.QuoteCurrency,
		m.AlertAmount.Float64(),
		m.QuoteCurrency,
	)
	fields := []slack.AttachmentField{
		{
			Title: "Base Currency",
			Value: m.BaseCurrency,
			Short: true,
		},
		{
			Title: "Side",
			Value: m.Side.String(),
			Short: true,
		},
		{
			Title: "Price",
			Value: m.Price.String(),
			Short: true,
		},
		{
			Title: "Quantity",
			Value: m.Quantity.String(),
			Short: true,
		},
	}
	hostName := getHostname()
	if hostName != "" && hostName != "localhost" {
		fields = append(fields, slack.AttachmentField{
			Title: "Hostname",
			Value: hostName,
			Short: true,
		})
	}
	text := "Please check the balances"
	if m.SlackAlert != nil && len(m.SlackAlert.Mentions) > 0 {
		text = strings.Join(m.SlackAlert.Mentions, " ") + " " + text
	}
	return slack.Attachment{
		Color:  color,
		Title:  title,
		Text:   text,
		Footer: fmt.Sprintf("strategy: %s", ID),
		Fields: fields,
	}
}

func (m *CriticalBalanceDiscrepancyAlert) ObjectID() string {
	if cbdDateCache.notifyDate.IsZero() {
		cbdDateCache.notifyDate = time.Now().Round(time.Hour * 24)
	}
	var dateString string
	currentTime := time.Now()
	if currentTime.Sub(cbdDateCache.notifyDate) <= time.Hour*24 {
		dateString = cbdDateCache.notifyDate.Format(time.DateOnly)
	} else {
		cbdDateCache.notifyDate = currentTime
		dateString = currentTime.Format(time.DateOnly)
	}

	return fmt.Sprintf(
		"critical-balance-discrepancy-%s%s-%s-%s",
		m.BaseCurrency,
		m.QuoteCurrency,
		dateString,
		m.Side.String(),
	)
}

func (m *CriticalBalanceDiscrepancyAlert) Comment() *livenote.OptionComment {
	return livenote.Comment(
		fmt.Sprintf(
			"%s %s sustained for %s (~= %f %s > %f %s)",
			m.BaseCurrency,
			m.Delta,
			m.SustainedDuration,
			m.Amount.Float64(),
			m.QuoteCurrency,
			m.AlertAmount.Float64(),
			m.QuoteCurrency,
		),
	)
}

func getHostname() string {
	return os.Getenv("HOSTNAME")
}
