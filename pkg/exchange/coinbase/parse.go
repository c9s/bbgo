package coinbase

import (
	"encoding/json"
	"errors"
)

// See https://docs.cdp.coinbase.com/exchange/docs/websocket-channels for message types
func (s *Stream) parseMessage(data []byte) (interface{}, error) {
	var msgType string
	{
		var e messageBaseType
		json.Unmarshal(data, &e)
		msgType = e.Type
	}

	switch msgType {
	case "heartbeat":
		var msg HeartbeatMessage
		json.Unmarshal(data, &msg)
		return &msg, nil
	case "status":
		var msg StatusMessage
		json.Unmarshal(data, &msg)
		return &msg, nil
	case "auction":
		var msg AuctionMessage
		json.Unmarshal(data, &msg)
		return &msg, nil
	case "rfq_match":
		var msg RfqMessage
		json.Unmarshal(data, &msg)
		return &msg, nil
	case "ticker":
		var msg TickerMessage
		json.Unmarshal(data, &msg)
		return &msg, nil
	case "received":
		// try market order first
		{
			var msg ReceivedMarketOrderMessage
			json.Unmarshal(data, &msg)
			if !msg.Funds.IsZero() {
				return &msg, nil
			}
		}
		var msg ReceivedLimitOrderMessage
		json.Unmarshal(data, &msg)
		return &msg, nil
	case "open":
		var msg OpenMessage
		json.Unmarshal(data, &msg)
		return &msg, nil
	case "done":
		var msg DoneMessage
		json.Unmarshal(data, &msg)
		return &msg, nil
	case "match", "last_match":
		// authenticated stream
		if !s.PublicOnly {
			// try maker order first
			{
				var msg AuthMakerMatchMessage
				json.Unmarshal(data, &msg)
				if len(msg.MakerUserID) > 0 {
					return &msg, nil
				}
			}
			// should be taker order
			var msg AuthTakerMatchMessage
			json.Unmarshal(data, &msg)
			return &msg, nil
		}
		// public stream
		var msg MatchMessage
		json.Unmarshal(data, &msg)
		return &msg, nil
	case "change":
		var reason string
		{
			var e changeMessageType
			json.Unmarshal(data, &e)
			reason = e.Reason
		}
		switch reason {
		case "stp":
			var msg StpChangeMessage
			json.Unmarshal(data, &msg)
			return &msg, nil
		case "modify_order":
			var msg ModifyOrderChangeMessage
			json.Unmarshal(data, &msg)
			return &msg, nil
		}
	case "active":
		var msg ActiveMessage
		json.Unmarshal(data, &msg)
		return &msg, nil
	}
	return nil, errors.New("unknown message type")
}
