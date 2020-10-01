package max

import (
	"github.com/pkg/errors"
	"github.com/valyala/fastjson"
)

func ParsePrivateEvent(message []byte) (interface{}, error) {
	var fp fastjson.Parser
	var v, err = fp.ParseBytes(message)
	if err != nil {
		return nil, errors.Wrap(err, "fail to parse account info raw message")
	}

	eventType := string(v.GetStringBytes("e"))
	switch eventType {
	case "order_snapshot":
		return parserOrderSnapshotEvent(v)

	case "order_update":
		return parseOrderUpdateEvent(v)

	case "trade_snapshot":
		return parseTradeSnapshotEvent(v)

	case "trade_update":
		return parseTradeUpdateEvent(v)

	case "account_snapshot":
		return parserAccountSnapshotEvent(v)

	case "account_update":
		return parserAccountUpdateEvent(v)

	case "authenticated":
		return parserAuthEvent(v)

	case "error":
		logger.Errorf("error %s", message)
	}

	return nil, errors.Wrapf(ErrMessageTypeNotSupported, "private message %s", message)
}

var (
	errParseOrder   = errors.New("failed parse order")
	errParseTrade   = errors.New("failed parse trade")
	errParseAccount = errors.New("failed parse account")
)

type OrderUpdate struct {
	Event        string `json:"e"`
	ID           uint64 `json:"i"`
	Side         string `json:"sd"`
	OrderType    string `json:"ot"`
	Price        string `json:"p"`
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

func parserOrderUpdate(v *fastjson.Value) (OrderUpdate, error) {
	return OrderUpdate{
		Event:           string(v.GetStringBytes("e")),
		ID:              v.GetUint64("i"),
		Side:            string(v.GetStringBytes("sd")),
		Market:          string(v.GetStringBytes("M")),
		OrderType:       string(v.GetStringBytes("ot")),
		State:           string(v.GetStringBytes("S")),
		Price:           string(v.GetStringBytes("p")),
		AveragePrice:    string(v.GetStringBytes("ap")),
		Volume:          string(v.GetStringBytes("v")),
		RemainingVolume: string(v.GetStringBytes("rv")),
		ExecutedVolume:  string(v.GetStringBytes("ev")),
		TradesCount:     v.GetInt64("tc"),
		GroupID:         v.GetInt64("gi"),
		ClientOID:       string(v.GetStringBytes("ci")),
		CreatedAtMs:     v.GetInt64("T"),
	}, nil
}

func parseOrderUpdateEvent(v *fastjson.Value) (OrderUpdate, error) {
	rawOrders := v.GetArray("o")
	if len(rawOrders) == 0 {
		return OrderUpdate{}, errParseOrder
	}

	return parserOrderUpdate(rawOrders[0])
}

type OrderSnapshot []OrderUpdate

func parserOrderSnapshotEvent(v *fastjson.Value) (orderSnapshot OrderSnapshot, err error) {
	var errCount int

	rawOrders := v.GetArray("o")
	for _, ov := range rawOrders {
		o, e := parserOrderUpdate(ov)
		if e != nil {
			errCount++
			err = e
		} else {
			orderSnapshot = append(orderSnapshot, o)
		}
	}

	if errCount > 0 {
		err = errors.Wrapf(err, "failed to parse order snapshot. %d errors in order snapshot. The last error: ", errCount)
	}

	return
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
}

func parseTradeUpdate(v *fastjson.Value) (TradeUpdate, error) {
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
	}, nil
}

func parseTradeUpdateEvent(v *fastjson.Value) (TradeUpdate, error) {
	rawTrades := v.GetArray("t")
	if len(rawTrades) == 0 {
		return TradeUpdate{}, errParseTrade
	}

	return parseTradeUpdate(rawTrades[0])
}

type TradeSnapshot []TradeUpdate

func parseTradeSnapshotEvent(v *fastjson.Value) (tradeSnapshot TradeSnapshot, err error) {
	var errCount int

	rawTrades := v.GetArray("t")
	for _, tv := range rawTrades {
		t, e := parseTradeUpdate(tv)
		if e != nil {
			errCount++
			err = e
		} else {
			tradeSnapshot = append(tradeSnapshot, t)
		}
	}

	if errCount > 0 {
		err = errors.Wrapf(err, "failed to parse trade snapshot. %d errors in trade snapshot. The last error: ", errCount)
	}

	return
}

type Balance struct {
	Currency  string `json:"cu"`
	Available string `json:"av"`
	Locked    string `json:"l"`
}

func parseBalance(v *fastjson.Value) (Balance, error) {
	return Balance{
		Currency:  string(v.GetStringBytes("cu")),
		Available: string(v.GetStringBytes("av")),
		Locked:    string(v.GetStringBytes("l")),
	}, nil
}

func parserAccountUpdateEvent(v *fastjson.Value) (Balance, error) {
	rawBalances := v.GetArray("B")
	if len(rawBalances) == 0 {
		return Balance{}, errParseAccount
	}

	return parseBalance(rawBalances[0])
}

type BalanceSnapshot []Balance

func parserAccountSnapshotEvent(v *fastjson.Value) (balanceSnapshot BalanceSnapshot, err error) {
	var errCount int

	rawBalances := v.GetArray("B")
	for _, bv := range rawBalances {
		b, e := parseBalance(bv)
		if e != nil {
			errCount++
			err = e
		} else {
			balanceSnapshot = append(balanceSnapshot, b)
		}
	}

	if errCount > 0 {
		err = errors.Wrapf(err, "failed to parse balance snapshot. %d errors in balance snapshot. The last error: ", errCount)
	}

	return
}

func parserAuthEvent(v *fastjson.Value) (AuthEvent, error) {
	return AuthEvent{
		Event:     string(v.GetStringBytes("e")),
		ID:        string(v.GetStringBytes("i")),
		Timestamp: v.GetInt64("T"),
	}, nil
}
