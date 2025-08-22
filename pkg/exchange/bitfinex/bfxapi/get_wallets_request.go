package bfxapi

import (
	"encoding/json"

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

// Wallet type could be (exchange, margin, funding)
type WalletType string

const (
	WalletTypeExchange WalletType = "exchange"
	WalletTypeMargin   WalletType = "margin"
	WalletTypeFunding  WalletType = "funding"
)

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
//	]
//
// JSON: [
//
//	  "exchange", //WALLET_TYPE
//	  "BTC", //CURRENCY
//	  1.61169184, //BALANCE
//	  0, //UNSETTLED_INTEREST
//	  null, //BALANCE_AVAILABLE
//	  "Exchange 0.01 BTC for USD @ 7804.6", //DESCRIPTION
//	  {
//	    "reason":"TRADE",
//	    "order_id":34988418651,
//	    "order_id_oppo":34990541044,
//	    "trade_price":"7804.6",
//	    "trade_amount":"0.01"
//	  } //META
//	] //WALLET_ARRAY
type WalletResponse struct {
	Type               WalletType       // Wallet type (e.g., "exchange", "margin", etc.)
	Currency           string           // Currency code (e.g., "UST", "BTC", etc.)
	Balance            fixedpoint.Value // Total balance
	UnsettledInterest  fixedpoint.Value // Unsettled interest
	AvailableBalance   fixedpoint.Value // Available balance
	LastChange         string
	LastChangeMetaData *WalletMetaData
}

// UnmarshalJSON maps the JSON array response to the WalletResponse struct fields.
func (r *WalletResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

type WalletMetaData struct {
	Reason string `json:"reason"`
	Data   any    `json:"-"`
}

func (m *WalletMetaData) UnmarshalJSON(data []byte) error {
	var preview struct {
		Reason string `json:"reason"`
	}

	if err := json.Unmarshal(data, &preview); err != nil {
		return err
	}

	switch preview.Reason {
	case "TRADE":
		var tradeDetail WalletTradeDetail
		if err := json.Unmarshal(data, &tradeDetail); err != nil {
			return err
		}

		m.Data = &tradeDetail
	default:
		log.Warnf("unknown wallet meta reason: %s, data: %s", preview.Reason, data)
	}

	return nil
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
