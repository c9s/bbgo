package binanceapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type MarginLiquidationRecord struct {
	AveragePrice     fixedpoint.Value           `json:"avgPrice"`
	ExecutedQuantity fixedpoint.Value           `json:"executedQty"`
	OrderId          uint64                     `json:"orderId"`
	Price            fixedpoint.Value           `json:"price"`
	Quantity         fixedpoint.Value           `json:"qty"`
	Side             SideType                   `json:"side"`
	Symbol           string                     `json:"symbol"`
	TimeInForce      string                     `json:"timeInForce"`
	IsIsolated       bool                       `json:"isIsolated"`
	UpdatedTime      types.MillisecondTimestamp `json:"updatedTime"`
}

//go:generate requestgen -method GET -url "/sapi/v1/margin/forceLiquidationRec" -type GetMarginLiquidationHistoryRequest -responseType .RowsResponse -responseDataField Rows -responseDataType []MarginLiquidationRecord
type GetMarginLiquidationHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	isolatedSymbol *string    `param:"isolatedSymbol"`
	startTime      *time.Time `param:"startTime,milliseconds"`
	endTime        *time.Time `param:"endTime,milliseconds"`
	size           *int       `param:"size"`
	current        *int       `param:"current"`
}

func (c *RestClient) NewGetMarginLiquidationHistoryRequest() *GetMarginLiquidationHistoryRequest {
	return &GetMarginLiquidationHistoryRequest{client: c}
}
