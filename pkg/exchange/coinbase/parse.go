package coinbase

import (
	"encoding/json"
	"fmt"
)

// See https://docs.cdp.coinbase.com/exchange/docs/websocket-channels for message types
func parseMessage(data []byte) (interface{}, error) {
	var baseMsg messageBaseType
	err := json.Unmarshal(data, &baseMsg)
	if err != nil {
		return nil, err
	}

	switch baseMsg.Type {
	case "subscriptions":
		var subMsg SubscriptionsMessage
		err = json.Unmarshal(data, &subMsg)
		if err != nil {
			return nil, err
		}
		return &subMsg, nil
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
	case "activate":
		var activeMsg ActivateMessage
		err = json.Unmarshal(data, &activeMsg)
		if err != nil {
			return nil, err
		}
		return &activeMsg, nil
	case "balance":
		var balanceMsg BalanceMessage
		err = json.Unmarshal(data, &balanceMsg)
		if err != nil {
			return nil, err
		}
		return &balanceMsg, nil
	case "snapshot":
		var snapshotMsg OrderBookSnapshotMessage
		err = json.Unmarshal(data, &snapshotMsg)
		if err != nil {
			return nil, err
		}
		return &snapshotMsg, nil
	case "l2update":
		var updateMsg OrderBookUpdateMessage
		err = json.Unmarshal(data, &updateMsg)
		if err != nil {
			return nil, err
		}
		return &updateMsg, nil
	}
	return nil, fmt.Errorf("unknown message type: %s", baseMsg.Type)
}
