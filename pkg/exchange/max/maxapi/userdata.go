package max

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
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

	Price     fixedpoint.Value `json:"p"`
	StopPrice fixedpoint.Value `json:"sp"`

	Volume       fixedpoint.Value `json:"v"`
	AveragePrice fixedpoint.Value `json:"ap"`
	State        OrderState       `json:"S"`
	Market       string           `json:"M"`

	RemainingVolume fixedpoint.Value `json:"rv"`
	ExecutedVolume  fixedpoint.Value `json:"ev"`

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

func parseOrderUpdateEvent(v *fastjson.Value) *OrderUpdateEvent {
	var e OrderUpdateEvent
	e.Event = string(v.GetStringBytes("e"))
	e.Timestamp = v.GetInt64("T")

	for _, ov := range v.GetArray("o") {
		var o = ov.String()
		var u OrderUpdate
		if err := json.Unmarshal([]byte(o), &u); err != nil {
			log.WithError(err).Error("parse error")
			continue
		}

		e.Orders = append(e.Orders, u)
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
		var o = ov.String()
		var u OrderUpdate
		if err := json.Unmarshal([]byte(o), &u); err != nil {
			log.WithError(err).Error("parse error")
			continue
		}

		e.Orders = append(e.Orders, u)
	}

	return &e
}

type TradeUpdate struct {
	ID     uint64           `json:"i"`
	Side   string           `json:"sd"`
	Price  fixedpoint.Value `json:"p"`
	Volume fixedpoint.Value `json:"v"`
	Funds  fixedpoint.Value `json:"fn"`
	Market string           `json:"M"`

	Fee           fixedpoint.Value `json:"f"`
	FeeCurrency   string           `json:"fc"`
	FeeDiscounted bool             `json:"fd"`

	Timestamp  types.MillisecondTimestamp `json:"T"`
	UpdateTime types.MillisecondTimestamp `json:"TU"`

	OrderID uint64 `json:"oi"`

	Maker bool `json:"m"`
}

type TradeUpdateEvent struct {
	BaseEvent
	Trades []TradeUpdate `json:"t"`
}

type TradeSnapshotEvent struct {
	BaseEvent
	Trades []TradeUpdate `json:"t"`
}

func parseTradeUpdateEvent(v *fastjson.Value) (*TradeUpdateEvent, error) {
	jsonBytes := v.String()
	var e TradeUpdateEvent
	err := json.Unmarshal([]byte(jsonBytes), &e)
	return &e, err
}

func parseTradeSnapshotEvent(v *fastjson.Value) (*TradeSnapshotEvent, error) {
	jsonBytes := v.String()
	var e TradeSnapshotEvent
	err := json.Unmarshal([]byte(jsonBytes), &e)
	return &e, err
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

func (r *ADRatio) String() string {
	return fmt.Sprintf("ADRatio: %v Asset: %v USDT, Debt: %v USDT (Mark Prices: %+v)", r.ADRatio, r.AssetInUSDT, r.DebtInUSDT, r.IndexPrices)
}

type ADRatioEvent struct {
	ADRatio ADRatio `json:"ad"`
}

func parseADRatioEvent(v *fastjson.Value) (*ADRatioEvent, error) {
	o := v.String()
	e := ADRatioEvent{}
	err := json.Unmarshal([]byte(o), &e)
	return &e, err
}

type Debt struct {
	Currency      string                     `json:"cu"`
	DebtPrincipal fixedpoint.Value           `json:"dbp"`
	DebtInterest  fixedpoint.Value           `json:"dbi"`
	TU            types.MillisecondTimestamp `json:"TU"`
}

func (d *Debt) String() string {
	return fmt.Sprintf("Debt %s %v (Interest %v)", d.Currency, d.DebtPrincipal, d.DebtInterest)
}

type DebtEvent struct {
	Debts []Debt `json:"db"`
}

func parseDebts(v *fastjson.Value) (*DebtEvent, error) {
	o := v.String()
	e := DebtEvent{}
	err := json.Unmarshal([]byte(o), &e)
	return &e, err
}

func ParseUserEvent(v *fastjson.Value) (interface{}, error) {
	eventType := string(v.GetStringBytes("e"))
	switch eventType {
	case "order_snapshot", "mwallet_order_snapshot":
		return parserOrderSnapshotEvent(v), nil

	case "order_update", "mwallet_order_update":
		return parseOrderUpdateEvent(v), nil

	case "trade_snapshot", "mwallet_trade_snapshot":
		return parseTradeSnapshotEvent(v)

	// trade_fast_update and mwallet_trade_fast_update are deprecated
	case "trade_update", "fast_trade_update", "trade_fast_update",
		"mwallet_trade_update", "mwallet_trade_fast_update", "mwallet_fast_trade_update":
		return parseTradeUpdateEvent(v)

	case "ad_ratio_snapshot", "ad_ratio_update":
		return parseADRatioEvent(v)

	case "borrowing_snapshot", "borrowing_update":
		return parseDebts(v)

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
