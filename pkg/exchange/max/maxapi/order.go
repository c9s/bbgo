package max

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type WalletType string

const (
	WalletTypeSpot   WalletType = "spot"
	WalletTypeMargin WalletType = "m"
)

type OrderByType string

const (
	OrderByAsc           OrderByType = "asc"
	OrderByDesc          OrderByType = "desc"
	OrderByAscUpdatedAt  OrderByType = "asc_updated_at"
	OrderByDescUpdatedAt OrderByType = "desc_updated_at"
)

type OrderStateToQuery int

const (
	All = iota
	Active
	Closed
)

type OrderState string

const (
	OrderStateDone = OrderState("done")

	OrderStateCancel     = OrderState("cancel")
	OrderStateWait       = OrderState("wait")
	OrderStateConvert    = OrderState("convert")
	OrderStateFinalizing = OrderState("finalizing")
	OrderStateFailed     = OrderState("failed")
)

type OrderType string

// Order types that the API can return.
const (
	OrderTypeMarket     = OrderType("market")
	OrderTypeLimit      = OrderType("limit")
	OrderTypePostOnly   = OrderType("post_only")
	OrderTypeStopLimit  = OrderType("stop_limit")
	OrderTypeStopMarket = OrderType("stop_market")
	OrderTypeIOCLimit   = OrderType("ioc_limit")
)

type QueryOrderOptions struct {
	GroupID int
	Offset  int
	Limit   int
	Page    int
	OrderBy string
}

// OrderService manages the Order endpoint.
type OrderService struct {
	client requestgen.AuthenticatedAPIClient
}

type SubmitOrder struct {
	Side      string    `json:"side"`
	Market    string    `json:"market"`
	Price     string    `json:"price"`
	StopPrice string    `json:"stop_price,omitempty"`
	OrderType OrderType `json:"ord_type"`
	Volume    string    `json:"volume"`
	GroupID   uint32    `json:"group_id,omitempty"`
	ClientOID string    `json:"client_oid,omitempty"`
}

// Order represents one returned order (POST order/GET order/GET orders) on the max platform.
type Order struct {
	ID              uint64                     `json:"id,omitempty"`
	WalletType      WalletType                 `json:"wallet_type,omitempty"`
	Side            string                     `json:"side"`
	OrderType       OrderType                  `json:"ord_type"`
	Price           fixedpoint.Value           `json:"price,omitempty"`
	StopPrice       fixedpoint.Value           `json:"stop_price,omitempty"`
	AveragePrice    fixedpoint.Value           `json:"avg_price,omitempty"`
	State           OrderState                 `json:"state,omitempty"`
	Market          string                     `json:"market,omitempty"`
	Volume          fixedpoint.Value           `json:"volume"`
	RemainingVolume fixedpoint.Value           `json:"remaining_volume,omitempty"`
	ExecutedVolume  fixedpoint.Value           `json:"executed_volume,omitempty"`
	TradesCount     int64                      `json:"trades_count,omitempty"`
	GroupID         uint32                     `json:"group_id,omitempty"`
	ClientOID       string                     `json:"client_oid,omitempty"`
	CreatedAt       types.MillisecondTimestamp `json:"created_at"`
	UpdatedAt       types.MillisecondTimestamp `json:"updated_at"`
}
