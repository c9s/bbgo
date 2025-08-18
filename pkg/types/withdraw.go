package types

import (
	"fmt"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type WithdrawStatus string

const (
	WithdrawStatusSent             WithdrawStatus = "sent"
	WithdrawStatusCancelled        WithdrawStatus = "cancelled"
	WithdrawStatusAwaitingApproval WithdrawStatus = "awaiting_approval"
	WithdrawStatusRejected         WithdrawStatus = "rejected"
	WithdrawStatusProcessing       WithdrawStatus = "processing"
	WithdrawStatusFailed           WithdrawStatus = "failed"
	WithdrawStatusCompleted        WithdrawStatus = "completed"
	WithdrawStatusUnknown          WithdrawStatus = "unknown"
)

type Withdraw struct {
	GID            int64            `json:"gid" db:"gid"`
	Exchange       ExchangeName     `json:"exchange" db:"exchange"`
	Asset          string           `json:"asset" db:"asset"`
	Amount         fixedpoint.Value `json:"amount" db:"amount"`
	Address        string           `json:"address" db:"address"`
	AddressTag     string           `json:"addressTag"`
	Status         WithdrawStatus   `json:"status"`
	OriginalStatus string           `json:"originalStatus"`

	TransactionID          string           `json:"transactionID" db:"txn_id"`
	TransactionFee         fixedpoint.Value `json:"transactionFee" db:"txn_fee"`
	TransactionFeeCurrency string           `json:"transactionFeeCurrency" db:"txn_fee_currency"`
	WithdrawOrderID        string           `json:"withdrawOrderId"`
	ApplyTime              Time             `json:"applyTime" db:"time"`
	Network                string           `json:"network" db:"network"`
}

func cutstr(s string, maxLen, head, tail int) string {
	if len(s) > maxLen {
		l := len(s)
		return s[0:head] + "..." + s[l-tail:]
	}
	return s
}

func (w Withdraw) String() (o string) {
	o = fmt.Sprintf("%s WITHDRAW %s %s -> ", w.Exchange, w.Amount.String(), w.Asset)

	if len(w.Network) > 0 && w.Network != w.Asset {
		o += w.Network + ":"
	}

	o += fmt.Sprintf("%s @ %s", w.Address, w.ApplyTime.Time())

	if !w.TransactionFee.IsZero() {
		feeCurrency := w.TransactionFeeCurrency
		if feeCurrency == "" {
			feeCurrency = w.Asset
		}

		o += fmt.Sprintf(" FEE %4f %5s", w.TransactionFee.Float64(), feeCurrency)
	}

	if len(w.TransactionID) > 0 {
		o += fmt.Sprintf(" TxID: %s", cutstr(w.TransactionID, 12, 4, 4))
	}

	o += fmt.Sprintf(" STATUS: %s (%s)", w.Status, w.OriginalStatus)
	return o
}

func (w Withdraw) EffectiveTime() time.Time {
	return w.ApplyTime.Time()
}

func (w *Withdraw) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField

	if len(w.TransactionID) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "TransactionID",
			Value: w.TransactionID,
			Short: false,
		})
	}

	if w.TransactionFee.Sign() > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Transaction Fee",
			Value: fmt.Sprintf("%s %s", w.TransactionFee.String(), w.TransactionFeeCurrency),
			Short: false,
		})
	}

	if len(w.Status) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Status",
			Value: fmt.Sprintf("%s (%s)", w.Status, w.OriginalStatus),
			Short: false,
		})
	}

	return slack.Attachment{
		Color: withdrawStatusSlackColor(w.Status),
		Title: fmt.Sprintf("Withdraw %s %s To %s (Network %s)", w.Amount.String(), w.Asset, w.Address, w.Network),
		// TitleLink:   "",
		Pretext: "",
		Text:    "",
		// ServiceName: "",
		// ServiceIcon: "",
		// FromURL:     "",
		// OriginalURL: "",
		Fields: fields,
		Footer: fmt.Sprintf("Apply Time: %s", w.ApplyTime.Time().Format(time.RFC3339)),
		// FooterIcon: "",
	}
}

func withdrawStatusSlackColor(status WithdrawStatus) string {
	switch status {

	case WithdrawStatusCompleted:
		return "good"

	case WithdrawStatusFailed:
		return "red"

	case WithdrawStatusCancelled:
		return "gray"

	default:
		return "gray"

	}
}

type WithdrawalOptions struct {
	Network    string
	AddressTag string
}
