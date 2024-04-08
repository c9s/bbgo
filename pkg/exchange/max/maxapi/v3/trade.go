package v3

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Liquidity string

const (
	LiquidityMaker = "maker"
	LiquidityTaker = "taker"
)

type Trade struct {
	ID                        uint64                     `json:"id" db:"exchange_id"`
	WalletType                WalletType                 `json:"wallet_type,omitempty"`
	Price                     fixedpoint.Value           `json:"price"`
	Volume                    fixedpoint.Value           `json:"volume"`
	Funds                     fixedpoint.Value           `json:"funds"`
	Market                    string                     `json:"market"`
	MarketName                string                     `json:"market_name"`
	CreatedAt                 types.MillisecondTimestamp `json:"created_at"`
	Side                      string                     `json:"side"`
	OrderID                   uint64                     `json:"order_id"`
	Fee                       *fixedpoint.Value          `json:"fee"` // float number in string, could be optional
	FeeCurrency               string                     `json:"fee_currency"`
	FeeDiscounted             bool                       `json:"fee_discounted"`
	Liquidity                 Liquidity                  `json:"liquidity"`
	SelfTradeBidFee           fixedpoint.Value           `json:"self_trade_bid_fee"`
	SelfTradeBidFeeCurrency   string                     `json:"self_trade_bid_fee_currency"`
	SelfTradeBidFeeDiscounted bool                       `json:"self_trade_bid_fee_discounted"`
	SelfTradeBidOrderID       uint64                     `json:"self_trade_bid_order_id"`
}

func (t Trade) IsBuyer() bool {
	return t.Side == "bid"
}

func (t Trade) IsMaker() bool {
	return t.Liquidity == "maker"
}
