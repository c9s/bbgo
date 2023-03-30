package xfunding

import (
	"fmt"
	"time"

	"github.com/slack-go/slack"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/style"
	"github.com/c9s/bbgo/pkg/types"
)

type ProfitStats struct {
	*types.ProfitStats

	FundingFeeCurrency string           `json:"fundingFeeCurrency"`
	TotalFundingFee    fixedpoint.Value `json:"totalFundingFee"`
	FundingFeeRecords  []FundingFee     `json:"fundingFeeRecords"`
	LastFundingFeeTxn  int64            `json:"lastFundingFeeTxn"`
	LastFundingFeeTime time.Time        `json:"lastFundingFeeTime"`
}

func (s *ProfitStats) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField
	var totalProfit = fmt.Sprintf("Total Funding Fee Profit: %s %s", style.PnLSignString(s.TotalFundingFee), s.FundingFeeCurrency)

	return slack.Attachment{
		Title: totalProfit,
		Color: style.PnLColor(s.TotalFundingFee),
		// Pretext:       "",
		// Text:  text,
		Fields: fields,
		Footer: fmt.Sprintf("Last Funding Fee Transation ID: %d Last Funding Fee Time %s", s.LastFundingFeeTxn, s.LastFundingFeeTime.Format(time.RFC822)),
	}
}

func (s *ProfitStats) AddFundingFee(fee FundingFee) error {
	if s.FundingFeeCurrency == "" {
		s.FundingFeeCurrency = fee.Asset
	} else if s.FundingFeeCurrency != fee.Asset {
		return fmt.Errorf("unexpected error, funding fee currency is not matched, given: %s, wanted: %s", fee.Asset, s.FundingFeeCurrency)
	}

	if s.LastFundingFeeTxn == fee.Txn {
		return errDuplicatedFundingFeeTxnId
	}

	s.FundingFeeRecords = append(s.FundingFeeRecords, fee)
	s.TotalFundingFee = s.TotalFundingFee.Add(fee.Amount)
	s.LastFundingFeeTxn = fee.Txn
	s.LastFundingFeeTime = fee.Time
	return nil
}
