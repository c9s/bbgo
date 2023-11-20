package bybit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type WsEvent struct {
	// "op" and "topic" are exclusive.
	*WebSocketOpEvent
	*WebSocketTopicEvent
}

func (w *WsEvent) IsOp() bool {
	return w.WebSocketOpEvent != nil && w.WebSocketTopicEvent == nil
}

func (w *WsEvent) IsTopic() bool {
	return w.WebSocketOpEvent == nil && w.WebSocketTopicEvent != nil
}

type WsOpType string

const (
	WsOpTypePing        WsOpType = "ping"
	WsOpTypePong        WsOpType = "pong"
	WsOpTypeAuth        WsOpType = "auth"
	WsOpTypeSubscribe   WsOpType = "subscribe"
	WsOpTypeUnsubscribe WsOpType = "unsubscribe"
)

type WebsocketOp struct {
	Op   WsOpType `json:"op"`
	Args []string `json:"args"`
}

type WebSocketOpEvent struct {
	Success bool   `json:"success"`
	RetMsg  string `json:"ret_msg"`
	ReqId   string `json:"req_id,omitempty"`

	ConnId string   `json:"conn_id"`
	Op     WsOpType `json:"op"`
	Args   []string `json:"args"`
}

func (w *WebSocketOpEvent) IsValid() error {
	switch w.Op {
	case WsOpTypePing:
		// public event
		if !w.Success || WsOpType(w.RetMsg) != WsOpTypePong {
			return fmt.Errorf("unexpected response result: %+v", w)
		}
		return nil
	case WsOpTypePong:
		// private event, no success and ret_msg fields in response
		return nil
	case WsOpTypeAuth:
		if !w.Success || w.RetMsg != "" {
			return fmt.Errorf("unexpected response result: %+v", w)
		}
		return nil
	case WsOpTypeSubscribe:
		// in the public channel, you can get RetMsg = 'subscribe', but in the private channel, you cannot.
		// so, we only verify that success is true.
		if !w.Success {
			return fmt.Errorf("unexpected response result: %+v", w)
		}
		return nil

	case WsOpTypeUnsubscribe:
		// in the public channel, you can get RetMsg = 'subscribe', but in the private channel, you cannot.
		// so, we only verify that success is true.
		if !w.Success {
			return fmt.Errorf("unexpected response result: %+v", w)
		}
		return nil

	default:
		return fmt.Errorf("unexpected op type: %+v", w)
	}
}

func (w *WebSocketOpEvent) toGlobalPongEventIfValid() (bool, *types.WebsocketPongEvent) {
	if w.Op == WsOpTypePing || w.Op == WsOpTypePong {
		return true, &types.WebsocketPongEvent{}
	}
	return false, nil
}

func (w *WebSocketOpEvent) IsAuthenticated() bool {
	return w.Op == WsOpTypeAuth && w.Success
}

type TopicType string

const (
	TopicTypeOrderBook   TopicType = "orderbook"
	TopicTypeMarketTrade TopicType = "publicTrade"
	TopicTypeWallet      TopicType = "wallet"
	TopicTypeOrder       TopicType = "order"
	TopicTypeKLine       TopicType = "kline"
	TopicTypeTrade       TopicType = "execution"
)

type DataType string

const (
	DataTypeSnapshot DataType = "snapshot"
	DataTypeDelta    DataType = "delta"
)

type WebSocketTopicEvent struct {
	Topic string   `json:"topic"`
	Type  DataType `json:"type"`
	// The timestamp (ms) that the system generates the data
	Ts   types.MillisecondTimestamp `json:"ts"`
	Data json.RawMessage            `json:"data"`
}

type BookEvent struct {
	// Symbol name
	Symbol string `json:"s"`
	// Bids. For snapshot stream, the element is sorted by price in descending order
	Bids types.PriceVolumeSlice `json:"b"`
	// Asks. For snapshot stream, the element is sorted by price in ascending order
	Asks types.PriceVolumeSlice `json:"a"`
	// Update ID. Is a sequence. Occasionally, you'll receive "u"=1, which is a snapshot data due to the restart of
	// the service. So please overwrite your local orderbook
	UpdateId fixedpoint.Value `json:"u"`
	// Cross sequence. You can use this field to compare different levels orderbook data, and for the smaller seq,
	// then it means the data is generated earlier.
	SequenceId fixedpoint.Value `json:"seq"`

	// internal use
	// Copied from WebSocketTopicEvent.Type, WebSocketTopicEvent.Ts
	// Type can be one of snapshot or delta.
	Type DataType
	// ServerTime using the websocket timestamp as server time. Since the event not provide server time information.
	ServerTime time.Time
}

func (e *BookEvent) OrderBook() (snapshot types.SliceOrderBook) {
	snapshot.Symbol = e.Symbol
	snapshot.Bids = e.Bids
	snapshot.Asks = e.Asks
	snapshot.Time = e.ServerTime
	return snapshot
}

