package okex

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/valyala/fastjson"
)

func Parse(str string) (interface{}, error) {
	v, err := fastjson.Parse(str)
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
	Event   string
	Code    string
	Message string
}

func parseEvent(v *fastjson.Value) (*WebSocketEvent, error) {
	// event could be "subscribe", "unsubscribe" or "error"
	event := string(v.GetStringBytes("event"))
	code := string(v.GetStringBytes("code"))
	message := string(v.GetStringBytes("message"))
	return &WebSocketEvent{
		Event:   event,
		Code:    code,
		Message: message,
	}, nil
}

type BookData struct {
	InstrumentID         string
	Symbol               string
	Action               string
	Bids                 []BookEntry
	Asks                 []BookEntry
	MillisecondTimestamp int64
	Checksum             int
}

func (data *BookData) Book() types.SliceOrderBook {
	book := types.SliceOrderBook{
		Symbol: data.Symbol,
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

func parseBookData(instrumentId string, v *fastjson.Value) (*BookData, error) {
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

	return &BookData{
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
		Interval:  interval,
		Open:      c.Open.Float64(),
		High:      c.High.Float64(),
		Low:       c.Low.Float64(),
		Close:     c.Close.Float64(),
		StartTime: c.StartTime,
		EndTime:   endTime,
	}
}

func parseCandle(channel, instrumentID string, v *fastjson.Value) (*Candle, error) {
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

func parseData(v *fastjson.Value) (interface{}, error) {
	instrumentId := string(v.GetStringBytes("arg", "instId"))
	channel := string(v.GetStringBytes("arg", "channel"))

	switch channel {
	case "books":
		return parseBookData(instrumentId, v)

	default:
		if strings.HasPrefix(channel, "candle") {
			return parseCandle(channel, instrumentId, v)
		}

	}

	return nil, nil
}
