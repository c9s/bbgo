package okex

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/valyala/fastjson"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func parseWebSocketEvent(str []byte) (interface{}, error) {
	v, err := fastjson.ParseBytes(str)
	if err != nil {
		return nil, err
	}

	if v.Exists("event") {
		return parseEvent(v)
	}

	if v.Exists("data") {
		return parseData(v)
	}

	return nil, nil
}

type WebSocketEvent struct {
	Event   string      `json:"event"`
	Code    string      `json:"code,omitempty"`
	Message string      `json:"msg,omitempty"`
	Arg     interface{} `json:"arg,omitempty"`
}

func parseEvent(v *fastjson.Value) (*WebSocketEvent, error) {
	// event could be "subscribe", "unsubscribe" or "error"
	event := string(v.GetStringBytes("event"))
	code := string(v.GetStringBytes("code"))
	message := string(v.GetStringBytes("msg"))
	arg := v.GetObject("arg")
	return &WebSocketEvent{
		Event:   event,
		Code:    code,
		Message: message,
		Arg:     arg,
	}, nil
}

type BookEvent struct {
	InstrumentID         string
	Symbol               string
	Action               string
	Bids                 []BookEntry
	Asks                 []BookEntry
	MillisecondTimestamp int64
	Checksum             int
	channel              string
}

func (data *BookEvent) BookTicker() types.BookTicker {
	ticker := types.BookTicker{
		Symbol: data.Symbol,
	}

	if len(data.Bids) > 0 {
		ticker.Buy = data.Bids[0].Price
		ticker.BuySize = data.Bids[0].Volume
	}

	if len(data.Asks) > 0 {
		ticker.Sell = data.Asks[0].Price
		ticker.SellSize = data.Asks[0].Volume
	}

	return ticker
}

func (data *BookEvent) Book() types.SliceOrderBook {
	book := types.SliceOrderBook{
		Symbol: data.Symbol,
		Time:   types.NewMillisecondTimestampFromInt(data.MillisecondTimestamp).Time(),
	}

	for _, bid := range data.Bids {
		book.Bids = append(book.Bids, types.PriceVolume{Price: bid.Price, Volume: bid.Volume})
	}

	for _, ask := range data.Asks {
		book.Asks = append(book.Asks, types.PriceVolume{Price: ask.Price, Volume: ask.Volume})
	}

	return book
}

type BookEntry struct {
	Price         fixedpoint.Value
	Volume        fixedpoint.Value
	NumLiquidated int
	NumOrders     int
}

func parseBookEntry(v *fastjson.Value) (*BookEntry, error) {
	arr, err := v.Array()
	if err != nil {
		return nil, err
	}

	if len(arr) < 4 {
		return nil, fmt.Errorf("unexpected book entry size: %d", len(arr))
	}

	price := fixedpoint.Must(fixedpoint.NewFromString(string(arr[0].GetStringBytes())))
	volume := fixedpoint.Must(fixedpoint.NewFromString(string(arr[1].GetStringBytes())))
	numLiquidated, err := strconv.Atoi(string(arr[2].GetStringBytes()))
	if err != nil {
		return nil, err
	}

	numOrders, err := strconv.Atoi(string(arr[3].GetStringBytes()))
	if err != nil {
		return nil, err
	}

	return &BookEntry{
		Price:         price,
		Volume:        volume,
		NumLiquidated: numLiquidated,
		NumOrders:     numOrders,
	}, nil
}

func parseBookData(v *fastjson.Value) (*BookEvent, error) {
	instrumentId := string(v.GetStringBytes("arg", "instId"))
	data := v.GetArray("data")
	if len(data) == 0 {
		return nil, errors.New("empty data payload")
	}

	// "snapshot" or "update"
	action := string(v.GetStringBytes("action"))

	millisecondTimestamp, err := strconv.ParseInt(string(data[0].GetStringBytes("ts")), 10, 64)
	if err != nil {
		return nil, err
	}

	checksum := data[0].GetInt("checksum")

	var asks []BookEntry
	var bids []BookEntry

	for _, v := range data[0].GetArray("asks") {
		entry, err := parseBookEntry(v)
		if err != nil {
			return nil, err
		}
		asks = append(asks, *entry)
	}

	for _, v := range data[0].GetArray("bids") {
		entry, err := parseBookEntry(v)
		if err != nil {
			return nil, err
		}
		bids = append(bids, *entry)
	}

	return &BookEvent{
		InstrumentID:         instrumentId,
		Symbol:               toGlobalSymbol(instrumentId),
		Action:               action,
		Bids:                 bids,
		Asks:                 asks,
		Checksum:             checksum,
		MillisecondTimestamp: millisecondTimestamp,
	}, nil
}

