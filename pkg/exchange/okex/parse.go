package okex

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func parseWebSocketEvent(in []byte) (interface{}, error) {
	var e WsEvent

	err := json.Unmarshal(in, &e)
	if err != nil {
		return nil, err
	}

	switch {
	case e.IsOp():
		return e.WebSocketOpEvent, nil
	case e.IsPushDataEvent():
		// need unmarshal again because arg in both WebSocketOpEvent and WebSocketPushDataEvent
		var pushDataEvent WebSocketPushDataEvent

		err := json.Unmarshal(in, &pushDataEvent)
		if err != nil {
			return nil, err
		}

		channel := pushDataEvent.Arg.Channel

		switch channel {

		case string(WsChannelTypeBooks5):
			data, err := parseBookData(&pushDataEvent)
			return data, err
		case string(WsChannelTypeBooks):
			data, err := parseBookData(&pushDataEvent)
			return data, err
		case string(WsChannelTypeAccount):
			data, err := parseAccount(&pushDataEvent)
			return data, err
		case string(WsChannelTypeOrders):
			data, err := parseOrder(&pushDataEvent)
			return data, err
		default:
			if strings.HasPrefix(channel, "candle") {
				data, err := parseCandle(channel, &pushDataEvent)
				return data, err
			}
		}
	}

	return nil, fmt.Errorf("unhandled websocket event: %+v", string(in))
}

// type WebSocketEvent struct {
// 	Event   string      `json:"event"`
// 	Code    string      `json:"code,omitempty"`
// 	Message string      `json:"msg,omitempty"`
// 	Arg     interface{} `json:"arg,omitempty"`
// }

// func parseEvent(v *WsEvent) (*WebSocketOpEvent, error) {
// 	// event could be "subscribe", "unsubscribe" or "error"
// 	// event := string(v.GetStringBytes("event"))
// 	// code := string(v.GetStringBytes("code"))
// 	// message := string(v.GetStringBytes("msg"))
// 	// arg := v.GetObject("arg")
// 	return &WebSocketOpEvent{
// 		Event:   v.WebSocketOpEvent.Event,
// 		Code:    v.WebSocketOpEvent.Code,
// 		Message: v.WebSocketOpEvent.Message,
// 		Arg:     v.WebSocketOpEvent.Arg,
// 	}, nil
// }

type BookEvent struct {
	InstrumentID         string
	Symbol               string
	Action               string
	Bids                 [][]fixedpoint.Value       `json:"bids"`
	Asks                 [][]fixedpoint.Value       `json:"asks"`
	MillisecondTimestamp types.MillisecondTimestamp `json:"ts"`
	Checksum             int                        `json:"checksum"`
	channel              string
}

func (data *BookEvent) BookTicker() types.BookTicker {
	ticker := types.BookTicker{
		Symbol: data.Symbol,
	}

	if len(data.Bids) > 0 {
		ticker.Buy = data.Bids[0][0]
		ticker.BuySize = data.Bids[0][1]
	}

	if len(data.Asks) > 0 {
		ticker.Sell = data.Asks[0][0]
		ticker.SellSize = data.Asks[0][1]
	}

	return ticker
}

func (data *BookEvent) Book() types.SliceOrderBook {
	book := types.SliceOrderBook{
		Symbol: data.Symbol,
		Time:   data.MillisecondTimestamp.Time(),
	}

	for i := range data.Bids {
		book.Bids = append(book.Bids, types.PriceVolume{Price: data.Bids[i][0], Volume: data.Bids[i][1]})
	}

	for j := range data.Asks {
		book.Asks = append(book.Asks, types.PriceVolume{Price: data.Asks[j][0], Volume: data.Asks[j][1]})
	}

	return book
}

// Order book channel
func parseBookData(v *WebSocketPushDataEvent) (*BookEvent, error) {
	instrumentId := v.Arg.InstrumentID
	data := v.Data
	var bookEvent []BookEvent
	if err := json.Unmarshal(data, &bookEvent); err != nil {
		return nil, err
	}

	action := v.Action

	checksum := bookEvent[0].Checksum

	return &BookEvent{
		InstrumentID:         instrumentId,
		Symbol:               toGlobalSymbol(instrumentId),
		Action:               *action,
		Bids:                 bookEvent[0].Bids,
		Asks:                 bookEvent[0].Asks,
		Checksum:             checksum,
		MillisecondTimestamp: bookEvent[0].MillisecondTimestamp,
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

// Candlesticks channel
func parseCandle(channel string, v *WebSocketPushDataEvent) (*Candle, error) {
	instrumentID := v.Arg.InstrumentID

	data := v.Data
	var dataPoints [][]string
	if err := json.Unmarshal(data, &dataPoints); err != nil {
		return nil, err
	}

	if len(dataPoints) == 0 {
		return nil, errors.New("candle data is empty")
	}

	if len(dataPoints[0]) < 7 { // okex actually return 9 points
		return nil, fmt.Errorf("unexpected candle data length: %d", len(dataPoints[0]))
	}

	interval := strings.ToLower(strings.TrimPrefix(channel, "candle"))

	timestamp, err := strconv.ParseInt(string(dataPoints[0][0]), 10, 64)
	if err != nil {
		return nil, err
	}

	open, err := fixedpoint.NewFromString(string(dataPoints[0][1]))
	if err != nil {
		return nil, err
	}

	high, err := fixedpoint.NewFromString(string(dataPoints[0][2]))
	if err != nil {
		return nil, err
	}

	low, err := fixedpoint.NewFromString(string(dataPoints[0][3]))
	if err != nil {
		return nil, err
	}

	cls, err := fixedpoint.NewFromString(string(dataPoints[0][4]))
	if err != nil {
		return nil, err
	}

	vol, err := fixedpoint.NewFromString(string(dataPoints[0][5]))
	if err != nil {
		return nil, err
	}

	volCurrency, err := fixedpoint.NewFromString(string(dataPoints[0][6]))
	if err != nil {
		return nil, err
	}

	candleTime := time.Unix(0, timestamp*int64(time.Millisecond))
	candle := &Candle{
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
	}
	return candle, nil
}

func parseAccount(v *WebSocketPushDataEvent) (*okexapi.Account, error) {
	data := v.Data

	var account []okexapi.Account
	if err := json.Unmarshal(data, &account); err != nil {
		return nil, err
	}

	if len(account) == 0 {
		return nil, fmt.Errorf("empty account")
	}
	return &account[0], nil
}

func parseOrder(v *WebSocketPushDataEvent) ([]okexapi.OrderDetails, error) {
	data := v.Data

	var orderDetails []okexapi.OrderDetails
	if err := json.Unmarshal(data, &orderDetails); err != nil {
		return nil, err
	}

	return orderDetails, nil
}
