package kucoin

import (
	"encoding/json"
	"strings"

	"github.com/c9s/bbgo/pkg/types"
)

func parseWebsocketPayload(in []byte) (*WebSocketEvent, error) {
	var resp WebSocketEvent
	var err = json.Unmarshal(in, &resp)
	if err != nil {
		return nil, err
	}

	switch resp.Type {
	case WebSocketMessageTypeAck:
		return &resp, nil

	case WebSocketMessageTypeError:
		resp.Object = string(resp.Data)
		return &resp, nil

	case WebSocketMessageTypeMessage:
		switch resp.Subject {
		case WebSocketSubjectOrderChange:
			var o WebSocketPrivateOrderEvent
			if err := json.Unmarshal(resp.Data, &o); err != nil {
				return &resp, err
			}
			resp.Object = &o

		case WebSocketSubjectAccountBalance:
			var o WebSocketAccountBalanceEvent
			if err := json.Unmarshal(resp.Data, &o); err != nil {
				return &resp, err
			}
			resp.Object = &o

		case WebSocketSubjectTradeCandlesUpdate, WebSocketSubjectTradeCandlesAdd:
			var o WebSocketCandleEvent
			if err := json.Unmarshal(resp.Data, &o); err != nil {
				return &resp, err
			}

			o.Interval = extractIntervalFromTopic(resp.Topic)
			o.Add = resp.Subject == WebSocketSubjectTradeCandlesAdd
			resp.Object = &o

		case WebSocketSubjectTradeL2Update:
			var o WebSocketOrderBookL2Event
			if err := json.Unmarshal(resp.Data, &o); err != nil {
				return &resp, err
			}
			resp.Object = &o

		case WebSocketSubjectTradeTicker:
			var o WebSocketTickerEvent
			if err := json.Unmarshal(resp.Data, &o); err != nil {
				return &resp, err
			}
			resp.Object = &o

		default:
			// return nil, fmt.Errorf("kucoin: unsupported subject: %s", resp.Subject)

		}
	}

	return &resp, nil
}

func extractIntervalFromTopic(topic string) types.Interval {
	ta := strings.Split(topic, ":")
	tb := strings.Split(ta[1], "_")
	interval := tb[1]
	return toGlobalInterval(interval)
}

func toGlobalInterval(a string) types.Interval {
	switch a {
	case "1min":
		return types.Interval1m
	case "15min":
		return types.Interval15m
	case "30min":
		return types.Interval30m
	case "1hour":
		return types.Interval1h
	case "2hour":
		return types.Interval2h
	case "4hour":
		return types.Interval4h
	case "6hour":
		return types.Interval6h
	case "12hour":
		return types.Interval12h

	}
	return ""
}
