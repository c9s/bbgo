package kucoin

import (
	"encoding/json"

	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
)

func parseWebsocketPayload(in []byte) (*kucoinapi.WebSocketEvent, error) {
	var resp kucoinapi.WebSocketEvent
	var err = json.Unmarshal(in, &resp)
	if err != nil {
		return nil, err
	}

	switch resp.Type {
	case kucoinapi.WebSocketMessageTypeAck:
		return &resp, nil

	case kucoinapi.WebSocketMessageTypeMessage:
		switch resp.Subject {
		case kucoinapi.WebSocketSubjectOrderChange:
			var o kucoinapi.WebSocketPrivateOrder
			if err := json.Unmarshal(resp.Data, &o); err != nil {
				return &resp, err
			}
			resp.Object = &o

		case kucoinapi.WebSocketSubjectAccountBalance:
			var o kucoinapi.WebSocketAccountBalance
			if err := json.Unmarshal(resp.Data, &o); err != nil {
				return &resp, err
			}
			resp.Object = &o

		case kucoinapi.WebSocketSubjectTradeCandlesUpdate:
			var o kucoinapi.WebSocketCandle
			if err := json.Unmarshal(resp.Data, &o); err != nil {
				return &resp, err
			}
			resp.Object = &o

		case kucoinapi.WebSocketSubjectTradeL2Update:
			var o kucoinapi.WebSocketOrderBookL2
			if err := json.Unmarshal(resp.Data, &o); err != nil {
				return &resp, err
			}
			resp.Object = &o

		case kucoinapi.WebSocketSubjectTradeTicker:
			var o kucoinapi.WebSocketTicker
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
