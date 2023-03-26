package xfunding

import (
	"fmt"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/style"
)

type FundingFee struct {
	Asset  string           `json:"asset"`
	Amount fixedpoint.Value `json:"amount"`
	Txn    int64            `json:"txn"`
	Time   time.Time        `json:"time"`
}

func (f *FundingFee) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Title: "Funding Fee " + fmt.Sprintf("%s %s", style.PnLSignString(f.Amount), f.Asset),
		Color: style.PnLColor(f.Amount),
		// Pretext:       "",
		// Text:  text,
		Fields: []slack.AttachmentField{},
		Footer: fmt.Sprintf("Transation ID: %d Transaction Time %s", f.Txn, f.Time.Format(time.RFC822)),
	}
}
