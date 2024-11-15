package types

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type DepositStatus string

const (
	// EMPTY string means not supported

	DepositPending = DepositStatus("pending")

	DepositRejected = DepositStatus("rejected")

	DepositSuccess = DepositStatus("success")

	DepositCancelled = DepositStatus("canceled")

	// created but can not withdraw
	DepositCredited = DepositStatus("credited")
)

func (s DepositStatus) SlackEmoji() string {
	switch s {
	case DepositPending:
		return "⏳"
	case DepositCredited:
		return "💵"
	case DepositSuccess:
		return "✅"
	case DepositCancelled:
		return "⏏"
	}

	return ""
}

type Deposit struct {
	GID           int64            `json:"gid" db:"gid"`
	Exchange      ExchangeName     `json:"exchange" db:"exchange"`
	Time          Time             `json:"time" db:"time"`
	Amount        fixedpoint.Value `json:"amount" db:"amount"`
	Asset         string           `json:"asset" db:"asset"`
	Address       string           `json:"address" db:"address"`
	AddressTag    string           `json:"addressTag"`
	TransactionID string           `json:"transactionID" db:"txn_id"`
	Status        DepositStatus    `json:"status"`

	RawStatus string `json:"rawStatus"`

	// Required confirm for unlock balance
	UnlockConfirm int `json:"unlockConfirm"`

	// Confirmation format = "current/required", for example: "7/16"
	Confirmation string `json:"confirmation"`

	Network string `json:"network,omitempty"`
}

func (d Deposit) GetCurrentConfirmation() (current int, required int) {
	if len(d.Confirmation) == 0 {
		return 0, 0
	}

	strs := strings.Split(d.Confirmation, "/")
	if len(strs) < 2 {
		return 0, 0
	}

	current, _ = strconv.Atoi(strs[0])
	required, _ = strconv.Atoi(strs[1])
	return current, required
}

func (d Deposit) EffectiveTime() time.Time {
	return d.Time.Time()
}

func (d *Deposit) ObjectID() string {
	return "deposit-" + d.Exchange.String() + "-" + d.Asset + "-" + d.Address + "-" + d.TransactionID
}

func (d Deposit) String() (o string) {
	o = fmt.Sprintf("%s deposit %s %v <- ", d.Exchange, d.Asset, d.Amount)

	if len(d.AddressTag) > 0 {
		o += fmt.Sprintf("%s (tag: %s) at %s", d.Address, d.AddressTag, d.Time.Time())
	} else {
		o += fmt.Sprintf("%s at %s", d.Address, d.Time.Time())
	}

	if len(d.TransactionID) > 0 {
		o += fmt.Sprintf("txID: %s", cutstr(d.TransactionID, 12, 4, 4))
	}
	if len(d.Status) > 0 {
		o += "status: " + string(d.Status)
	}

	return o
}

func (d *Deposit) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField

	if len(d.TransactionID) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "TransactionID",
			Value: d.TransactionID,
			Short: false,
		})
	}

	if len(d.Status) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Status",
			Value: string(d.Status) + " " + d.Status.SlackEmoji(),
			Short: false,
		})
	}

	if len(d.Confirmation) > 0 {
		text := d.Confirmation
		if d.UnlockConfirm > 0 {
			text = fmt.Sprintf("%s (unlock %d)", d.Confirmation, d.UnlockConfirm)
		}
		fields = append(fields, slack.AttachmentField{
			Title: "Confirmation",
			Value: text,
			Short: false,
		})
	}

	if len(d.Exchange) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Exchange",
			Value: d.Exchange.String(),
			Short: false,
		})
	}

	if len(d.Network) > 0 {
		fields = append(fields, slack.AttachmentField{
			Title: "Network",
			Value: d.Network,
			Short: false,
		})
	}

	fields = append(fields, slack.AttachmentField{
		Title: "Amount",
		Value: d.Amount.String() + " " + d.Asset,
		Short: false,
	})

	return slack.Attachment{
		Color:       depositStatusSlackColor(d.Status),
		Fallback:    "",
		ID:          0,
		Title:       fmt.Sprintf("Deposit %s %s To %s", d.Amount.String(), d.Asset, d.Address),
		TitleLink:   getExplorerURL(d.Network, d.TransactionID),
		Pretext:     "",
		Text:        "",
		ImageURL:    "",
		ThumbURL:    "",
		ServiceName: "",
		ServiceIcon: "",
		FromURL:     "",
		OriginalURL: "",
		// ServiceName: "",
		// ServiceIcon: "",
		// FromURL:     "",
		// OriginalURL: "",
		Fields:     fields,
		Actions:    nil,
		MarkdownIn: nil,
		Blocks:     slack.Blocks{},
		Footer:     fmt.Sprintf("Apply Time: %s", d.Time.Time().Format(time.RFC3339)),
		FooterIcon: ExchangeFooterIcon(d.Exchange),
		Ts:         "",
	}
}

func getExplorerURL(network string, txID string) string {
	switch strings.ToUpper(network) {
	case "BTC":
		return getBitcoinNetworkExplorerURL(txID)
	case "BSC":
		return getBscNetworkExplorerURL(txID)
	case "ETH", "ETHEREUM":
		// binance uses "ETH"
		// max uses "ethereum"
		return getEthNetworkExplorerURL(txID)
	case "ARB", "ARBITRUM":
		// binance uses "ARB"
		// max uses "ARBITRUM"
		return getArbitrumExplorerURL(txID)

	}

	return ""
}

func getArbitrumExplorerURL(txID string) string {
	return "https://arbiscan.io/tx/" + txID
}

func getEthNetworkExplorerURL(txID string) string {
	return "https://etherscan.io/tx/" + txID
}

func getBitcoinNetworkExplorerURL(txID string) string {
	return "https://www.blockchain.com/explorer/transactions/btc/" + txID
}

func getBscNetworkExplorerURL(txID string) string {
	return "https://bscscan.com/tx/" + txID
}

func depositStatusSlackColor(status DepositStatus) string {
	switch status {

	case DepositSuccess:
		return "good"

	case DepositRejected:
		return "red"

	default:
		return "gray"

	}
}
