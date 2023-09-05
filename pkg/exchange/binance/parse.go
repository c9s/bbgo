package binance

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/adshao/go-binance/v2/futures"
	"github.com/slack-go/slack"

	"github.com/adshao/go-binance/v2"
	"github.com/valyala/fastjson"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type EventBase struct {
	Event string `json:"e"` // event name
	Time  int64  `json:"E"` // event time
}

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
	  "Q": "0.00000000"              // Quote Order Quantity
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

	TimeInForce     string           `json:"f"`
	IcebergQuantity fixedpoint.Value `json:"F"`

	OrderQuantity      fixedpoint.Value `json:"q"`
	QuoteOrderQuantity fixedpoint.Value `json:"Q"`

	OrderPrice fixedpoint.Value `json:"p"`
	StopPrice  fixedpoint.Value `json:"P"`

	IsOnBook     bool                       `json:"w"`
	WorkingTime  types.MillisecondTimestamp `json:"W"`
	TrailingTime types.MillisecondTimestamp `json:"D"`

	IsMaker bool `json:"m"`
	Ignore  bool `json:"M"`

	CommissionAmount fixedpoint.Value `json:"n"`
	CommissionAsset  string           `json:"N"`

	CurrentExecutionType string `json:"x"`
	CurrentOrderStatus   string `json:"X"`

	OrderID int64 `json:"i"`
	Ignored int64 `json:"I"`

	TradeID         int64 `json:"t"`
	TransactionTime int64 `json:"T"`

	LastExecutedQuantity fixedpoint.Value `json:"l"`
	LastExecutedPrice    fixedpoint.Value `json:"L"`

	CumulativeFilledQuantity               fixedpoint.Value `json:"z"`
	CumulativeQuoteAssetTransactedQuantity fixedpoint.Value `json:"Z"`

	LastQuoteAssetTransactedQuantity fixedpoint.Value `json:"Y"`
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
		SubmitOrder: types.SubmitOrder{
			ClientOrderID: e.ClientOrderID,
			Symbol:        e.Symbol,
			Side:          toGlobalSideType(binance.SideType(e.Side)),
			Type:          toGlobalOrderType(binance.OrderType(e.OrderType)),
			Quantity:      e.OrderQuantity,
			Price:         e.OrderPrice,
			StopPrice:     e.StopPrice,
			TimeInForce:   types.TimeInForce(e.TimeInForce),
			ReduceOnly:    false,
			ClosePosition: false,
		},
		Exchange:         types.ExchangeBinance,
		IsWorking:        e.IsOnBook,
		OrderID:          uint64(e.OrderID),
		Status:           toGlobalOrderStatus(binance.OrderStatusType(e.CurrentOrderStatus)),
		ExecutedQuantity: e.CumulativeFilledQuantity,
		CreationTime:     types.Time(orderCreationTime),
		UpdateTime:       types.Time(orderCreationTime),
	}, nil
}

func (e *ExecutionReportEvent) Trade() (*types.Trade, error) {
	if e.CurrentExecutionType != "TRADE" {
		return nil, errors.New("execution report is not a trade")
	}

	tt := time.Unix(0, e.TransactionTime*int64(time.Millisecond))
	return &types.Trade{
		ID:            uint64(e.TradeID),
		Exchange:      types.ExchangeBinance,
		Symbol:        e.Symbol,
		OrderID:       uint64(e.OrderID),
		Side:          toGlobalSideType(binance.SideType(e.Side)),
		Price:         e.LastExecutedPrice,
		Quantity:      e.LastExecutedQuantity,
		QuoteQuantity: e.LastQuoteAssetTransactedQuantity,
		IsBuyer:       e.Side == "BUY",
		IsMaker:       e.IsMaker,
		Time:          types.Time(tt),
		Fee:           e.CommissionAmount,
		FeeCurrency:   e.CommissionAsset,
	}, nil
}

