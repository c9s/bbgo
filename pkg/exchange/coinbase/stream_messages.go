package coinbase

// https://docs.cdp.coinbase.com/exchange/docs/websocket-channels
import (
	"strings"
	"time"

	api "github.com/c9s/bbgo/pkg/exchange/coinbase/api/v1"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// TODO: Level3 channels

type MessageType string
type SequenceNumberType uint64

type messageBaseType struct {
	Type MessageType `json:"type"`
}

type seqenceMessageType struct {
	messageBaseType

	ProductID string             `json:"product_id"`
	Sequence  SequenceNumberType `json:"sequence"`
}

func (s *seqenceMessageType) QuoteCurrency() string {
	splits := strings.Split(s.ProductID, "-")
	if len(splits) == 2 {
		return splits[1]
	}
	return ""
}

// Websocket message types
// heartbeat channel
type HeartbeatMessage struct {
	seqenceMessageType

	LastTradeID int       `json:"last_trade_id"`
	Time        time.Time `json:"time"`
}

// status channel
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

// auction channel
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

// rfq_matches channel
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

// ticker channel
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

func (msg *TickerMessage) Trade() types.Trade {
	var side types.SideType
	switch msg.Side {
	case "buy":
		side = types.SideTypeBuy
	case "sell":
		side = types.SideTypeSell
	default:
		side = types.SideType(msg.Side)
	}
	isBuyer := side == types.SideTypeBuy
	quoteQuantity := msg.Price.Mul(msg.LastSize)
	return types.Trade{
		Exchange:      types.ExchangeCoinBase,
		Price:         msg.Price,
		Quantity:      msg.LastSize,
		QuoteQuantity: quoteQuantity,
		Side:          side,
		Symbol:        toGlobalSymbol(msg.ProductID),
		IsBuyer:       isBuyer,
		IsMaker:       !isBuyer,
		Time:          types.Time(msg.Time),
		FeeCurrency:   msg.QuoteCurrency(),
		Fee:           fixedpoint.Zero, // not available
	}
}

// full channel
type ReceivedMessage struct {
	seqenceMessageType

	ClientOid string       `json:"client-oid"`
	OrderID   string       `json:"order_id"`
	OrderType string       `json:"order_type"`
	Side      api.SideType `json:"side"`
	Time      time.Time    `json:"time"`

	// limit order fields
	Size  fixedpoint.Value `json:"size,omitempty"`
	Price fixedpoint.Value `json:"price,omitempty"`

	// market order fields
	Funds fixedpoint.Value `json:"funds,omitempty"`
}

func (m *ReceivedMessage) IsMarketOrder() bool {
	return !m.Funds.IsZero()
}

func (m *ReceivedMessage) ToGlobalOrder() types.Order {
	return types.Order{
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: m.ClientOid,
		},
		Exchange:  types.ExchangeCoinBase,
		IsWorking: false,
	}
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
	Side          api.SideType     `json:"side"`
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

	UserID    string `json:"user_id"`
	ProfileID string `json:"profile_id"`

	// extra fields for taker
	TakerUserID    string           `json:"taker_user_id,omitempty"`
	TakerProfileID string           `json:"taker_profile_id,omitempty"`
	TakerFeeRate   fixedpoint.Value `json:"taker_fee_rate,omitempty"`

	// extra fields for maker
	MakerUserID    string           `json:"maker_user_id,omitempty"`
	MakerProfileID string           `json:"maker_profile_id,omitempty"`
	MakerFeeRate   fixedpoint.Value `json:"maker_fee_rate,omitempty"`
}

func (msg *MatchMessage) Trade() types.Trade {
	var side types.SideType
	switch msg.Side {
	case "buy":
		side = types.SideTypeBuy
	case "sell":
		side = types.SideTypeSell
	default:
		side = types.SideType(msg.Side)
	}
	quoteQuantity := msg.Size.Mul(msg.Price)
	return types.Trade{
		Exchange:      types.ExchangeCoinBase,
		Price:         msg.Price,
		Quantity:      msg.Size,
		QuoteQuantity: quoteQuantity,
		Side:          side,
		Symbol:        toGlobalSymbol(msg.ProductID),
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       msg.IsAuthMaker(),
		Time:          types.Time(msg.Time),
		FeeCurrency:   msg.QuoteCurrency(),
		Fee:           quoteQuantity.Mul(msg.FeeRate()),
	}
}

func (m *MatchMessage) isAuth() bool {
	return len(m.TakerUserID) > 0 || len(m.MakerUserID) > 0
}

func (m *MatchMessage) IsAuthMaker() bool {
	// not authenticated
	if !m.isAuth() {
		return false
	}
	return len(m.MakerUserID) > 0
}

// https://help.coinbase.com/en/exchange/trading-and-funding/exchange-fees
func (m *MatchMessage) FeeRate() fixedpoint.Value {
	if !m.isAuth() {
		// not available
		return fixedpoint.Zero
	}
	if m.IsAuthMaker() {
		return m.MakerFeeRate
	}
	return m.TakerFeeRate
}

type ChangeMessage struct {
	seqenceMessageType

	Reason  string           `json:"reason"` // "STP" or "modify_order"
	Time    time.Time        `json:"time"`
	OrderID string           `json:"order_id"`
	Side    api.SideType     `json:"side"`
	OldSize fixedpoint.Value `json:"old_size"`
	NewSize fixedpoint.Value `json:"new_size"`

	// STP fields
	Price fixedpoint.Value `json:"price,omitempty"`

	// modify_order fields
	OldPrice fixedpoint.Value `json:"old_price,omitempty"`
	NewPrice fixedpoint.Value `json:"new_price,omitempty"`
}

func (m *ChangeMessage) IsStp() bool {
	return m.Reason == "STP"
}

func (m *ChangeMessage) IsModifyOrder() bool {
	return m.Reason == "modify_order"
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

// balance channel
type BalanceMessage struct {
	messageBaseType

	AccountID string           `json:"account_id"`
	Currency  string           `json:"currency"`
	Holds     fixedpoint.Value `json:"holds"`
	Available fixedpoint.Value `json:"available"`
	Updated   types.Time       `json:"updated"`
	Timestamp types.Time       `json:"timestamp"`
}

// level2 channel
type OrderBookSnapshotMessage struct {
	messageBaseType

	ProductID string               `json:"product_id"`
	Bids      [][]fixedpoint.Value `json:"bids"` // [["price", "size"], ...]
	Asks      [][]fixedpoint.Value `json:"asks"` // [["price", "size"], ...]
}

type OrderBookUpdateMessage struct {
	messageBaseType

	ProductID string     `json:"product_id"`
	Time      time.Time  `json:"time"`
	Changes   [][]string `json:"changes"` // [["side", "price", "size"], ...]
}
