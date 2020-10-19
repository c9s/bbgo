package max

import (
	"github.com/pkg/errors"
	"github.com/valyala/fastjson"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

type BaseEvent struct {
	Event     string `json:"e"`
	Timestamp int64  `json:"T"`
}

type OrderUpdate struct {
	Event     string `json:"e"`
	ID        uint64 `json:"i"`
	Side      string `json:"sd"`
	OrderType string `json:"ot"`

	Price     string `json:"p"`
	StopPrice string `json:"sp"`

	Volume       string `json:"v"`
	AveragePrice string `json:"ap"`
	State        string `json:"S"`
	Market       string `json:"M"`

	RemainingVolume string `json:"rv"`
	ExecutedVolume  string `json:"ev"`

	TradesCount int64 `json:"tc"`

	GroupID     int64  `json:"gi"`
	ClientOID   string `json:"ci"`
	CreatedAtMs int64  `json:"T"`
}

type OrderUpdateEvent struct {
	BaseEvent

	Orders []OrderUpdate `json:"o"`
}

func parserOrderUpdate(v *fastjson.Value) OrderUpdate {
	return OrderUpdate{
		Event:           string(v.GetStringBytes("e")),
		ID:              v.GetUint64("i"),
		Side:            string(v.GetStringBytes("sd")),
		Market:          string(v.GetStringBytes("M")),
		OrderType:       string(v.GetStringBytes("ot")),
		State:           string(v.GetStringBytes("S")),
		Price:           string(v.GetStringBytes("p")),
		StopPrice:       string(v.GetStringBytes("sp")),
		AveragePrice:    string(v.GetStringBytes("ap")),
		Volume:          string(v.GetStringBytes("v")),
		RemainingVolume: string(v.GetStringBytes("rv")),
		ExecutedVolume:  string(v.GetStringBytes("ev")),
		TradesCount:     v.GetInt64("tc"),
		GroupID:         v.GetInt64("gi"),
		ClientOID:       string(v.GetStringBytes("ci")),
		CreatedAtMs:     v.GetInt64("T"),
	}
}

func parseOrderUpdateEvent(v *fastjson.Value) *OrderUpdateEvent {
	var e OrderUpdateEvent
	e.Event = string(v.GetStringBytes("e"))
	e.Timestamp = v.GetInt64("T")

	for _, ov := range v.GetArray("o") {
		o := parserOrderUpdate(ov)
		e.Orders = append(e.Orders, o)
	}

	return &e
}

type OrderSnapshotEvent struct {
	BaseEvent

	Orders []OrderUpdate `json:"o"`
}

func parserOrderSnapshotEvent(v *fastjson.Value) *OrderSnapshotEvent {
	var e OrderSnapshotEvent
	e.Event = string(v.GetStringBytes("e"))
	e.Timestamp = v.GetInt64("T")

	for _, ov := range v.GetArray("o") {
		o := parserOrderUpdate(ov)
		e.Orders = append(e.Orders, o)
	}

	return &e
}

type TradeUpdate struct {
	ID     uint64 `json:"i"`
	Side   string `json:"sd"`
	Price  string `json:"p"`
	Volume string `json:"v"`
	Market string `json:"M"`

	Fee         string `json:"f"`
	FeeCurrency string `json:"fc"`
	Timestamp   int64  `json:"T"`

	OrderID uint64 `json:"oi"`

	Maker bool `json:"m"`
}

func parseTradeUpdate(v *fastjson.Value) TradeUpdate {
	return TradeUpdate{
		ID:          v.GetUint64("i"),
		Side:        string(v.GetStringBytes("sd")),
		Price:       string(v.GetStringBytes("p")),
		Volume:      string(v.GetStringBytes("v")),
		Market:      string(v.GetStringBytes("M")),
		Fee:         string(v.GetStringBytes("f")),
		FeeCurrency: string(v.GetStringBytes("fc")),
		Timestamp:   v.GetInt64("T"),
		OrderID:     v.GetUint64("oi"),
		Maker:       v.GetBool("m"),
	}
}

type TradeUpdateEvent struct {
	BaseEvent

	Trades []TradeUpdate `json:"t"`
}

func parseTradeUpdateEvent(v *fastjson.Value) *TradeUpdateEvent {
	var e TradeUpdateEvent
	e.Event = string(v.GetStringBytes("e"))
	e.Timestamp = v.GetInt64("T")

	for _, tv := range v.GetArray("t") {
		e.Trades = append(e.Trades, parseTradeUpdate(tv))
	}

	return &e
}

type TradeSnapshot []TradeUpdate

type TradeSnapshotEvent struct {
	BaseEvent

	Trades []TradeUpdate `json:"t"`
}

func parseTradeSnapshotEvent(v *fastjson.Value) *TradeSnapshotEvent {
	var e TradeSnapshotEvent
	e.Event = string(v.GetStringBytes("e"))
	e.Timestamp = v.GetInt64("T")

	for _, tv := range v.GetArray("t") {
		e.Trades = append(e.Trades, parseTradeUpdate(tv))
	}

	return &e
}

type BalanceMessage struct {
	Currency  string `json:"cu"`
	Available string `json:"av"`
	Locked    string `json:"l"`
}

func (m *BalanceMessage) Balance() (*types.Balance, error) {
	available, err := util.ParseFloat(m.Available)
	if err != nil {
		return nil, err
	}

	locked, err := util.ParseFloat(m.Locked)
	if err != nil {
		return nil, err
	}

	return &types.Balance{
		Currency:  m.Currency,
		Locked:    locked,
		Available: available,
	}, nil
}

func parseBalance(v *fastjson.Value) BalanceMessage {
	return BalanceMessage{
		Currency:  string(v.GetStringBytes("cu")),
		Available: string(v.GetStringBytes("av")),
		Locked:    string(v.GetStringBytes("l")),
	}
}

type AccountUpdateEvent struct {
	BaseEvent
	Balances []BalanceMessage `json:"B"`
}

func parserAccountUpdateEvent(v *fastjson.Value) *AccountUpdateEvent {
	var e AccountUpdateEvent
	e.Event = string(v.GetStringBytes("e"))
	e.Timestamp = v.GetInt64("T")

	for _, bv := range v.GetArray("B") {
		e.Balances = append(e.Balances, parseBalance(bv))
	}

	return &e
}

type AccountSnapshotEvent struct {
	BaseEvent
	Balances []BalanceMessage `json:"B"`
}

func parserAccountSnapshotEvent(v *fastjson.Value) *AccountSnapshotEvent {
	var e AccountSnapshotEvent
	e.Event = string(v.GetStringBytes("e"))
	e.Timestamp = v.GetInt64("T")

	for _, bv := range v.GetArray("B") {
		e.Balances = append(e.Balances, parseBalance(bv))
	}

	return &e
}

func parseAuthEvent(v *fastjson.Value) *AuthEvent {
	return &AuthEvent{
		Event:     string(v.GetStringBytes("e")),
		ID:        string(v.GetStringBytes("i")),
		Timestamp: v.GetInt64("T"),
	}
}

func ParseUserEvent(v *fastjson.Value) (interface{}, error) {
	eventType := string(v.GetStringBytes("e"))
	switch eventType {
	case "order_snapshot":
		return parserOrderSnapshotEvent(v), nil

	case "order_update":
		return parseOrderUpdateEvent(v), nil

	case "trade_snapshot":
		return parseTradeSnapshotEvent(v), nil

	case "trade_update":
		return parseTradeUpdateEvent(v), nil

	case "account_snapshot":
		return parserAccountSnapshotEvent(v), nil

	case "account_update":
		return parserAccountUpdateEvent(v), nil

	case "authenticated":
		return parseAuthEvent(v), nil

	case "error":
		logger.Errorf("error %s", v.MarshalTo(nil))
	}

	return nil, errors.Wrapf(ErrMessageTypeNotSupported, "private message %s", v.MarshalTo(nil))
}