type MarketTradeEvent struct {
	// Timestamp is the timestamp (ms) that the order is filled
	Timestamp types.MillisecondTimestamp `json:"T"`
	Symbol    string                     `json:"s"`
	// Side of taker. Buy,Sell
	Side bybitapi.Side `json:"S"`
	// Quantity is the trade size
	Quantity fixedpoint.Value `json:"v"`
	// Price is the trade price
	Price fixedpoint.Value `json:"p"`
	// L is the direction of price change. Unique field for future
	Direction string `json:"L"`
	// trade ID
	TradeId string `json:"i"`
	// Whether it is a block trade order or not
	BlockTrade bool `json:"BT"`
}

func (m *MarketTradeEvent) toGlobalTrade() (types.Trade, error) {
	tradeId, err := strconv.ParseUint(m.TradeId, 10, 64)
	if err != nil {
		return types.Trade{}, fmt.Errorf("unexpected trade id: %s, err: %w", m.TradeId, err)
	}

	side, err := toGlobalSideType(m.Side)
	if err != nil {
		return types.Trade{}, err
	}

	return types.Trade{
		ID:            tradeId,
		OrderID:       0, // not supported
		Exchange:      types.ExchangeBybit,
		Price:         m.Price,
		Quantity:      m.Quantity,
		QuoteQuantity: m.Price.Mul(m.Quantity),
		Symbol:        m.Symbol,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       false, // not supported
		Time:          types.Time(m.Timestamp.Time()),
		Fee:           fixedpoint.Zero, // not supported
		FeeCurrency:   "",              // not supported
	}, nil
}

const topicSeparator = "."

func genTopic(in ...interface{}) string {
	out := make([]string, len(in))
	for k, v := range in {
		out[k] = fmt.Sprintf("%v", v)
	}
	return strings.Join(out, topicSeparator)
}

func getTopicType(topic string) TopicType {
	slice := strings.Split(topic, topicSeparator)
	if len(slice) == 0 {
		return ""
	}
	return TopicType(slice[0])
}

func getSymbolFromTopic(topic string) (string, error) {
	slice := strings.Split(topic, topicSeparator)
	if len(slice) != 3 {
		return "", fmt.Errorf("unexpected topic: %s", topic)
	}
	return slice[2], nil
}

type OrderEvent struct {
	bybitapi.Order

	Category bybitapi.Category `json:"category"`
}

type KLineEvent struct {
	KLines []KLine

	// internal use
	// Type can be one of snapshot or delta. Copied from WebSocketTopicEvent.Type
	Type DataType
	// Symbol. Copied from WebSocketTopicEvent.Topic
	Symbol string
}

type KLine struct {
	// The start timestamp (ms)
	StartTime types.MillisecondTimestamp `json:"start"`
	// The end timestamp (ms)
	EndTime types.MillisecondTimestamp `json:"end"`
	// Kline interval
	Interval   string           `json:"interval"`
	OpenPrice  fixedpoint.Value `json:"open"`
	ClosePrice fixedpoint.Value `json:"close"`
	HighPrice  fixedpoint.Value `json:"high"`
	LowPrice   fixedpoint.Value `json:"low"`
	// Trade volume
	Volume fixedpoint.Value `json:"volume"`
	// Turnover.  Unit of figure: quantity of quota coin
	Turnover fixedpoint.Value `json:"turnover"`
	// Weather the tick is ended or not
	Confirm bool `json:"confirm"`
	// The timestamp (ms) of the last matched order in the candle
	Timestamp types.MillisecondTimestamp `json:"timestamp"`
}

func (k *KLine) toGlobalKLine(symbol string) (types.KLine, error) {
	interval, found := bybitapi.ToGlobalInterval[k.Interval]
	if !found {
		return types.KLine{}, fmt.Errorf("unexpected k line interval: %+v", k)
	}

	return types.KLine{
		Exchange:    types.ExchangeBybit,
		Symbol:      symbol,
		StartTime:   types.Time(k.StartTime.Time()),
		EndTime:     types.Time(k.EndTime.Time()),
		Interval:    interval,
		Open:        k.OpenPrice,
		Close:       k.ClosePrice,
		High:        k.HighPrice,
		Low:         k.LowPrice,
		Volume:      k.Volume,
		QuoteVolume: k.Turnover,
		Closed:      k.Confirm,
	}, nil
}

