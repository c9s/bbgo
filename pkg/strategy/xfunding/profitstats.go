package xfunding

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type FundingFee struct {
	Asset  string           `json:"asset"`
	Amount fixedpoint.Value `json:"amount"`
	Txn    int64            `json:"txn"`
}

type ProfitStats struct {
	*types.ProfitStats

	FundingFeeCurrency string           `json:"fundingFeeCurrency"`
	TotalFundingFee    fixedpoint.Value `json:"totalFundingFee"`
	FundingFeeRecords  []FundingFee     `json:"fundingFeeRecords"`
	LastFundingFeeTxn  int64            `json:"lastFundingFeeTxn"`
}

func (s *ProfitStats) AddFundingFee(fee FundingFee) error {
	if s.FundingFeeCurrency == "" {
		s.FundingFeeCurrency = fee.Asset
	} else if s.FundingFeeCurrency != fee.Asset {
		return fmt.Errorf("unexpected error, funding fee currency is not matched, given: %s, wanted: %s", fee.Asset, s.FundingFeeCurrency)
	}

	s.FundingFeeRecords = append(s.FundingFeeRecords, fee)
	s.TotalFundingFee = s.TotalFundingFee.Add(fee.Amount)
	s.LastFundingFeeTxn = fee.Txn
	return nil
}
