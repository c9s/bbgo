package bybitapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type TradesResponse struct {
	NextPageCursor string   `json:"nextPageCursor"`
	Category       Category `json:"category"`
	List           []Trade  `json:"list"`
}

type Trade struct {
	Symbol    string    `json:"symbol"`
	OrderId   string    `json:"orderId"`
	Side      Side      `json:"side"`
	OrderType OrderType `json:"orderType"`
	// ExecFee is supported on restful API v5, but not on websocket API.
	ExecFee   fixedpoint.Value           `json:"execFee"`
	ExecId    string                     `json:"execId"`
	ExecPrice fixedpoint.Value           `json:"execPrice"`
	ExecQty   fixedpoint.Value           `json:"execQty"`
	ExecTime  types.MillisecondTimestamp `json:"execTime"`
	IsMaker   bool                       `json:"isMaker"`
}

//go:generate GetRequest -url "/v5/execution/list" -type GetExecutionListRequest -responseDataType .TradesResponse -rateLimiter 5+15/1s
type GetExecutionListRequest struct {
	client requestgen.AuthenticatedAPIClient

	category Category `param:"category,query" validValues:"spot"`

	symbol  *string `param:"symbol,query"`
	orderId *string `param:"orderId,query"`

	// startTime the start timestamp (ms)
	// startTime and endTime are not passed, return 7 days by default;
	// Only startTime is passed, return range between startTime and startTime+7 days
	// Only endTime is passed, return range between endTime-7 days and endTime
	// If both are passed, the rule is endTime - startTime <= 7 days
	startTime *time.Time `param:"startTime,query,milliseconds"`

	// endTime the end timestamp (ms)
	endTime *time.Time `param:"endTime,query,milliseconds"`

	// limit for data size per page. [1, 100]. Default: 50
	limit *uint64 `param:"limit,query"`
	// cursor. Use the nextPageCursor token from the response to retrieve the next page of the result set
	cursor *string `param:"cursor,query"`
}

// NewGetExecutionListRequest query users' execution records, sorted by execTime in descending order. However,
// for Classic spot, they are sorted by execId in descending order.
func (c *RestClient) NewGetExecutionListRequest() *GetExecutionListRequest {
	return &GetExecutionListRequest{
		client:   c,
		category: CategorySpot,
	}
}
