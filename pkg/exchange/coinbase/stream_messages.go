package coinbase

// https://docs.cdp.coinbase.com/exchange/docs/websocket-channels
import (
	"time"

	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// TODOs:
// - Level2 channels
// - Level3 channels

type messageBaseType struct {
	Type string `json:"type"`
}

type seqenceMessageType struct {
	messageBaseType

	ProductID string `json:"product_id"`
	Sequence  int    `json:"sequence"`
}

// Websocket message types
type HeartbeatMessage struct {
	seqenceMessageType

	LastTradeID int       `json:"last_trade_id"`
	Time        time.Time `json:"time"`
}

type StatusMessage struct {
	messageBaseType
	Products []struct {
		ID             string           `json:"id"`
		BaseCurrency   string           `json:"base_currency"`
		QuoteCurrency  string           `json:"quote_currency"`
		BaseIncrement  fixedpoint.Value `json:"base_increment"`  // ex: 0.00000001
		QuoteIncrement fixedpoint.Value `json:"quote_increment"` // ex: 0.01
		DisplayName    string           `json:"display_name"`
		Status         string           `json:"status"`
		StatusMessage  any              `json:"status_message"`
		MinMarketFunds fixedpoint.Value `json:"min_market_funds"` // ex: 10
		PostOnly       bool             `json:"post_only"`
		LimitOnly      bool             `json:"limit_only"`
		CancelOnly     bool             `json:"cancel_only"`
		FxStablecoin   bool             `json:"fx_stablecoin"`
	} `json:"products"`
	Currencies []struct {
		ID            string           `json:"id"`
		Name          string           `json:"name"`
		DisplayName   string           `json:"display_name"`
		MinSize       fixedpoint.Value `json:"min_size"` // ex: 0.01
		Status        string           `json:"status"`
		StatusMessage any              `json:"status_message"`
		MaxPrecision  fixedpoint.Value `json:"max_precision"` // ex: 0.01
		ConvertibleTo []string         `json:"convertible_to"`
		Details       struct {
		} `json:"details"`
		DefaultNetwork    string `json:"default_network"`
		SupportedNetworks []struct {
			ID                    string           `json:"id"`
			Name                  string           `json:"name"`
			Status                string           `json:"status"`
			ContractAddress       string           `json:"contract_address"`
			CryptoTransactionLink string           `json:"crypto_transaction_link"`
			MinWithdrawalAmount   fixedpoint.Value `json:"min_withdrawal_amount"`   // ex: 0.001
			MaxWithdrawalAmount   fixedpoint.Value `json:"max_withdrawal_amount"`   // ex: 300000000
			NetworkConfirmations  int              `json:"network_confirmations"`   // ex: 14
			ProcessingTimeSeconds int              `json:"processing_time_seconds"` // ex: 0
			DestinationTagRegex   string           `json:"destination_tag_regex"`
		} `json:"supported_networks"`
	} `json:"currencies"`
}

type AuctionMessage struct {
	seqenceMessageType

	AuctionState string           `json:"auction_state"`
	BestBidPrice fixedpoint.Value `json:"best_bid_price"` // ex: 333.98
	BestBidSize  fixedpoint.Value `json:"best_bid_size"`  // ex: 4.39088265
	BestAskPrice fixedpoint.Value `json:"best_ask_price"`
	BestAskSize  fixedpoint.Value `json:"best_ask_size"`
	OpenPrice    fixedpoint.Value `json:"open_price"`
	OpenSize     fixedpoint.Value `json:"open_size"`
	CanOpen      string           `json:"can_open"`
	Timestamp    time.Time        `json:"timestamp"` // ex: "2015-11-14T20:46:03.511254Z"
}

type RfqMessage struct {
	messageBaseType

	MakerOrderID string           `json:"maker_order_id"`
	TakerOrderID string           `json:"taker_order_id"`
	Time         time.Time        `json:"time"`
	TradeID      int              `json:"trade_id"`
	ProductID    string           `json:"product_id"`
	Size         fixedpoint.Value `json:"size"`
	Price        fixedpoint.Value `json:"price"`
	Side         api.SideType     `json:"side"`
}

type TickerMessage struct {
	seqenceMessageType

	Price       fixedpoint.Value `json:"price"`
	Open24H     fixedpoint.Value `json:"open_24h"` // ex: 1310.79
	Volume24H   fixedpoint.Value `json:"volume_24h"`
	Low24H      fixedpoint.Value `json:"low_24h"`
	High24H     fixedpoint.Value `json:"high_24h"`
	Volume30D   fixedpoint.Value `json:"volume_30d"`
	BestBid     fixedpoint.Value `json:"best_bid"`
	BestBidSize fixedpoint.Value `json:"best_bid_size"`
	BestAsk     fixedpoint.Value `json:"best_ask"`
	BestAskSize fixedpoint.Value `json:"best_ask_size"`
	Side        api.SideType     `json:"side"`
	Time        time.Time        `json:"time"`
	TradeID     int              `json:"trade_id"`
	LastSize    fixedpoint.Value `json:"last_size"`
}

type ReceivedLimitOrderMessage struct {
	seqenceMessageType

	ClientOid string       `json:"client-oid"`
	OrderID   string       `json:"order_id"`
	OrderType string       `json:"order_type"`
	Side      api.SideType `json:"side"`
	Time      time.Time    `json:"time"`

	// limit order fields
	Size  fixedpoint.Value `json:"size"`
	Price fixedpoint.Value `json:"price"`
}

type ReceivedMarketOrderMessage struct {
	seqenceMessageType

	ClientOid string       `json:"client-oid"`
	OrderID   string       `json:"order_id"`
	OrderType string       `json:"order_type"`
	Side      api.SideType `json:"side"`
	Time      time.Time    `json:"time"`

	// market order fields
	Funds fixedpoint.Value `json:"funds"`
}

type OpenMessage struct {
	seqenceMessageType

	Time          time.Time        `json:"time"`
	OrderID       string           `json:"order_id"`
	Price         fixedpoint.Value `json:"price"`
	RemainingSize fixedpoint.Value `json:"remaining_size"`
	Side          api.SideType     `json:"side"`
}

type DoneMessage struct {
	seqenceMessageType

	Time          time.Time        `json:"time"`
	Price         fixedpoint.Value `json:"price"`
	OrderID       string           `json:"order_id"`
	Reason        string           `json:"reason"` // filled, canceled
	Side          string           `json:"side"`
	RemainingSize fixedpoint.Value `json:"remaining_size"`
	CancelReason  string           `json:"cancel_reason,omitempty"` // non-empty if reason is canceled
}

type MatchMessage struct {
	seqenceMessageType

	TradeID      int              `json:"trade_id"`
	MakerOrderID string           `json:"maker_order_id"`
	TakerOrderID string           `json:"taker_order_id"`
	Time         time.Time        `json:"time"`
	Size         fixedpoint.Value `json:"size"`
	Price        fixedpoint.Value `json:"price"`
	Side         api.SideType     `json:"side"`
}

type AuthTakerMatchMessage struct {
	MatchMessage

	UserID    string `json:"user_id"`
	ProfileID string `json:"profile_id"`

	// extra fields for taker
	TakerUserID    string `json:"taker_user_id"`
	TakerProfileID string `json:"taker_profile_id"`
	TakerFeeRate   string `json:"taker_fee_rate"`
}

type AuthMakerMatchMessage struct {
	MatchMessage

	UserID    string `json:"user_id"`
	ProfileID string `json:"profile_id"`

	// extra fields for maker
	MakerUserID    string `json:"maker_user_id"`
	MakerProfileID string `json:"maker_profile_id"`
	MakerFeeRate   string `json:"maker_fee_rate"`
}

type changeMessageType struct {
	seqenceMessageType

	Reason  string           `json:"reason"` // "STP" or "modify_order"
	Time    time.Time        `json:"time"`
	OrderID string           `json:"order_id"`
	Side    api.SideType     `json:"side"`
	OldSize fixedpoint.Value `json:"old_size"`
	NewSize fixedpoint.Value `json:"new_size"`
}

type StpChangeMessage struct {
	changeMessageType

	// STP fields
	Price fixedpoint.Value `json:"price"`
}

type ModifyOrderChangeMessage struct {
	changeMessageType

	// modify_order fields
	OldPrice fixedpoint.Value `json:"old_price"`
	NewPrice fixedpoint.Value `json:"new_price"`
}

type ActiveMessage struct {
	messageBaseType

	ProductID string           `json:"product_id"`
	Timestamp string           `json:"timestamp"` // ex: "1483736448.299000"
	UserID    string           `json:"user_id"`
	ProfileID string           `json:"profile_id"`
	OrderID   string           `json:"order_id"`
	StopType  string           `json:"stop_type"`
	Side      api.SideType     `json:"side"`
	StopPrice fixedpoint.Value `json:"stop_price"`
	Size      fixedpoint.Value `json:"size"`
	Funds     fixedpoint.Value `json:"funds"`
	Private   bool             `json:"private"`
}
