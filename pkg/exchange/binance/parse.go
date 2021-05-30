package binance

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/valyala/fastjson"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

/*

executionReport

{
  "e": "executionReport",        // Event type
  "E": 1499405658658,            // Event time
  "s": "ETHBTC",                 // Symbol
  "c": "mUvoqJxFIILMdfAW5iGSOW", // Client order ID
  "S": "BUY",                    // Side
  "o": "LIMIT",                  // Order type
  "f": "GTC",                    // Time in force
  "q": "1.00000000",             // Order quantity
  "p": "0.10264410",             // Order price
  "P": "0.00000000",             // Stop price
  "F": "0.00000000",             // Iceberg quantity
  "g": -1,                       // OrderListId
  "C": null,                     // Original client order ID; This is the ID of the order being canceled
  "x": "NEW",                    // Current execution type
  "X": "NEW",                    // Current order status
  "r": "NONE",                   // Order reject reason; will be an error code.
  "i": 4293153,                  // Order ID
  "l": "0.00000000",             // Last executed quantity
  "z": "0.00000000",             // Cumulative filled quantity
  "L": "0.00000000",             // Last executed price
  "n": "0",                      // Commission amount
  "N": null,                     // Commission asset
  "T": 1499405658657,            // Transaction time
  "t": -1,                       // Trade ID
  "I": 8641984,                  // Ignore
  "w": true,                     // Is the order on the book?
  "m": false,                    // Is this trade the maker side?
  "M": false,                    // Ignore
  "O": 1499405658657,            // Order creation time
  "Z": "0.00000000",             // Cumulative quote asset transacted quantity
  "Y": "0.00000000",             // Last quote asset transacted quantity (i.e. lastPrice * lastQty)
  "Q": "0.00000000"              // Quote Order Qty
}
*/
type ExecutionReportEvent struct {
	EventBase

	Symbol string `json:"s"`
	Side   string `json:"S"`

	ClientOrderID         string `json:"c"`
	OriginalClientOrderID string `json:"C"`

	OrderType         string `json:"o"`
	OrderCreationTime int64  `json:"O"`

	TimeInForce     string `json:"f"`
	IcebergQuantity string `json:"F"`

	OrderQuantity      string `json:"q"`
	QuoteOrderQuantity string `json:"Q"`

	OrderPrice string `json:"p"`
	StopPrice  string `json:"P"`

	IsOnBook bool `json:"w"`

	IsMaker bool `json:"m"`
	Ignore  bool `json:"M"`

	CommissionAmount string `json:"n"`
	CommissionAsset  string `json:"N"`

	CurrentExecutionType string `json:"x"`
	CurrentOrderStatus   string `json:"X"`

	OrderID int64 `json:"i"`
	Ignored int64 `json:"I"`

	TradeID         int64 `json:"t"`
	TransactionTime int64 `json:"T"`

	LastExecutedQuantity string `json:"l"`
	LastExecutedPrice    string `json:"L"`

	CumulativeFilledQuantity               string `json:"z"`
	CumulativeQuoteAssetTransactedQuantity string `json:"Z"`

	LastQuoteAssetTransactedQuantity string `json:"Y"`
}

func (e *ExecutionReportEvent) Order() (*types.Order, error) {

	switch e.CurrentExecutionType {
	case "NEW", "CANCELED", "REJECTED", "EXPIRED":
	case "REPLACED":
	case "TRADE": // For Order FILLED status. And the order has been completed.
	default:
		return nil, errors.New("execution report type is not for order")
	}

	orderCreationTime := time.Unix(0, e.OrderCreationTime*int64(time.Millisecond))
	return &types.Order{
		Exchange: types.ExchangeBinance,
		SubmitOrder: types.SubmitOrder{
			Symbol:        e.Symbol,
			ClientOrderID: e.ClientOrderID,
			Side:          toGlobalSideType(binance.SideType(e.Side)),
			Type:          toGlobalOrderType(binance.OrderType(e.OrderType)),
			Quantity:      util.MustParseFloat(e.OrderQuantity),
			Price:         util.MustParseFloat(e.OrderPrice),
			TimeInForce:   e.TimeInForce,
		},
		OrderID:          uint64(e.OrderID),
		Status:           toGlobalOrderStatus(binance.OrderStatusType(e.CurrentOrderStatus)),
		ExecutedQuantity: util.MustParseFloat(e.CumulativeFilledQuantity),
		CreationTime:     types.Time(orderCreationTime),
	}, nil
}