type Candle struct {
	Channel      string
	InstrumentID string
	Symbol       string
	Interval     string
	Open         fixedpoint.Value
	High         fixedpoint.Value
	Low          fixedpoint.Value
	Close        fixedpoint.Value

	// Trading volume, with a unit of contact.
	// If it is a derivatives contract, the value is the number of contracts.
	// If it is SPOT/MARGIN, the value is the amount of trading currency.
	Volume fixedpoint.Value

	// Trading volume, with a unit of currency.
	// If it is a derivatives contract, the value is the number of settlement currency.
	// If it is SPOT/MARGIN, the value is the number of quote currency.
	VolumeInCurrency fixedpoint.Value

	MillisecondTimestamp int64

	StartTime time.Time
}

func (c *Candle) KLine() types.KLine {
	interval := types.Interval(c.Interval)
	endTime := c.StartTime.Add(interval.Duration() - 1*time.Millisecond)
	return types.KLine{
		Exchange:    types.ExchangeOKEx,
		Interval:    interval,
		Open:        c.Open,
		High:        c.High,
		Low:         c.Low,
		Close:       c.Close,
		Volume:      c.Volume,
		QuoteVolume: c.VolumeInCurrency,
		StartTime:   types.Time(c.StartTime),
		EndTime:     types.Time(endTime),
	}
}

func parseCandle(channel string, v *fastjson.Value) (*Candle, error) {
	instrumentID := string(v.GetStringBytes("arg", "instId"))
	data, err := v.Get("data").Array()
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, errors.New("candle data is empty")
	}

	arr, err := data[0].Array()
	if err != nil {
		return nil, err
	}

	if len(arr) < 7 {
		return nil, fmt.Errorf("unexpected candle data length: %d", len(arr))
	}

	interval := strings.ToLower(strings.TrimPrefix(channel, "candle"))

	timestamp, err := strconv.ParseInt(string(arr[0].GetStringBytes()), 10, 64)
	if err != nil {
		return nil, err
	}

	open, err := fixedpoint.NewFromString(string(arr[1].GetStringBytes()))
	if err != nil {
		return nil, err
	}

	high, err := fixedpoint.NewFromString(string(arr[2].GetStringBytes()))
	if err != nil {
		return nil, err
	}

	low, err := fixedpoint.NewFromString(string(arr[3].GetStringBytes()))
	if err != nil {
		return nil, err
	}

	cls, err := fixedpoint.NewFromString(string(arr[4].GetStringBytes()))
	if err != nil {
		return nil, err
	}

	vol, err := fixedpoint.NewFromString(string(arr[5].GetStringBytes()))
	if err != nil {
		return nil, err
	}

	volCurrency, err := fixedpoint.NewFromString(string(arr[6].GetStringBytes()))
	if err != nil {
		return nil, err
	}

	candleTime := time.Unix(0, timestamp*int64(time.Millisecond))
	return &Candle{
		Channel:              channel,
		InstrumentID:         instrumentID,
		Symbol:               toGlobalSymbol(instrumentID),
		Interval:             interval,
		Open:                 open,
		High:                 high,
		Low:                  low,
		Close:                cls,
		Volume:               vol,
		VolumeInCurrency:     volCurrency,
		MillisecondTimestamp: timestamp,
		StartTime:            candleTime,
	}, nil
}

func parseAccount(v *fastjson.Value) (*okexapi.Account, error) {
	data := v.Get("data").MarshalTo(nil)

	var accounts []okexapi.Account
	err := json.Unmarshal(data, &accounts)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, errors.New("empty account data")
	}

	return &accounts[0], nil
}

func parseOrder(v *fastjson.Value) ([]okexapi.OrderDetails, error) {
	data := v.Get("data").MarshalTo(nil)

	var orderDetails []okexapi.OrderDetails
	err := json.Unmarshal(data, &orderDetails)
	if err != nil {
		return nil, err
	}

	return orderDetails, nil
}

func parseData(v *fastjson.Value) (interface{}, error) {

	channel := string(v.GetStringBytes("arg", "channel"))

	switch channel {
	case "books5":
		data, err := parseBookData(v)
		data.channel = channel
		return data, err
	case "books":
		data, err := parseBookData(v)
		data.channel = channel
		return data, err
	case "account":
		return parseAccount(v)
	case "orders":
		return parseOrder(v)
	default:
		if strings.HasPrefix(channel, "candle") {
			data, err := parseCandle(channel, v)
			return data, err
		}

	}

	return nil, nil
}