type TradeEvent struct {
	// linear and inverse order id format: 42f4f364-82e1-49d3-ad1d-cd8cf9aa308d (UUID format)
	// spot: 1468264727470772736 (only numbers)
	// we only use spot trading.
	OrderId     string            `json:"orderId"`
	OrderLinkId string            `json:"orderLinkId"`
	Category    bybitapi.Category `json:"category"`
	Symbol      string            `json:"symbol"`
	ExecId      string            `json:"execId"`
	ExecPrice   fixedpoint.Value  `json:"execPrice"`
	ExecQty     fixedpoint.Value  `json:"execQty"`

	// Is maker order. true: maker, false: taker
	IsMaker bool `json:"isMaker"`
	// Paradigm block trade ID
	BlockTradeId string `json:"blockTradeId"`
	// Order type. Market,Limit
	OrderType bybitapi.OrderType `json:"orderType"`
	// 	Side. Buy,Sell
	Side bybitapi.Side `json:"side"`
	// 	Executed timestamp（ms）
	ExecTime types.MillisecondTimestamp `json:"execTime"`
	// 	Closed position size
	ClosedSize fixedpoint.Value `json:"closedSize"`

	/* The following parameters do not support SPOT trading. */
	// Executed trading fee. You can get spot fee currency instruction here. Normal spot is not supported
	ExecFee fixedpoint.Value `json:"execFee"`
	// Executed type. Normal spot is not supported
	ExecType string `json:"execType"`
	// Executed order value. Normal spot is not supported
	ExecValue fixedpoint.Value `json:"execValue"`
	// Trading fee rate. Normal spot is not supported
	FeeRate fixedpoint.Value `json:"feeRate"`
	// The remaining qty not executed. Normal spot is not supported
	LeavesQty fixedpoint.Value `json:"leavesQty"`
	// Order price. Normal spot is not supported
	OrderPrice fixedpoint.Value `json:"orderPrice"`
	// Order qty. Normal spot is not supported
	OrderQty fixedpoint.Value `json:"orderQty"`
	// Stop order type. If the order is not stop order, any type is not returned. Normal spot is not supported
	StopOrderType string `json:"stopOrderType"`
	// Whether to borrow. Unified spot only. 0: false, 1: true. . Normal spot is not supported, always 0
	IsLeverage string `json:"isLeverage"`
	// Implied volatility of mark price. Valid for option
	MarkIv string `json:"markIv"`
	// The mark price of the symbol when executing. Valid for option
	MarkPrice fixedpoint.Value `json:"markPrice"`
	// The index price of the symbol when executing. Valid for option
	IndexPrice fixedpoint.Value `json:"indexPrice"`
	// The underlying price of the symbol when executing. Valid for option
	UnderlyingPrice fixedpoint.Value `json:"underlyingPrice"`
	// Implied volatility. Valid for option
	TradeIv string `json:"tradeIv"`
}

func (t *TradeEvent) toGlobalTrade(symbolFee symbolFeeDetail) (*types.Trade, error) {
	if t.Category != bybitapi.CategorySpot {
		return nil, fmt.Errorf("unexected category: %s", t.Category)
	}

	side, err := toGlobalSideType(t.Side)
	if err != nil {
		return nil, err
	}

	orderIdNum, err := strconv.ParseUint(t.OrderId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unexpected order id: %s, err: %w", t.OrderId, err)
	}

	execIdNum, err := strconv.ParseUint(t.ExecId, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unexpected exec id: %s, err: %w", t.ExecId, err)
	}

	trade := &types.Trade{
		ID:            execIdNum,
		OrderID:       orderIdNum,
		Exchange:      types.ExchangeBybit,
		Price:         t.ExecPrice,
		Quantity:      t.ExecQty,
		QuoteQuantity: t.ExecPrice.Mul(t.ExecQty),
		Symbol:        t.Symbol,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       t.IsMaker,
		Time:          types.Time(t.ExecTime),
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	}
	trade.FeeCurrency, trade.Fee = calculateFee(*t, symbolFee)
	return trade, nil
}

// CalculateFee given isMaker to get the fee currency and fee.
// https://bybit-exchange.github.io/docs/v5/enum#spot-fee-currency-instruction
//
// with the example of BTCUSDT:
//
// Is makerFeeRate positive?
//
//   - TRUE
//     Side = Buy -> base currency (BTC)
//     Side = Sell -> quote currency (USDT)
//
//   - FALSE
//     IsMakerOrder = TRUE
//     -> Side = Buy -> quote currency (USDT)
//     -> Side = Sell -> base currency (BTC)
//
//     IsMakerOrder = FALSE
//     -> Side = Buy -> base currency (BTC)
//     -> Side = Sell -> quote currency (USDT)
func calculateFee(t TradeEvent, feeDetail symbolFeeDetail) (string, fixedpoint.Value) {
	if feeDetail.MakerFeeRate.Sign() > 0 || !t.IsMaker {
		if t.Side == bybitapi.SideBuy {
			return feeDetail.BaseCoin, baseCoinAsFee(t, feeDetail)
		}
		return feeDetail.QuoteCoin, quoteCoinAsFee(t, feeDetail)
	}

	if t.Side == bybitapi.SideBuy {
		return feeDetail.QuoteCoin, quoteCoinAsFee(t, feeDetail)
	}
	return feeDetail.BaseCoin, baseCoinAsFee(t, feeDetail)
}

func baseCoinAsFee(t TradeEvent, feeDetail symbolFeeDetail) fixedpoint.Value {
	if t.IsMaker {
		return feeDetail.MakerFeeRate.Mul(t.ExecQty)
	}
	return feeDetail.TakerFeeRate.Mul(t.ExecQty)
}

func quoteCoinAsFee(t TradeEvent, feeDetail symbolFeeDetail) fixedpoint.Value {
	baseFee := t.ExecPrice.Mul(t.ExecQty)
	if t.IsMaker {
		return feeDetail.MakerFeeRate.Mul(baseFee)
	}
	return feeDetail.TakerFeeRate.Mul(baseFee)
}