/*
event: balanceUpdate

Balance Update occurs during the following:

Deposits or withdrawals from the account
Transfer of funds between accounts (e.g. Spot to Margin)

	{
	  "e": "balanceUpdate",         //KLineEvent Type
	  "E": 1573200697110,           //KLineEvent Time
	  "a": "BTC",                   //Asset
	  "d": "100.00000000",          //Balance Delta
	  "T": 1573200697068            //Clear Time
	}

This event is only for Spot
*/
type BalanceUpdateEvent struct {
	EventBase

	Asset     string                     `json:"a"`
	Delta     fixedpoint.Value           `json:"d"`
	ClearTime types.MillisecondTimestamp `json:"T"`
}

func (e *BalanceUpdateEvent) SlackAttachment() slack.Attachment {
	return slack.Attachment{
		Title: "Binance Balance Update Event",
		Color: "warning",
		Fields: []slack.AttachmentField{
			{
				Title: "Asset",
				Value: e.Asset,
				Short: true,
			},
			{
				Title: "Delta",
				Value: e.Delta.String(),
				Short: true,
			},
			{
				Title: "Time",
				Value: e.ClearTime.String(),
				Short: true,
			},
		},
	}
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

func parseWebSocketEvent(message []byte) (interface{}, error) {
	val, err := fastjson.ParseBytes(message)

	if err != nil {
		return nil, err
	}

	// res, err := json.MarshalIndent(message, "", "  ")
	// if err != nil {
	//	log.Fatal(err)
	// }
	// str := strings.ReplaceAll(string(res), "\\", "")
	// fmt.Println(str)
	eventType := string(val.GetStringBytes("e"))
	if eventType == "" && IsBookTicker(val) {
		eventType = "bookTicker"
	}

	switch eventType {
	case "kline":
		var event KLineEvent
		err := json.Unmarshal([]byte(message), &event)
		return &event, err
	case "bookTicker":
		var event BookTickerEvent
		err := json.Unmarshal([]byte(message), &event)
		event.Event = eventType
		return &event, err

	case "outboundAccountPosition":
		var event OutboundAccountPositionEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	case "outboundAccountInfo":
		var event OutboundAccountInfoEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	case "balanceUpdate":
		var event BalanceUpdateEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	case "executionReport":
		var event ExecutionReportEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	case "depthUpdate":
		return parseDepthEvent(val)

	case "listenKeyExpired":
		var event ListenKeyExpired
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	case "trade":
		var event MarketTradeEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	case "aggTrade":
		var event AggTradeEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	}

	// futures stream
	switch eventType {

	// futures market data stream
	// ========================================================
	case "continuousKline":
		var event ContinuousKLineEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	case "markPriceUpdate":
		var event MarkPriceUpdateEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	// futures user data stream
	// ========================================================
	case "ORDER_TRADE_UPDATE":
		var event OrderTradeUpdateEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	// Event: Balance and Position Update
	case "ACCOUNT_UPDATE":
		var event AccountUpdateEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	// Event: Order Update
	case "ACCOUNT_CONFIG_UPDATE":
		var event AccountConfigUpdateEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	case "MARGIN_CALL":
		var event MarginCallEvent
		err = json.Unmarshal([]byte(message), &event)
		return &event, err

	default:
		id := val.GetInt("id")
		if id > 0 {
			return &ResultEvent{ID: id}, nil
		}
	}

	return nil, fmt.Errorf("unsupported message: %s", message)
}

// IsBookTicker document ref :https://binance-docs.github.io/apidocs/spot/en/#individual-symbol-book-ticker-streams
// use key recognition because there's no identify in the content.
func IsBookTicker(val *fastjson.Value) bool {
	return !val.Exists("e") && val.Exists("u") &&
		val.Exists("s") && val.Exists("b") &&
		val.Exists("B") && val.Exists("a") && val.Exists("A")
}

type DepthEntry struct {
	PriceLevel fixedpoint.Value
	Quantity   fixedpoint.Value
}

type DepthEvent struct {
	EventBase

	Symbol        string `json:"s"`
	FirstUpdateID int64  `json:"U"`
	FinalUpdateID int64  `json:"u"`

	Bids types.PriceVolumeSlice `json:"b"`
	Asks types.PriceVolumeSlice `json:"a"`
}

func (e *DepthEvent) String() (o string) {
	o += fmt.Sprintf("Depth %s bid/ask = ", e.Symbol)

	if len(e.Bids) == 0 {
		o += "empty"
	} else {
		o += e.Bids[0].Price.String()
	}

	o += "/"

	if len(e.Asks) == 0 {
		o += "empty"
	} else {
		o += e.Asks[0].Price.String()
	}

	o += fmt.Sprintf(" %d ~ %d", e.FirstUpdateID, e.FinalUpdateID)
	return o
}

func (e *DepthEvent) OrderBook() (book types.SliceOrderBook, err error) {
	book.Symbol = e.Symbol
	book.Time = types.NewMillisecondTimestampFromInt(e.EventBase.Time).Time()

	// already in descending order
	book.Bids = e.Bids
	book.Asks = e.Asks
	return book, err
}

func parseDepthEntry(val *fastjson.Value) (*types.PriceVolume, error) {
	arr, err := val.Array()
	if err != nil {
		return nil, err
	}

	if len(arr) < 2 {
		return nil, errors.New("incorrect depth entry element length")
	}

	price, err := fixedpoint.NewFromString(string(arr[0].GetStringBytes()))
	if err != nil {
		return nil, err
	}

	quantity, err := fixedpoint.NewFromString(string(arr[1].GetStringBytes()))
	if err != nil {
		return nil, err
	}

	return &types.PriceVolume{
		Price:  price,
		Volume: quantity,
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

type MarketTradeEvent struct {
	EventBase
	Symbol   string           `json:"s"`
	Quantity fixedpoint.Value `json:"q"`
	Price    fixedpoint.Value `json:"p"`

	BuyerOrderId  int64 `json:"b"`
	SellerOrderId int64 `json:"a"`

	OrderTradeTime int64 `json:"T"`
	TradeId        int64 `json:"t"`

	IsMaker bool `json:"m"`
	Dummy   bool `json:"M"`
}

/*

market trade

{
  "e": "trade",     // Event type
  "E": 123456789,   // Event time
  "s": "BNBBTC",    // Symbol
  "t": 12345,       // Trade ID
  "p": "0.001",     // Price
  "q": "100",       // Quantity
  "b": 88,          // Buyer order ID
  "a": 50,          // Seller order ID
  "T": 123456785,   // Trade time
  "m": true,        // Is the buyer the market maker?
  "M": true         // Ignore
}

*/

func (e *MarketTradeEvent) Trade() types.Trade {
	tt := time.Unix(0, e.OrderTradeTime*int64(time.Millisecond))
	var orderId int64
	var side types.SideType
	var isBuyer bool
	if e.IsMaker {
		orderId = e.SellerOrderId // seller is taker
		side = types.SideTypeSell
		isBuyer = false
	} else {
		orderId = e.BuyerOrderId // buyer is taker
		side = types.SideTypeBuy
		isBuyer = true
	}
	return types.Trade{
		ID:            uint64(e.TradeId),
		Exchange:      types.ExchangeBinance,
		Symbol:        e.Symbol,
		OrderID:       uint64(orderId),
		Side:          side,
		Price:         e.Price,
		Quantity:      e.Quantity,
		QuoteQuantity: e.Quantity.Mul(e.Price),
		IsBuyer:       isBuyer,
		IsMaker:       e.IsMaker,
		Time:          types.Time(tt),
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	}
}

type AggTradeEvent struct {
	EventBase
	Symbol         string           `json:"s"`
	Quantity       fixedpoint.Value `json:"q"`
	Price          fixedpoint.Value `json:"p"`
	FirstTradeId   int64            `json:"f"`
	LastTradeId    int64            `json:"l"`
	OrderTradeTime int64            `json:"T"`
	IsMaker        bool             `json:"m"`
	Dummy          bool             `json:"M"`
}

/*
aggregate trade
{
  "e": "aggTrade",  // Event type
  "E": 123456789,   // Event time
  "s": "BNBBTC",    // Symbol
  "a": 12345,       // Aggregate trade ID
  "p": "0.001",     // Price
  "q": "100",       // Quantity
  "f": 100,         // First trade ID
  "l": 105,         // Last trade ID
  "T": 123456785,   // Trade time
  "m": true,        // Is the buyer the market maker?
  "M": true         // Ignore
}
*/

func (e *AggTradeEvent) Trade() types.Trade {
	tt := time.Unix(0, e.OrderTradeTime*int64(time.Millisecond))
	var side types.SideType
	var isBuyer bool
	if e.IsMaker {
		side = types.SideTypeSell
		isBuyer = false
	} else {
		side = types.SideTypeBuy
		isBuyer = true
	}
	return types.Trade{
		ID:            uint64(e.LastTradeId),
		Exchange:      types.ExchangeBinance,
		Symbol:        e.Symbol,
		OrderID:       0,
		Side:          side,
		Price:         e.Price,
		Quantity:      e.Quantity,
		QuoteQuantity: e.Quantity.Mul(e.Price),
		IsBuyer:       isBuyer,
		IsMaker:       e.IsMaker,
		Time:          types.Time(tt),
		Fee:           fixedpoint.Zero,
		FeeCurrency:   "",
	}
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
		StartTime:                types.NewTimeFromUnix(0, k.StartTime*int64(time.Millisecond)),
		EndTime:                  types.NewTimeFromUnix(0, k.EndTime*int64(time.Millisecond)),
		Open:                     k.Open,
		Close:                    k.Close,
		High:                     k.High,
		Low:                      k.Low,
		Volume:                   k.Volume,
		QuoteVolume:              k.QuoteVolume,
		TakerBuyBaseAssetVolume:  k.TakerBuyBaseAssetVolume,
		TakerBuyQuoteAssetVolume: k.TakerBuyQuoteAssetVolume,
		LastTradeID:              uint64(k.LastTradeID),
		NumberOfTrades:           uint64(k.NumberOfTrades),
		Closed:                   k.Closed,
	}
}

type ListenKeyExpired struct {
	EventBase
}

type MarkPriceUpdateEvent struct {
	EventBase

	Symbol string `json:"s"`

	MarkPrice      fixedpoint.Value `json:"p"`
	IndexPrice     fixedpoint.Value `json:"i"`
	EstimatedPrice fixedpoint.Value `json:"P"`

	FundingRate     fixedpoint.Value `json:"r"`
	NextFundingTime int64            `json:"T"`
}

/*
{
  "e": "markPriceUpdate",     // Event type
  "E": 1562305380000,         // Event time
  "s": "BTCUSDT",             // Symbol
  "p": "11794.15000000",      // Mark price
  "i": "11784.62659091",      // Index price
  "P": "11784.25641265",      // Estimated Settle Price, only useful in the last hour before the settlement starts
  "r": "0.00038167",          // Funding rate
  "T": 1562306400000          // Next funding time
}
*/

type ContinuousKLineEvent struct {
	EventBase
	Symbol string `json:"ps"`
	ct     string `json:"ct"`
	KLine  KLine  `json:"k,omitempty"`
}

/*
{
  "e":"continuous_kline",   // Event type
  "E":1607443058651,        // Event time
  "ps":"BTCUSDT",           // Pair
  "ct":"PERPETUAL"          // Contract type
  "k":{
    "t":1607443020000,      // Kline start time
    "T":1607443079999,      // Kline close time
    "i":"1m",               // Interval
    "f":116467658886,       // First trade ID
    "L":116468012423,       // Last trade ID
    "o":"18787.00",         // Open price
    "c":"18804.04",         // Close price
    "h":"18804.04",         // High price
    "l":"18786.54",         // Low price
    "v":"197.664",          // volume
    "n": 543,               // Number of trades
    "x":false,              // Is this kline closed?
    "q":"3715253.19494",    // Quote asset volume
    "V":"184.769",          // Taker buy volume
    "Q":"3472925.84746",    //Taker buy quote asset volume
    "B":"0"                 // Ignore
  }
}
*/

// Similar to the ExecutionReportEvent's fields. But with totally different json key.
// e.g., Stop price. So that, we can not merge them.
type OrderTrade struct {
	Symbol           string           `json:"s"`
	ClientOrderID    string           `json:"c"`
	Side             string           `json:"S"`
	OrderType        string           `json:"o"`
	TimeInForce      string           `json:"f"`
	OriginalQuantity fixedpoint.Value `json:"q"`
	OriginalPrice    fixedpoint.Value `json:"p"`

	AveragePrice         fixedpoint.Value `json:"ap"`
	StopPrice            fixedpoint.Value `json:"sp"`
	CurrentExecutionType string           `json:"x"`
	CurrentOrderStatus   string           `json:"X"`

	OrderId                        int64            `json:"i"`
	OrderLastFilledQuantity        fixedpoint.Value `json:"l"`
	OrderFilledAccumulatedQuantity fixedpoint.Value `json:"z"`
	LastFilledPrice                fixedpoint.Value `json:"L"`

	CommissionAmount fixedpoint.Value `json:"n"`
	CommissionAsset  string           `json:"N"`

	OrderTradeTime types.MillisecondTimestamp `json:"T"`
	TradeId        int64                      `json:"t"`

	BidsNotional string `json:"b"`
	AskNotional  string `json:"a"`

	IsMaker      bool `json:"m"`
	IsReduceOnly bool ` json:"r"`

	StopPriceWorkingType string `json:"wt"`
	OriginalOrderType    string `json:"ot"`
	PositionSide         string `json:"ps"`
	RealizedProfit       string `json:"rp"`
}

type OrderTradeUpdateEvent struct {
	EventBase
	Transaction int64      `json:"T"`
	OrderTrade  OrderTrade `json:"o"`
}

// {

// 	"e":"ORDER_TRADE_UPDATE",     // Event Type
// 	"E":1568879465651,            // Event Time
// 	"T":1568879465650,            // Transaction Time
// 	"o":{
// 	  "s":"BTCUSDT",              // Symbol
// 	  "c":"TEST",                 // Client Order Id
// 		// special client order id:
// 		// starts with "autoclose-": liquidation order
// 		// "adl_autoclose": ADL auto close order
// 	  "S":"SELL",                 // Side
// 	  "o":"TRAILING_STOP_MARKET", // Order Type
// 	  "f":"GTC",                  // Time in Force
// 	  "q":"0.001",                // Original Quantity
// 	  "p":"0",                    // Original Price
// 	  "ap":"0",                   // Average Price
// 	  "sp":"7103.04",             // Stop Price. Please ignore with TRAILING_STOP_MARKET order
// 	  "x":"NEW",                  // Execution Type
// 	  "X":"NEW",                  // Order Status
// 	  "i":8886774,                // Order Id
// 	  "l":"0",                    // Order Last Filled Quantity
// 	  "z":"0",                    // Order Filled Accumulated Quantity
// 	  "L":"0",                    // Last Filled Price
// 	  "N":"USDT",             // Commission Asset, will not push if no commission
// 	  "n":"0",                // Commission, will not push if no commission
// 	  "T":1568879465651,          // Order Trade Time
// 	  "t":0,                      // Trade Id
// 	  "b":"0",                    // Bids Notional
// 	  "a":"9.91",                 // Ask Notional
// 	  "m":false,                  // Is this trade the maker side?
// 	  "R":false,                  // Is this reduce only
// 	  "wt":"CONTRACT_PRICE",      // Stop Price Working Type
// 	  "ot":"TRAILING_STOP_MARKET",    // Original Order Type
// 	  "ps":"LONG",                        // Position Side
// 	  "cp":false,                     // If Close-All, pushed with conditional order
// 	  "AP":"7476.89",             // Activation Price, only puhed with TRAILING_STOP_MARKET order
// 	  "cr":"5.0",                 // Callback Rate, only puhed with TRAILING_STOP_MARKET order
// 	  "rp":"0"                            // Realized Profit of the trade
// 	}

//   }

func (e *OrderTradeUpdateEvent) OrderFutures() (*types.Order, error) {

	switch e.OrderTrade.CurrentExecutionType {
	case "NEW", "CANCELED", "EXPIRED":
	case "CALCULATED - Liquidation Execution":
	case "TRADE": // For Order FILLED status. And the order has been completed.
	default:
		return nil, errors.New("execution report type is not for futures order")
	}

	return &types.Order{
		Exchange: types.ExchangeBinance,
		SubmitOrder: types.SubmitOrder{
			Symbol:        e.OrderTrade.Symbol,
			ClientOrderID: e.OrderTrade.ClientOrderID,
			Side:          toGlobalFuturesSideType(futures.SideType(e.OrderTrade.Side)),
			Type:          toGlobalFuturesOrderType(futures.OrderType(e.OrderTrade.OrderType)),
			Quantity:      e.OrderTrade.OriginalQuantity,
			Price:         e.OrderTrade.OriginalPrice,
			TimeInForce:   types.TimeInForce(e.OrderTrade.TimeInForce),
		},
		OrderID:          uint64(e.OrderTrade.OrderId),
		Status:           toGlobalFuturesOrderStatus(futures.OrderStatusType(e.OrderTrade.CurrentOrderStatus)),
		ExecutedQuantity: e.OrderTrade.OrderFilledAccumulatedQuantity,
		CreationTime:     types.Time(e.OrderTrade.OrderTradeTime.Time()), // FIXME: find the correct field for creation time
		UpdateTime:       types.Time(e.OrderTrade.OrderTradeTime.Time()),
	}, nil
}

func (e *OrderTradeUpdateEvent) TradeFutures() (*types.Trade, error) {
	if e.OrderTrade.CurrentExecutionType != "TRADE" {
		return nil, errors.New("execution report is not a futures trade")
	}

	return &types.Trade{
		ID:            uint64(e.OrderTrade.TradeId),
		Exchange:      types.ExchangeBinance,
		Symbol:        e.OrderTrade.Symbol,
		OrderID:       uint64(e.OrderTrade.OrderId),
		Side:          toGlobalSideType(binance.SideType(e.OrderTrade.Side)),
		Price:         e.OrderTrade.LastFilledPrice,
		Quantity:      e.OrderTrade.OrderLastFilledQuantity,
		QuoteQuantity: e.OrderTrade.LastFilledPrice.Mul(e.OrderTrade.OrderLastFilledQuantity),
		IsBuyer:       e.OrderTrade.Side == "BUY",
		IsMaker:       e.OrderTrade.IsMaker,
		Time:          types.Time(e.OrderTrade.OrderTradeTime.Time()),
		Fee:           e.OrderTrade.CommissionAmount,
		FeeCurrency:   e.OrderTrade.CommissionAsset,
	}, nil
}

type FuturesStreamBalance struct {
	Asset              string           `json:"a"`
	WalletBalance      fixedpoint.Value `json:"wb"`
	CrossWalletBalance fixedpoint.Value `json:"cw"`
	BalanceChange      fixedpoint.Value `json:"bc"`
}

type FuturesStreamPosition struct {
	Symbol                 string           `json:"s"`
	PositionAmount         fixedpoint.Value `json:"pa"`
	EntryPrice             fixedpoint.Value `json:"ep"`
	AccumulatedRealizedPnL fixedpoint.Value `json:"cr"` // (Pre-fee) Accumulated Realized PnL
	UnrealizedPnL          fixedpoint.Value `json:"up"`
	MarginType             string           `json:"mt"`
	IsolatedWallet         fixedpoint.Value `json:"iw"`
	PositionSide           string           `json:"ps"`
}

type AccountUpdateEventReasonType string

const (
	AccountUpdateEventReasonDeposit          AccountUpdateEventReasonType = "DEPOSIT"
	AccountUpdateEventReasonWithdraw         AccountUpdateEventReasonType = "WITHDRAW"
	AccountUpdateEventReasonOrder            AccountUpdateEventReasonType = "ORDER"
	AccountUpdateEventReasonFundingFee       AccountUpdateEventReasonType = "FUNDING_FEE"
	AccountUpdateEventReasonMarginTransfer   AccountUpdateEventReasonType = "MARGIN_TRANSFER"
	AccountUpdateEventReasonMarginTypeChange AccountUpdateEventReasonType = "MARGIN_TYPE_CHANGE"
	AccountUpdateEventReasonAssetTransfer    AccountUpdateEventReasonType = "ASSET_TRANSFER"
	AccountUpdateEventReasonAdminDeposit     AccountUpdateEventReasonType = "ADMIN_DEPOSIT"
	AccountUpdateEventReasonAdminWithdraw    AccountUpdateEventReasonType = "ADMIN_WITHDRAW"
)

type AccountUpdate struct {
	// m: DEPOSIT WITHDRAW
	// ORDER FUNDING_FEE
	// WITHDRAW_REJECT ADJUSTMENT
	// INSURANCE_CLEAR
	// ADMIN_DEPOSIT ADMIN_WITHDRAW
	// MARGIN_TRANSFER MARGIN_TYPE_CHANGE
	// ASSET_TRANSFER
	// OPTIONS_PREMIUM_FEE OPTIONS_SETTLE_PROFIT
	// AUTO_EXCHANGE
	// COIN_SWAP_DEPOSIT COIN_SWAP_WITHDRAW
	EventReasonType AccountUpdateEventReasonType `json:"m"`
	Balances        []FuturesStreamBalance       `json:"B,omitempty"`
	Positions       []FuturesStreamPosition      `json:"P,omitempty"`
}

type MarginCallEvent struct {
	EventBase

	CrossWalletBalance fixedpoint.Value `json:"cw"`
	P                  []struct {
		Symbol                    string           `json:"s"`
		PositionSide              string           `json:"ps"`
		PositionAmount            fixedpoint.Value `json:"pa"`
		MarginType                string           `json:"mt"`
		IsolatedWallet            fixedpoint.Value `json:"iw"`
		MarkPrice                 fixedpoint.Value `json:"mp"`
		UnrealizedPnL             fixedpoint.Value `json:"up"`
		MaintenanceMarginRequired fixedpoint.Value `json:"mm"`
	} `json:"p"` // Position(s) of Margin Call
}

// AccountUpdateEvent is only used in the futures user data stream
type AccountUpdateEvent struct {
	EventBase
	Transaction   int64         `json:"T"`
	AccountUpdate AccountUpdate `json:"a"`
}

type AccountConfigUpdateEvent struct {
	EventBase
	Transaction int64 `json:"T"`

	// When the leverage of a trade pair changes,
	// the payload will contain the object ac to represent the account configuration of the trade pair,
	// where s represents the specific trade pair and l represents the leverage
	AccountConfig struct {
		Symbol   string           `json:"s"`
		Leverage fixedpoint.Value `json:"l"`
	} `json:"ac"`

	// When the user Multi-Assets margin mode changes the payload will contain the object ai representing the user account configuration,
	// where j represents the user Multi-Assets margin mode
	MarginModeConfig struct {
		MultiAssetsMode bool `json:"j"`
	} `json:"ai"`
}

type BookTickerEvent struct {
	EventBase
	Symbol   string           `json:"s"`
	Buy      fixedpoint.Value `json:"b"`
	BuySize  fixedpoint.Value `json:"B"`
	Sell     fixedpoint.Value `json:"a"`
	SellSize fixedpoint.Value `json:"A"`
	// "u":400900217,     // order book updateId
	// "s":"BNBUSDT",     // symbol
	// "b":"25.35190000", // best bid price
	// "B":"31.21000000", // best bid qty
	// "a":"25.36520000", // best ask price
	// "A":"40.66000000"  // best ask qty
}

func (k *BookTickerEvent) BookTicker() types.BookTicker {
	return types.BookTicker{
		Symbol:   k.Symbol,
		Buy:      k.Buy,
		BuySize:  k.BuySize,
		Sell:     k.Sell,
		SellSize: k.SellSize,
	}
}
