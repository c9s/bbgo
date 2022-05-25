package max

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"github.com/valyala/fastjson"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type BaseEvent struct {
	Event     string `json:"e"`
	Timestamp int64  `json:"T"`
}

type OrderUpdate struct {
	Event     string    `json:"e"`
	ID        uint64    `json:"i"`
	Side      string    `json:"sd"`
	OrderType OrderType `json:"ot"`

	Price     string `json:"p"`
	StopPrice string `json:"sp"`

	Volume       string     `json:"v"`
	AveragePrice string     `json:"ap"`
	State        OrderState `json:"S"`
	Market       string     `json:"M"`

	RemainingVolume string `json:"rv"`
	ExecutedVolume  string `json:"ev"`

	TradesCount int64 `json:"tc"`

	GroupID     uint32 `json:"gi"`
	ClientOID   string `json:"ci"`
	CreatedAtMs int64  `json:"T"`
	UpdateTime  int64  `json:"TU"`
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
		OrderType:       OrderType(v.GetStringBytes("ot")),
		State:           OrderState(v.GetStringBytes("S")),
		Price:           string(v.GetStringBytes("p")),
		StopPrice:       string(v.GetStringBytes("sp")),
		AveragePrice:    string(v.GetStringBytes("ap")),
		Volume:          string(v.GetStringBytes("v")),
		RemainingVolume: string(v.GetStringBytes("rv")),
		ExecutedVolume:  string(v.GetStringBytes("ev")),
		TradesCount:     v.GetInt64("tc"),
		GroupID:         uint32(v.GetInt("gi")),
		ClientOID:       string(v.GetStringBytes("ci")),
		CreatedAtMs:     v.GetInt64("T"),
		UpdateTime:      v.GetInt64("TU"),
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
	UpdateTime  int64  `json:"TU"`

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
		UpdateTime:  v.GetInt64("TU"),
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
	Currency  string           `json:"cu"`
	Available fixedpoint.Value `json:"av"`
	Locked    fixedpoint.Value `json:"l"`
}

func (m *BalanceMessage) Balance() (*types.Balance, error) {
	return &types.Balance{
		Currency:  strings.ToUpper(m.Currency),
		Locked:    m.Locked,
		Available: m.Available,
	}, nil
}

type AccountUpdateEvent struct {
	BaseEvent
	Balances []BalanceMessage `json:"B"`
}

type AccountSnapshotEvent struct {
	BaseEvent
	Balances []BalanceMessage `json:"B"`
}

func parseAuthEvent(v *fastjson.Value) (*AuthEvent, error) {
	var e AuthEvent
	var err = json.Unmarshal([]byte(v.String()), &e)
	return &e, err
}

type ADRatio struct {
	ADRatio     fixedpoint.Value `json:"ad"`
	AssetInUSDT fixedpoint.Value `json:"as"`
	DebtInUSDT  fixedpoint.Value `json:"db"`
	IndexPrices []struct {
		Market string           `json:"M"`
		Price  fixedpoint.Value `json:"p"`
	} `json:"idxp"`
	TU types.MillisecondTimestamp `json:"TU"`
}

func parseADRatio(v *fastjson.Value) (*ADRatio, error) {
	o, err := v.StringBytes()
	if err != nil {
		return nil, err
	}
	adRatio := ADRatio{}
	err = json.Unmarshal(o, &adRatio)
	return &adRatio, err
}

func ParseUserEvent(v *fastjson.Value) (interface{}, error) {
	eventType := string(v.GetStringBytes("e"))
	switch eventType {
	case "order_snapshot", "mwallet_order_snapshot":
		return parserOrderSnapshotEvent(v), nil

	case "order_update", "mwallet_order_update":
		return parseOrderUpdateEvent(v), nil

	case "trade_snapshot", "mwallet_trade_snapshot":
		return parseTradeSnapshotEvent(v), nil

	case "trade_update", "mwallet_trade_update":
		return parseTradeUpdateEvent(v), nil

	case "ad_ratio_snapshot", "ad_ratio_update":
		return parseADRatio(v)

	case "account_snapshot", "account_update", "mwallet_account_snapshot", "mwallet_account_update":
		var e AccountUpdateEvent
		o := v.String()
		err := json.Unmarshal([]byte(o), &e)
		return &e, err

	case "error":
		logger.Errorf("error %s", v.MarshalTo(nil))
	}

	return nil, errors.Wrapf(ErrMessageTypeNotSupported, "private message %s", v.MarshalTo(nil))
}
