package grid2

import (
	"fmt"
	"strconv"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/style"
	"github.com/c9s/bbgo/pkg/types"
)

type GridProfit struct {
	Currency string           `json:"currency"`
	Profit   fixedpoint.Value `json:"profit"`
	Time     time.Time        `json:"time"`
	Order    types.Order      `json:"order"`
}

func (p *GridProfit) String() string {
	return fmt.Sprintf("GRID PROFIT: %f %s @ %s orderID %d", p.Profit.Float64(), p.Currency, p.Time.String(), p.Order.OrderID)
}

func (p *GridProfit) PlainText() string {
	return fmt.Sprintf("Grid profit: %f %s @ %s orderID %d", p.Profit.Float64(), p.Currency, p.Time.String(), p.Order.OrderID)
}

func (p *GridProfit) SlackAttachment() slack.Attachment {
	title := fmt.Sprintf("Grid Profit %s %s", style.PnLSignString(p.Profit), p.Currency)
	return slack.Attachment{
		Title: title,
		Color: "warning",
		Fields: []slack.AttachmentField{
			{
				Title: "OrderID",
				Value: strconv.FormatUint(p.Order.OrderID, 10),
				Short: true,
			},
			{
				Title: "Time",
				Value: p.Time.String(),
				Short: true,
			},
		},
	}
}
