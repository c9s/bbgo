package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type BalanceDetail struct {
	Available   fixedpoint.Value `json:"availBal"`
	AvailEquity fixedpoint.Value `json:"availEq"`

	// BorrowFrozen is Potential borrowing IMR of the account in USD
	BorrowFrozen      fixedpoint.Value `json:"borrowFroz"`
	CashBalance       fixedpoint.Value `json:"cashBal"`
	Currency          string           `json:"ccy"`
	CrossLiab         fixedpoint.Value `json:"crossLiab,omitempty"`
	DisEquity         fixedpoint.Value `json:"disEq,omitempty"`
	Equity            fixedpoint.Value `json:"eq,omitempty"`
	EquityInUSD       fixedpoint.Value `json:"eqUsd"`
	SmtSyncEquity     fixedpoint.Value `json:"smtSyncEq,omitempty"`
	SpotCopyTradingEq fixedpoint.Value `json:"spotCopyTradingEq,omitempty"`

	FixedBalance  fixedpoint.Value `json:"fixedBal"`
	FrozenBalance fixedpoint.Value `json:"frozenBal"`

	Imr           string           `json:"imr,omitempty"`
	Interest      fixedpoint.Value `json:"interest"`
	IsoEquity     fixedpoint.Value `json:"isoEq,omitempty"`
	IsoLiability  fixedpoint.Value `json:"isoLiab,omitempty"`
	IsoUpl        fixedpoint.Value `json:"isoUpl,omitempty"`
	Liability     fixedpoint.Value `json:"liab,omitempty"`
	MaxLoan       fixedpoint.Value `json:"maxLoan,omitempty"`
	MgnRatio      fixedpoint.Value `json:"mgnRatio,omitempty"`
	Mmr           string           `json:"mmr,omitempty"`
	NotionalLever string           `json:"notionalLever"`

	// OrderFrozen is margin frozen for open orders
	OrderFrozen             fixedpoint.Value           `json:"ordFrozen"`
	RewardBal               fixedpoint.Value           `json:"rewardBal,omitempty"`
	SpotInUseAmt            fixedpoint.Value           `json:"spotInUseAmt,omitempty"`
	ClSpotInUseAmt          fixedpoint.Value           `json:"clSpotInUseAmt,omitempty"`
	MaxSpotInUse            fixedpoint.Value           `json:"maxSpotInUse,omitempty"`
	SpotIsoBal              fixedpoint.Value           `json:"spotIsoBal,omitempty"`
	StgyEquity              fixedpoint.Value           `json:"stgyEq"`
	Twap                    string                     `json:"twap,omitempty"`
	UpdateTime              types.MillisecondTimestamp `json:"uTime"`
	UnrealizedProfitAndLoss fixedpoint.Value           `json:"upl"`
	UplLiab                 fixedpoint.Value           `json:"uplLiab"`
	SpotBal                 fixedpoint.Value           `json:"spotBal"`
	OpenAvgPx               fixedpoint.Value           `json:"openAvgPx,omitempty"`
	AccAvgPx                fixedpoint.Value           `json:"accAvgPx,omitempty"`
	SpotUpl                 fixedpoint.Value           `json:"spotUpl,omitempty"`
	SpotUplRatio            fixedpoint.Value           `json:"spotUplRatio,omitempty"`
	TotalPnl                fixedpoint.Value           `json:"totalPnl,omitempty"`
	TotalPnlRatio           fixedpoint.Value           `json:"totalPnlRatio,omitempty"`
}

type Account struct {
	TotalEquityInUSD      fixedpoint.Value           `json:"totalEq,omitempty"`
	AdjustEquity          fixedpoint.Value           `json:"adjEq,omitempty"`
	UpdateTime            types.MillisecondTimestamp `json:"uTime,omitempty"`
	MarginRatio           fixedpoint.Value           `json:"mgnRatio,omitempty"`
	NotionalUsd           fixedpoint.Value           `json:"notionalUsd,omitempty"`
	NotionalUsdForBorrow  fixedpoint.Value           `json:"notionalUsdForBorrow,omitempty"`
	NotionalUsdForSwap    fixedpoint.Value           `json:"notionalUsdForSwap,omitempty"`
	NotionalUsdForFutures fixedpoint.Value           `json:"notionalUsdForFutures,omitempty"`
	NotionalUsdForOption  fixedpoint.Value           `json:"notionalUsdForOption,omitempty"`
	BorrowFroz            fixedpoint.Value           `json:"borrowFroz,omitempty"`

	UnrealizedPnl fixedpoint.Value `json:"upl,omitempty"`
	Details       []BalanceDetail  `json:"details"`

	TotalInitialMargin          fixedpoint.Value `json:"imr,omitempty"`
	TotalMaintMargin            fixedpoint.Value `json:"mmr,omitempty"`
	TotalOpenOrderInitialMargin fixedpoint.Value `json:"ordFroz,omitempty"`
}

//go:generate GetRequest -url "/api/v5/account/balance" -type GetAccountBalanceRequest -responseDataType []Account
type GetAccountBalanceRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetAccountBalanceRequest() *GetAccountBalanceRequest {
	return &GetAccountBalanceRequest{
		client: c,
	}
}
