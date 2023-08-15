package bybitapi

import (
	"time"

	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

//go:generate GetRequest -url "/v5/order/history" -type GetOrderHistoriesRequest -responseDataType .OrdersResponse
type GetOrderHistoriesRequest struct {
	client requestgen.AuthenticatedAPIClient

	category Category `param:"category,query" validValues:"spot"`

	symbol      *string `param:"symbol,query"`
	orderId     *string `param:"orderId,query"`
	orderLinkId *string `param:"orderLinkId,query"`
	// orderFilter supports 3 types of Order:
	// 1. active order, 2. StopOrder: conditional order, 3. tpslOrder: spot TP/SL order
	// Normal spot: return Order active order by default
	// Others: all kinds of orders by default
	orderFilter *string `param:"orderFilter,query"`
	// orderStatus if the account belongs to Normal spot, orderStatus is not supported.
	//// For other accounts, return all status orders if not explicitly passed.
	orderStatus *OrderStatus `param:"orderStatus,query"`

	// startTime must
	// Normal spot is not supported temporarily
	// startTime and endTime must be passed together
	// If not passed, query the past 7 days data by default
	// For each request, startTime and endTime interval should be less then 7 days
	startTime *time.Time `param:"startTime,query,milliseconds"`

	// endTime for each request, startTime and endTime interval should be less then 7 days
	endTime *time.Time `param:"endTime,query,milliseconds"`

	// limit for data size per page. [1, 50]. Default: 20
	limit *uint64 `param:"limit,query"`
	// cursor uses the nextPageCursor token from the response to retrieve the next page of the result set
	cursor *string `param:"cursor,query"`
}

// NewGetOrderHistoriesRequest is descending order by createdTime
func (c *RestClient) NewGetOrderHistoriesRequest() *GetOrderHistoriesRequest {
	return &GetOrderHistoriesRequest{
		client:   c,
		category: CategorySpot,
	}
}
