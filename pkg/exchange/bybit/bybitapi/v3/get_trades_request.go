package v3

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type TradesResponse struct {
	List []Trade `json:"list"`
}

type Trade struct {
	Symbol        string                     `json:"symbol"`
	Id            string                     `json:"id"`
	OrderId       string                     `json:"orderId"`
	TradeId       string                     `json:"tradeId"`
	OrderPrice    fixedpoint.Value           `json:"orderPrice"`
	OrderQty      fixedpoint.Value           `json:"orderQty"`
	ExecFee       fixedpoint.Value           `json:"execFee"`
	FeeTokenId    string                     `json:"feeTokenId"`
	CreatTime     types.MillisecondTimestamp `json:"creatTime"`
	IsBuyer       Side                       `json:"isBuyer"`
	IsMaker       OrderType                  `json:"isMaker"`
	MatchOrderId  string                     `json:"matchOrderId"`
	MakerRebate   fixedpoint.Value           `json:"makerRebate"`
	ExecutionTime types.MillisecondTimestamp `json:"executionTime"`
	BlockTradeId  string                     `json:"blockTradeId"`
}

//go:generate GetRequest -url "/spot/v3/private/my-trades" -type GetTradesRequest -responseDataType .TradesResponse
type GetTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol  *string `param:"symbol,query"`
	orderId *string `param:"orderId,query"`
	// Limit default value is 50, max 50
	limit       *uint64    `param:"limit,query"`
	startTime   *time.Time `param:"startTime,query,milliseconds"`
	endTime     *time.Time `param:"endTime,query,milliseconds"`
	fromTradeId *string    `param:"fromTradeId,query"`
	toTradeId   *string    `param:"toTradeId,query"`
}

func (c *Client) NewGetTradesRequest() *GetTradesRequest {
	return &GetTradesRequest{
		client: c.Client,
	}
}
