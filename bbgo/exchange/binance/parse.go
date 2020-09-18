package binance

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/valyala/fastjson"

	"github.com/c9s/bbgo/pkg/bbgo/types"
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
  "Y": "0.00000000",              // Last quote asset transacted quantity (i.e. lastPrice * lastQty)
  "Q": "0.00000000"              // Quote Order Qty
}
*/
type ExecutionReportEvent struct {
	EventBase

	Symbol        string `json:"s"`
	ClientOrderID string `json:"c"`
	Side          string `json:"S"`
	OrderType     string `json:"o"`
	TimeInForce   string `json:"f"`

	OrderQuantity string `json:"q"`
	OrderPrice    string `json:"p"`
	StopPrice     string `json:"P"`

	IsOnBook bool `json:"w"`
	IsMaker  bool `json:"m"`

	CommissionAmount string `json:"n"`
	CommissionAsset  string `json:"N"`

	CurrentExecutionType string `json:"x"`
	CurrentOrderStatus   string `json:"X"`

	OrderID int `json:"i"`

	TradeID         int64 `json:"t"`
	TransactionTime int64 `json:"T"`

	LastExecutedQuantity             string `json:"l"`
	CumulativeFilledQuantity         string `json:"z"`
	LastExecutedPrice                string `json:"L"`
	LastQuoteAssetTransactedQuantity string `json:"Y"`

	OrderCreationTime int `json:"O"`
}

func (e *ExecutionReportEvent) Trade() (*types.Trade, error) {
	if e.CurrentExecutionType != "TRADE" {
		return nil, errors.New("execution report is not a trade")
	}

	tt := time.Unix(0, e.TransactionTime/1000000)
	return &types.Trade{
		ID:            e.TradeID,
		Symbol:        e.Symbol,
		Price:         util.MustParseFloat(e.LastExecutedPrice),
		Quantity:      util.MustParseFloat(e.LastExecutedQuantity),
		QuoteQuantity: util.MustParseFloat(e.LastQuoteAssetTransactedQuantity),
		Side:          e.Side,
		IsBuyer:       e.Side == "BUY",
		IsMaker:       e.IsMaker,
		Time:          tt,
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
  "B": [                        // Balances array
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
	Asset  string `json:"a"`
	Free   string `json:"f"`
	Locked string `json:"l"`
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

	case "outboundAccountInfo", "outboundAccountPosition":
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

	default:
		id := val.GetInt("id")
		if id > 0 {
			return &ResultEvent{ID: id}, nil
		}
	}

	return nil, fmt.Errorf("unsupported message: %s", message)
}

type KLine struct {
	StartTime int64 `json:"t"`
	EndTime   int64 `json:"T"`

	Symbol   string `json:"s"`
	Interval string `json:"i"`

	Open  string `json:"o"`
	Close string `json:"c"`
	High  string `json:"h"`

	Low         string `json:"l"`
	Volume      string `json:"V"` // taker buy base asset volume (like 10 BTC)
	QuoteVolume string `json:"Q"` // taker buy quote asset volume (like 1000USDT)

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
		Symbol:         k.Symbol,
		Interval:       k.Interval,
		StartTime:      time.Unix(0, k.StartTime*int64(time.Millisecond)),
		EndTime:        time.Unix(0, k.EndTime*int64(time.Millisecond)),
		Open:           util.MustParseFloat(k.Open),
		Close:          util.MustParseFloat(k.Close),
		High:           util.MustParseFloat(k.High),
		Low:            util.MustParseFloat(k.Low),
		Volume:         util.MustParseFloat(k.Volume),
		QuoteVolume:    util.MustParseFloat(k.QuoteVolume),
		LastTradeID:    k.LastTradeID,
		NumberOfTrades: k.NumberOfTrades,
		Closed:         k.Closed,
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