func (e *ExecutionReportEvent) Trade() (*types.Trade, error) {
	if e.CurrentExecutionType != "TRADE" {
		return nil, errors.New("execution report is not a trade")
	}

	tt := time.Unix(0, e.TransactionTime*int64(time.Millisecond))
	return &types.Trade{
		ID:            e.TradeID,
		Exchange:      types.ExchangeBinance,
		Symbol:        e.Symbol,
		OrderID:       uint64(e.OrderID),
		Side:          toGlobalSideType(binance.SideType(e.Side)),
		Price:         util.MustParseFloat(e.LastExecutedPrice),
		Quantity:      util.MustParseFloat(e.LastExecutedQuantity),
		QuoteQuantity: util.MustParseFloat(e.LastQuoteAssetTransactedQuantity),
		IsBuyer:       e.Side == "BUY",
		IsMaker:       e.IsMaker,
		Time:          types.Time(tt),
		Fee:           util.MustParseFloat(e.CommissionAmount),
		FeeCurrency:   e.CommissionAsset,
	}, nil
}

/*
balanceUpdate

{
  "e": "balanceUpdate",         //KLineEvent Type
  "E": 1573200697110,           //KLineEvent Time
  "a": "BTC",                   //Asset
  "d": "100.00000000",          //Balance Delta
  "T": 1573200697068            //Clear Time
}
*/
type BalanceUpdateEvent struct {
	EventBase

	Asset     string `json:"a"`
	Delta     string `json:"d"`
	ClearTime int64  `json:"T"`
}

/*

outboundAccountInfo

{
  "e": "outboundAccountInfo",   // KLineEvent type
  "E": 1499405658849,           // KLineEvent time
  "m": 0,                       // Maker commission rate (bips)
  "t": 0,                       // Taker commission rate (bips)
  "b": 0,                       // Buyer commission rate (bips)
  "s": 0,                       // Seller commission rate (bips)
  "T": true,                    // Can trade?
  "W": true,                    // Can withdraw?
  "D": true,                    // Can deposit?
  "u": 1499405658848,           // Time of last account update
  "B": [                        // AccountBalances array
    {
      "a": "LTC",               // Asset
      "f": "17366.18538083",    // Free amount
      "l": "0.00000000"         // Locked amount
    },
    {
      "a": "BTC",
      "f": "10537.85314051",
      "l": "2.19464093"
    },
    {
      "a": "ETH",
      "f": "17902.35190619",
      "l": "0.00000000"
    },
    {
      "a": "BNC",
      "f": "1114503.29769312",
      "l": "0.00000000"
    },
    {
      "a": "NEO",
      "f": "0.00000000",
      "l": "0.00000000"
    }
  ],
  "P": [                       // Account Permissions
        "SPOT"
  ]
}

*/
type Balance struct {
	Asset  string           `json:"a"`
	Free   fixedpoint.Value `json:"f"`
	Locked fixedpoint.Value `json:"l"`
}

type OutboundAccountPositionEvent struct {
	EventBase

	LastAccountUpdateTime int       `json:"u"`
	Balances              []Balance `json:"B,omitempty"`
}

type OutboundAccountInfoEvent struct {
	EventBase

	MakerCommissionRate  int `json:"m"`
	TakerCommissionRate  int `json:"t"`
	BuyerCommissionRate  int `json:"b"`
	SellerCommissionRate int `json:"s"`

	CanTrade    bool `json:"T"`
	CanWithdraw bool `json:"W"`
	CanDeposit  bool `json:"D"`

	LastAccountUpdateTime int `json:"u"`

	Balances    []Balance `json:"B,omitempty"`
	Permissions []string  `json:"P,omitempty"`
}

