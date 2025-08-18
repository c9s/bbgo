package bfxapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type WalletTradeDetail struct {
	Reason      string           `json:"reason"`
	OrderId     int64            `json:"order_id"`
	OrderIdOppo int64            `json:"order_id_oppo"`
	TradePrice  fixedpoint.Value `json:"trade_price"`
	TradeAmount fixedpoint.Value `json:"trade_amount"`
	OrderCid    int64            `json:"order_cid"`
	OrderGid    int64            `json:"order_gid"`
}

// WalletResponse represents the response structure for the authenticated wallets endpoint.
// API response example:
// [
//
//	"exchange",          // TYPE
//	"UST",               // CURRENCY
//	19788.6529257,       // BALANCE
//	0,                   // UNSETTLED_INTEREST
//	19788.6529257        // AVAILABLE_BALANCE
//	"USD",              // MARGIN_CURRENCY (optional)
//
// ]
type WalletResponse struct {
	Type              string           // Wallet type (e.g., "exchange", "margin", etc.)
	Currency          string           // Currency code (e.g., "UST", "BTC", etc.)
	Balance           fixedpoint.Value // Total balance
	UnsettledInterest fixedpoint.Value // Unsettled interest
	AvailableBalance  fixedpoint.Value // Available balance
	LastChange        string
	TradeDetail       *WalletTradeDetail
}

// UnmarshalJSON maps the JSON array response to the WalletResponse struct fields.
func (r *WalletResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

// GetWalletsRequest represents the request structure for the authenticated wallets endpoint.
// API: https://docs.bitfinex.com/reference/rest-auth-wallets
//
//go:generate requestgen -type GetWalletsRequest -method POST -url "/v2/auth/r/wallets" -responseType []WalletResponse
type GetWalletsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

// NewGetWalletsRequest creates a new instance of GetWalletsRequest.
func (c *Client) NewGetWalletsRequest() *GetWalletsRequest {
	return &GetWalletsRequest{
		client: c,
	}
}
