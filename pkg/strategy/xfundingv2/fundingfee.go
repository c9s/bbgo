package xfundingv2

import (
	"fmt"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/style"
)

type FundingFee struct {
	RoundID string           `json:"round_id" db:"round_id"`
	Asset   string           `json:"asset" db:"asset"`
	Amount  fixedpoint.Value `json:"amount" db:"amount"`
	Txn     int64            `json:"txn" db:"txn"`
	Time    time.Time        `json:"time" db:"time"`
}

func (f *FundingFee) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Title:  "Funding Fee " + fmt.Sprintf("%s %s", style.PnLSignString(f.Amount), f.Asset),
		Color:  style.PnLColor(f.Amount),
		Fields: []slack.AttachmentField{},
		Footer: fmt.Sprintf("Transation ID: %d Transaction Time %s", f.Txn, f.Time.Format(time.RFC822)),
	}
}
