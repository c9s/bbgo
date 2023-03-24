package binanceapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// FuturesIncomeType can be one of the following value:
// TRANSFER, WELCOME_BONUS, REALIZED_PNL, FUNDING_FEE, COMMISSION, INSURANCE_CLEAR, REFERRAL_KICKBACK, COMMISSION_REBATE,
// API_REBATE, CONTEST_REWARD, CROSS_COLLATERAL_TRANSFER, OPTIONS_PREMIUM_FEE,
// OPTIONS_SETTLE_PROFIT, INTERNAL_TRANSFER, AUTO_EXCHANGE,
// DELIVERED_SETTELMENT, COIN_SWAP_DEPOSIT, COIN_SWAP_WITHDRAW, POSITION_LIMIT_INCREASE_FEE
type FuturesIncomeType string

const (
	FuturesIncomeTransfer         FuturesIncomeType = "TRANSFER"
	FuturesIncomeWelcomeBonus     FuturesIncomeType = "WELCOME_BONUS"
	FuturesIncomeFundingFee       FuturesIncomeType = "FUNDING_FEE"
	FuturesIncomeRealizedPnL      FuturesIncomeType = "REALIZED_PNL"
	FuturesIncomeCommission       FuturesIncomeType = "COMMISSION"
	FuturesIncomeReferralKickback FuturesIncomeType = "REFERRAL_KICKBACK"
	FuturesIncomeCommissionRebate FuturesIncomeType = "COMMISSION_REBATE"
	FuturesIncomeApiRebate        FuturesIncomeType = "API_REBATE"
	FuturesIncomeContestReward    FuturesIncomeType = "CONTEST_REWARD"
)

type FuturesIncome struct {
	Symbol     string                     `json:"symbol"`
	IncomeType FuturesIncomeType          `json:"incomeType"`
	Income     fixedpoint.Value           `json:"income"`
	Asset      string                     `json:"asset"`
	Info       string                     `json:"info"`
	Time       types.MillisecondTimestamp `json:"time"`
	TranId     string                     `json:"tranId"`
	TradeId    string                     `json:"tradeId"`
}

//go:generate requestgen -method GET -url "/fapi/v2/positionRisk" -type FuturesGetIncomeHistoryRequest -responseType []FuturesIncome
type FuturesGetIncomeHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol string `param:"symbol"`

	incomeType FuturesIncomeType `param:"incomeType"`

	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`

	limit *uint64 `param:"limit"`
}

func (c *FuturesRestClient) NewFuturesGetIncomeHistoryRequest() *FuturesGetIncomeHistoryRequest {
	return &FuturesGetIncomeHistoryRequest{client: c}
}