type ResultEvent struct {
	Result interface{} `json:"result,omitempty"`
	ID     int         `json:"id"`
}

func ParseEvent(message string) (interface{}, error) {
	val, err := fastjson.Parse(message)

	if err != nil {
		return nil, err
	}

	eventType := string(val.GetStringBytes("e"))

	switch eventType {
	case "kline":
		var event KLineEvent
		err := json.Unmarshal([]byte(message), &event)
		return &event, err

	case "outboundAccountPosition":
		var event OutboundAccountPositionEvent
		err := json.Unmarshal([]byte(message), &event)
		return &event, err

	case "outboundAccountInfo":
		var event OutboundAccountInfoEvent
		err := json.Unmarshal([]byte(message), &event)
		return &event, err

	case "balanceUpdate":
		var event BalanceUpdateEvent
		err := json.Unmarshal([]byte(message), &event)
		return &event, err

	case "executionReport":
		var event ExecutionReportEvent
		err := json.Unmarshal([]byte(message), &event)
		return &event, err

	case "depthUpdate":
		return parseDepthEvent(val)

	default:
		id := val.GetInt("id")
		if id > 0 {
			return &ResultEvent{ID: id}, nil
		}
	}

	return nil, fmt.Errorf("unsupported message: %s", message)
}

type DepthEntry struct {
	PriceLevel string
	Quantity   string
}

type DepthEvent struct {
	EventBase

	Symbol        string `json:"s"`
	FirstUpdateID int64  `json:"U"`
	FinalUpdateID int64  `json:"u"`

	Bids []DepthEntry
	Asks []DepthEntry
}

func (e *DepthEvent) String() (o string) {
	o += fmt.Sprintf("Depth %s bid/ask = ", e.Symbol)

	if len(e.Bids) == 0 {
		o += "empty"
	} else {
		o += e.Bids[0].PriceLevel
	}

	o += "/"

	if len(e.Asks) == 0 {
		o += "empty"
	} else {
		o += e.Asks[0].PriceLevel
	}

	o += fmt.Sprintf(" %d ~ %d", e.FirstUpdateID, e.FinalUpdateID)
	return o
}

func (e *DepthEvent) OrderBook() (book types.SliceOrderBook, err error) {
	book.Symbol = e.Symbol

	for _, entry := range e.Bids {
		quantity, err := fixedpoint.NewFromString(entry.Quantity)
		if err != nil {
			log.WithError(err).Errorf("depth quantity parse error: %s", entry.Quantity)
			continue
		}

		price, err := fixedpoint.NewFromString(entry.PriceLevel)
		if err != nil {
			log.WithError(err).Errorf("depth price parse error: %s", entry.PriceLevel)
			continue
		}

		pv := types.PriceVolume{
			Price:  price,
			Volume: quantity,
		}

		book.Bids = book.Bids.Upsert(pv, true)
	}

	for _, entry := range e.Asks {
		quantity, err := fixedpoint.NewFromString(entry.Quantity)
		if err != nil {
			log.WithError(err).Errorf("depth quantity parse error: %s", entry.Quantity)
			continue
		}

		price, err := fixedpoint.NewFromString(entry.PriceLevel)
		if err != nil {
			log.WithError(err).Errorf("depth price parse error: %s", entry.PriceLevel)
			continue
		}

		pv := types.PriceVolume{
			Price:  price,
			Volume: quantity,
		}

		book.Asks = book.Asks.Upsert(pv, false)
	}

	return book, nil
}

func parseDepthEntry(val *fastjson.Value) (*DepthEntry, error) {
	arr, err := val.Array()
	if err != nil {
		return nil, err
	}

	if len(arr) < 2 {
		return nil, errors.New("incorrect depth entry element length")
	}

	return &DepthEntry{
		PriceLevel: string(arr[0].GetStringBytes()),
		Quantity:   string(arr[1].GetStringBytes()),
	}, nil
}

