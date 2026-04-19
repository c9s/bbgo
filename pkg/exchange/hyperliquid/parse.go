package hyperliquid

import (
	"encoding/json"
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type MarketType string

const (
	MarketTypeUnknown MarketType = "unknown"
	MarketTypeSpot    MarketType = "spot"
	MarketTypePerp    MarketType = "perp"
)

// wsFrame is the generic WebSocket message frame from Hyperliquid.
// See https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/websocket/subscriptions
type wsFrame struct {
	Channel    string          `json:"channel"`
	Data       json.RawMessage `json:"data"`
	IsSnapshot *bool           `json:"isSnapshot,omitempty"`
}

// WsLevel is a single price level in the order book.
type WsLevel struct {
	Px fixedpoint.Value `json:"px"` // price
	Sz fixedpoint.Value `json:"sz"` // size
	N  int              `json:"n"`  // number of orders
}

// WsBook is the l2Book snapshot/update format.
type WsBook struct {
	Coin   string                     `json:"coin"`
	Levels [2][]WsLevel               `json:"levels"` // [bids, asks]
	Time   types.MillisecondTimestamp `json:"time"`
}

// WsTrade is a single trade in the trades feed.
type WsTrade struct {
	Coin  string                     `json:"coin"`
	Side  string                     `json:"side"`
	Px    fixedpoint.Value           `json:"px"`
	Sz    fixedpoint.Value           `json:"sz"`
	Hash  string                     `json:"hash"`
	Time  types.MillisecondTimestamp `json:"time"`
	Tid   int64                      `json:"tid"`
	Users [2]string                  `json:"users"` // [buyer, seller]
}

// WsCandle is the candle subscription data format.
type WsCandle struct {
	OpenTime  types.MillisecondTimestamp `json:"t"` // open millis
	CloseTime types.MillisecondTimestamp `json:"T"` // close millis
	Symbol    string                     `json:"s"`
	Interval  string                     `json:"i"`
	O         fixedpoint.Value           `json:"o"`
	C         fixedpoint.Value           `json:"c"`
	H         fixedpoint.Value           `json:"h"`
	L         fixedpoint.Value           `json:"l"`
	V         fixedpoint.Value           `json:"v"` // volume base unit
	N         int                        `json:"n"` // number of trades
}

// WsFill is a single fill in userFills.
type WsFill struct {
	Coin          string                     `json:"coin"`
	Px            fixedpoint.Value           `json:"px"`
	Sz            fixedpoint.Value           `json:"sz"`
	Side          string                     `json:"side"`
	Time          types.MillisecondTimestamp `json:"time"`
	StartPosition string                     `json:"startPosition"`
	Dir           string                     `json:"dir"`
	ClosedPnl     string                     `json:"closedPnl"`
	Hash          string                     `json:"hash"`
	Oid           int64                      `json:"oid"`
	Crossed       bool                       `json:"crossed"`
	Fee           string                     `json:"fee"`
	Tid           int64                      `json:"tid"`
	FeeToken      string                     `json:"feeToken"`
}

// WsUserFills is the userFills subscription data.
type WsUserFills struct {
	IsSnapshot *bool    `json:"isSnapshot,omitempty"`
	User       string   `json:"user"`
	Fills      []WsFill `json:"fills"`
}

// WsBasicOrder is the order payload inside orderUpdates.
type WsBasicOrder struct {
	Coin      string                     `json:"coin"`
	Side      string                     `json:"side"`
	LimitPx   fixedpoint.Value           `json:"limitPx"`
	Sz        fixedpoint.Value           `json:"sz"`
	Oid       int64                      `json:"oid"`
	Timestamp types.MillisecondTimestamp `json:"timestamp"`
	OrigSz    fixedpoint.Value           `json:"origSz"`
	Cloid     string                     `json:"cloid,omitempty"`
}

// WsOrderUpdate is the orderUpdates subscription message.
type WsOrderUpdate struct {
	Order           WsBasicOrder               `json:"order"`
	Status          string                     `json:"status"`
	StatusTimestamp types.MillisecondTimestamp `json:"statusTimestamp"`
}

// ClearinghouseState for balance/position (optional).
type WsClearinghouseState struct {
	AssetPositions []WsAssetPosition `json:"assetPositions"`
	MarginSummary  WsMarginSummary   `json:"marginSummary"`
	Withdrawable   float64           `json:"withdrawable"`
}

type WsAssetPosition struct {
	Type     string     `json:"type"`
	Position WsPosition `json:"position"`
}

type WsPosition struct {
	Coin          string           `json:"coin"`
	Szi           fixedpoint.Value `json:"szi"`
	EntryPx       fixedpoint.Value `json:"entryPx"`
	PositionValue fixedpoint.Value `json:"positionValue"`
	Leverage      struct {
		Type  string           `json:"type"`
		Value fixedpoint.Value `json:"value"`
	} `json:"leverage"`
	LiquidationPx *fixedpoint.Value `json:"liquidationPx"`
	MarginUsed    fixedpoint.Value  `json:"marginUsed"`
	UnrealizedPnl fixedpoint.Value  `json:"unrealizedPnl"`
}

type WsMarginSummary struct {
	AccountValue    fixedpoint.Value `json:"accountValue"`
	TotalNtlPos     fixedpoint.Value `json:"totalNtlPos"`
	TotalRawUsd     fixedpoint.Value `json:"totalRawUsd"`
	TotalMarginUsed fixedpoint.Value `json:"totalMarginUsed"`
}

// Event types returned by the parser for dispatcher type-switch.
type (
	SubscriptionResponseEvent struct{ Data json.RawMessage }
	WsBookEvent               struct{ Book WsBook }
	WsTradesEvent             struct{ Trades []WsTrade }
	WsCandleEvent             struct{ Candle WsCandle }
	WsUserFillsEvent          struct{ UserFills WsUserFills }
	WsOrderUpdateEvent        struct{ OrderUpdate WsOrderUpdate }
	WsClearinghouseStateEvent struct{ State WsClearinghouseState }
)

const (
	channelSubscriptionResponse = "subscriptionResponse"
	channelL2Book               = "l2Book"
	channelTrades               = "trades"
	channelCandle               = "candle"
	channelUserFills            = "userFills"
	channelOrderUpdates         = "orderUpdates"
	channelClearinghouseState   = "clearinghouseState"
)

func parseWebSocketEvent(message []byte) (any, error) {
	var f wsFrame
	if err := json.Unmarshal(message, &f); err != nil {
		return nil, err
	}

	switch f.Channel {
	case channelSubscriptionResponse:
		return &SubscriptionResponseEvent{Data: f.Data}, nil

	case channelL2Book:
		var book WsBook
		if err := json.Unmarshal(f.Data, &book); err != nil {
			return nil, fmt.Errorf("parse l2Book: %w", err)
		}
		return &WsBookEvent{Book: book}, nil

	case channelTrades:
		var trades []WsTrade
		if err := json.Unmarshal(f.Data, &trades); err != nil {
			return nil, fmt.Errorf("parse trades: %w", err)
		}
		return &WsTradesEvent{Trades: trades}, nil

	case channelCandle:
		var candle WsCandle
		if err := json.Unmarshal(f.Data, &candle); err != nil {
			return nil, fmt.Errorf("parse candle: %w", err)
		}
		return &WsCandleEvent{Candle: candle}, nil

	case channelUserFills:
		var uf WsUserFills
		if err := json.Unmarshal(f.Data, &uf); err != nil {
			return nil, fmt.Errorf("parse userFills: %w", err)
		}
		uf.IsSnapshot = f.IsSnapshot
		return &WsUserFillsEvent{UserFills: uf}, nil

	case channelOrderUpdates:
		var ou WsOrderUpdate
		if err := json.Unmarshal(f.Data, &ou); err != nil {
			return nil, fmt.Errorf("parse orderUpdates: %w", err)
		}
		return &WsOrderUpdateEvent{OrderUpdate: ou}, nil

	case channelClearinghouseState:
		var state WsClearinghouseState
		if err := json.Unmarshal(f.Data, &state); err != nil {
			return nil, fmt.Errorf("parse clearinghouseState: %w", err)
		}
		return &WsClearinghouseStateEvent{State: state}, nil

	default:
		// Unknown channel: return nil, nil so caller can skip or log raw
		return nil, nil
	}
}

// intervalFromCandleInterval maps Hyperliquid interval string to types.Interval.
func intervalFromCandleInterval(s string) types.Interval {
	switch s {
	case "1m":
		return types.Interval1m
	case "3m":
		return types.Interval3m
	case "5m":
		return types.Interval5m
	case "15m":
		return types.Interval15m
	case "30m":
		return types.Interval30m
	case "1h":
		return types.Interval1h
	case "2h":
		return types.Interval2h
	case "4h":
		return types.Interval4h
	case "8h":
		return types.Interval8h
	case "12h":
		return types.Interval12h
	case "1d":
		return types.Interval1d
	case "3d":
		return types.Interval3d
	case "1w":
		return types.Interval1w
	case "1M":
		return types.Interval1mo
	default:
		return types.Interval(s)
	}
}
