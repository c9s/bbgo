package coinbase

import (
	"encoding/json"
	"errors"
	"fmt"
)

// See https://docs.cdp.coinbase.com/exchange/docs/websocket-channels for message types
func (s *Stream) parseMessage(data []byte) (interface{}, error) {
	var baseMsg messageBaseType
	err := json.Unmarshal(data, &baseMsg)
	if err != nil {
		return nil, err
	}

	switch baseMsg.Type {
	case "heartbeat":
		var heartbeatMsg HeartbeatMessage
		err = json.Unmarshal(data, &heartbeatMsg)
		if err != nil {
			return nil, err
		}
		return &heartbeatMsg, nil
	case "status":
		var statusMsg StatusMessage
		err = json.Unmarshal(data, &statusMsg)
		if err != nil {
			return nil, err
		}
		return &statusMsg, nil
	case "auction":
		var aucMsg AuctionMessage
		err = json.Unmarshal(data, &aucMsg)
		if err != nil {
			return nil, err
		}
		return &aucMsg, nil
	case "rfq_match":
		var rfqMsg RfqMessage
		err = json.Unmarshal(data, &rfqMsg)
		if err != nil {
			return nil, err
		}
		return &rfqMsg, nil
	case "ticker":
		var tickerMsg TickerMessage
		err = json.Unmarshal(data, &tickerMsg)
		if err != nil {
			return nil, err
		}
		return &tickerMsg, nil
	case "received":
		var receivedMsg ReceivedMessage
		err = json.Unmarshal(data, &receivedMsg)
		if err != nil {
			return nil, err
		}
		return &receivedMsg, nil
	case "open":
		var openMsg OpenMessage
		err = json.Unmarshal(data, &openMsg)
		if err != nil {
			return nil, err
		}
		return &openMsg, nil
	case "done":
		var doneMsg DoneMessage
		err = json.Unmarshal(data, &doneMsg)
		if err != nil {
			return nil, err
		}
		return &doneMsg, nil
	case "match", "last_match":
		var matchMsg MatchMessage
		err = json.Unmarshal(data, &matchMsg)
		if err != nil {
			return nil, err
		}
		return &matchMsg, nil
	case "change":
		var changeMsg ChangeMessage
		err = json.Unmarshal(data, &changeMsg)
		if err != nil {
			return nil, err
		}
		return &changeMsg, nil
	case "active":
		var activeMsg ActiveMessage
		err = json.Unmarshal(data, &activeMsg)
		if err != nil {
			return nil, err
		}
		return &activeMsg, nil
	}
	return nil, errors.New(fmt.Sprintf("unknown message type: %s", baseMsg.Type))
}