func parseDepthEvent(val *fastjson.Value) (*DepthEvent, error) {
	var err error
	var depth = &DepthEvent{
		EventBase: EventBase{
			Event: string(val.GetStringBytes("e")),
			Time:  val.GetInt64("E"),
		},
		Symbol:        string(val.GetStringBytes("s")),
		FirstUpdateID: val.GetInt64("U"),
		FinalUpdateID: val.GetInt64("u"),
	}

	for _, ev := range val.GetArray("b") {
		entry, err2 := parseDepthEntry(ev)
		if err2 != nil {
			err = err2
			continue
		}

		depth.Bids = append(depth.Bids, *entry)
	}

	for _, ev := range val.GetArray("a") {
		entry, err2 := parseDepthEntry(ev)
		if err2 != nil {
			err = err2
			continue
		}

		depth.Asks = append(depth.Asks, *entry)
	}

	return depth, err
}

type KLine struct {
	StartTime int64 `json:"t"`
	EndTime   int64 `json:"T"`

	Symbol   string `json:"s"`
	Interval string `json:"i"`

	Open  fixedpoint.Value `json:"o"`
	Close fixedpoint.Value `json:"c"`
	High  fixedpoint.Value `json:"h"`
	Low   fixedpoint.Value `json:"l"`

	Volume      fixedpoint.Value `json:"v"` // base asset volume (like 10 BTC)
	QuoteVolume fixedpoint.Value `json:"q"` // quote asset volume

	TakerBuyBaseAssetVolume  fixedpoint.Value `json:"V"` // taker buy base asset volume (like 10 BTC)
	TakerBuyQuoteAssetVolume fixedpoint.Value `json:"Q"` // taker buy quote asset volume (like 1000USDT)

	LastTradeID    int   `json:"L"`
	NumberOfTrades int64 `json:"n"`
	Closed         bool  `json:"x"`
}

type KLineEvent struct {
	EventBase
	Symbol string `json:"s"`
	KLine  KLine  `json:"k,omitempty"`
}

func (k *KLine) KLine() types.KLine {
	return types.KLine{
		Exchange:                 types.ExchangeBinance,
		Symbol:                   k.Symbol,
		Interval:                 types.Interval(k.Interval),
		StartTime:                time.Unix(0, k.StartTime*int64(time.Millisecond)),
		EndTime:                  time.Unix(0, k.EndTime*int64(time.Millisecond)),
		Open:                     k.Open.Float64(),
		Close:                    k.Close.Float64(),
		High:                     k.High.Float64(),
		Low:                      k.Low.Float64(),
		Volume:                   k.Volume.Float64(),
		QuoteVolume:              k.QuoteVolume.Float64(),
		TakerBuyBaseAssetVolume:  k.TakerBuyBaseAssetVolume.Float64(),
		TakerBuyQuoteAssetVolume: k.TakerBuyQuoteAssetVolume.Float64(),
		LastTradeID:              uint64(k.LastTradeID),
		NumberOfTrades:           uint64(k.NumberOfTrades),
		Closed:                   k.Closed,
	}
}

/*

kline

{
  "e": "kline",     // KLineEvent type
  "E": 123456789,   // KLineEvent time
  "s": "BNBBTC",    // Symbol
  "k": {
    "t": 123400000, // Kline start time
    "T": 123460000, // Kline close time
    "s": "BNBBTC",  // Symbol
    "i": "1m",      // Interval
    "f": 100,       // First trade ID
    "L": 200,       // Last trade ID
    "o": "0.0010",  // Open price
    "c": "0.0020",  // Close price
    "h": "0.0025",  // High price
    "l": "0.0015",  // Low price
    "v": "1000",    // Base asset volume
    "n": 100,       // Number of trades
    "x": false,     // Is this kline closed?
    "q": "1.0000",  // Quote asset volume
    "V": "500",     // Taker buy base asset volume
    "Q": "0.500",   // Taker buy quote asset volume
    "B": "123456"   // Ignore
  }
}

*/
type EventBase struct {
	Event string `json:"e"` // event
	Time  int64  `json:"E"`
}
