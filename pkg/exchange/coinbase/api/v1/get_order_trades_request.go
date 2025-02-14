package coinbase

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/requestgen"
)

type Trade struct {
	TradeID         int              `json:"trade_id"`
	ProductID       string           `json:"product_id"`
	OrderID         string           `json:"order_id"`
	UserID          string           `json:"user_id"`
	ProfileID       string           `json:"profile_id"`
	Liquidity       Liquidity        `json:"liquidity"`
	Price           fixedpoint.Value `json:"price"`
	Size            fixedpoint.Value `json:"size"`
	Fee             fixedpoint.Value `json:"fee"`
	CreatedAt       time.Time        `json:"created_at"`
	Side            SideType         `json:"side"`
	Settled         bool             `json:"settled"`
	UsdVolume       string           `json:"usd_volume"`
	FundingCurrency string           `json:"funding_currency"`
}

type TradeSnapshot []Trade

//go:generate requestgen -method GET -url "/orders/fills" -type GetOrderTradesRequest -responseType .TradeSnapshot
type GetOrderTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	orderID    string      `param:"order_id,required"`
	limit      int         `param:"limit"`
	before     *string     `param:"before"`
	after      *string     `param:"after"`
	marketType *MarketType `param:"market_type"`
	startDate  *string     `param:"start_date"`
	endDate    *string     `param:"end_date"`
}
