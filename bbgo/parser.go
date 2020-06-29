package bbgo

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/valyala/fastjson"
)

/*

executionReport

{
  "e": "executionReport",        // KLineEvent type
  "E": 1499405658658,            // KLineEvent time
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
type BinanceExecutionReportEvent struct {
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

	LastExecutedQuantity     string `json:"l"`
	CumulativeFilledQuantity string `json:"z"`
	LastExecutedPrice        string `json:"L"`

	OrderCreationTime int `json:"O"`
}

func (e *BinanceExecutionReportEvent) Trade() (*Trade, error) {
	if e.CurrentExecutionType != "TRADE" {
		return nil, errors.New("execution report is not a trade")
	}

	tt := time.Unix(0, e.TransactionTime/1000000)
	return &Trade{
		ID:          e.TradeID,
		Market:      e.Symbol,
		Price:       MustParseFloat(e.LastExecutedPrice),
		Volume:      MustParseFloat(e.LastExecutedQuantity),
		IsBuyer:     e.Side == "BUY",
		IsMaker:     e.IsMaker,
		Time:        tt,
		Fee:         MustParseFloat(e.CommissionAmount),
		FeeCurrency: e.CommissionAsset,
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

func parseEvent(message string) (interface{}, error) {
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
		var event BinanceExecutionReportEvent
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
