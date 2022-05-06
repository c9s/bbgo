package binanceapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type ForceLiquidationRecord2 struct {
	Asset             string                     `json:"asset"`
	DailyInterestRate fixedpoint.Value           `json:"dailyInterestRate"`
	Timestamp         types.MillisecondTimestamp `json:"timestamp"`
	VipLevel          int                        `json:"vipLevel"`
}

type ForceLiquidationRecord struct {
	AvgPrice    fixedpoint.Value           `json:"avgPrice"`
	ExecutedQty fixedpoint.Value           `json:"executedQty"`
	OrderId     uint64                     `json:"orderId"`
	Price       fixedpoint.Value           `json:"price"`
	Qty         fixedpoint.Value           `json:"qty"`
	Side        SideType                   `json:"side"`
	Symbol      string                     `json:"symbol"`
	TimeInForce string                     `json:"timeInForce"`
	IsIsolated  bool                       `json:"isIsolated"`
	UpdatedTime types.MillisecondTimestamp `json:"updatedTime"`
}

//go:generate requestgen -method GET -url "/sapi/v1/margin/interestRateHistory" -type GetForceLiquidationRecordRequest -responseType []ForceLiquidationRecord
type GetForceLiquidationRecordRequest struct {
	client requestgen.AuthenticatedAPIClient

	asset     string     `param:"asset"`
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
}

func (c *RestClient) NewGetForceLiquidationRecordRequest() *GetForceLiquidationRecordRequest {
	return &GetForceLiquidationRecordRequest{client: c}
}
