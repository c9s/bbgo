package okexapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type LiquidityType string

const (
	LiquidityTypeMaker = "M"
	LiquidityTypeTaker = "T"
)

type Trade struct {
	InstrumentType InstrumentType `json:"instType"`
	InstrumentId   string         `json:"instId"`
	TradeId        types.StrInt64 `json:"tradeId"`
	OrderId        types.StrInt64 `json:"ordId"`
	ClientOrderId  string         `json:"clOrdId"`
	// Data generation time, Unix timestamp format in milliseconds, e.g. 1597026383085.
	Timestamp     types.MillisecondTimestamp `json:"ts"`
	FillTime      types.MillisecondTimestamp `json:"fillTime"`
	FeeCurrency   string                     `json:"feeCcy"`
	Fee           fixedpoint.Value           `json:"fee"`
	BillId        types.StrInt64             `json:"billId"`
	Side          SideType                   `json:"side"`
	ExecutionType LiquidityType              `json:"execType"`

	Tag string `json:"tag"`
	// Last filled price
	FillPrice fixedpoint.Value `json:"fillPx"`
	// Last filled quantity
	FillSize fixedpoint.Value `json:"fillSz"`
	// Index price at the moment of trade execution
	//For cross currency spot pairs, it returns baseCcy-USDT index price. For example, for LTC-ETH, this field returns the index price of LTC-USDT.
	FillIndexPrice fixedpoint.Value `json:"fillIdxPx"`
	FillPnl        string           `json:"fillPnl"`

	// Only applicable to options; return "" for other instrument types
	FillPriceVolume  fixedpoint.Value `json:"fillPxVol"`
	FillPriceUsd     fixedpoint.Value `json:"fillPxUsd"`
	FillMarkVolume   fixedpoint.Value `json:"fillMarkVol"`
	FillForwardPrice fixedpoint.Value `json:"fillFwdPx"`
	FillMarkPrice    fixedpoint.Value `json:"fillMarkPx"`
	PosSide          string           `json:"posSide"`
}

//go:generate GetRequest -url "/api/v5/trade/fills-history" -type GetTransactionHistoryRequest -responseDataType []Trade
type GetTransactionHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	instrumentType InstrumentType `param:"instType,query"`
	instrumentID   *string        `param:"instId,query"`
	orderID        string         `param:"ordId,query"`

	// Underlying and InstrumentFamily Applicable to FUTURES/SWAP/OPTION
	underlying       *string `param:"uly,query"`
	instrumentFamily *string `param:"instFamily,query"`

	after     *string    `param:"after,query"`
	before    *string    `param:"before,query"`
	startTime *time.Time `param:"begin,query,milliseconds"`

	// endTime for each request, startTime and endTime can be any interval, but should be in last 3 months
	endTime *time.Time `param:"end,query,milliseconds"`

	// limit for data size per page. Default: 100
	limit *uint64 `param:"limit,query"`
}

type OrderList []OrderDetails

func (c *RestClient) NewGetTransactionHistoryRequest() *GetTransactionHistoryRequest {
	return &GetTransactionHistoryRequest{
		client:         c,
		instrumentType: InstrumentTypeSpot,
	}
}
