package okex

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/c9s/bbgo/pkg/fixedpoint"
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

func parseBookData(instrumentId string, v *fastjson.Value) (interface{}, error) {
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

func parseData(v *fastjson.Value) (interface{}, error) {
	instrumentId := string(v.GetStringBytes("arg", "instId"))
	channel := string(v.GetStringBytes("arg", "channel"))

	switch channel {
	case "books":
		return parseBookData(instrumentId, v)

	}

	return nil, nil
}
