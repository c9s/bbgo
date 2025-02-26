package coinbase

import (
	"encoding/json"
	"errors"
	"fmt"
)

// See https://docs.cdp.coinbase.com/exchange/docs/websocket-channels for message types
func (s *Stream) parseMessage(data []byte) (msg interface{}, err error) {
	var baseMsg messageBaseType
	err = json.Unmarshal(data, &baseMsg)
	if err != nil {
		return
	}

	switch baseMsg.Type {
	case "heartbeat":
		var heartbeatMsg HeartbeatMessage
		err = json.Unmarshal(data, &heartbeatMsg)
		if err == nil {
			msg = heartbeatMsg
		}
	case "status":
		var statusMsg StatusMessage
		err = json.Unmarshal(data, &statusMsg)
		if err == nil {
			msg = statusMsg
		}
	case "auction":
		var aucMsg AuctionMessage
		err = json.Unmarshal(data, &aucMsg)
		if err == nil {
			msg = aucMsg
		}
	case "rfq_match":
		var rfqMsg RfqMessage
		err = json.Unmarshal(data, &rfqMsg)
		if err == nil {
			msg = rfqMsg
		}
	case "ticker":
		var tickerMsg TickerMessage
		err = json.Unmarshal(data, &tickerMsg)
		if err == nil {
			msg = tickerMsg
		}
	case "received":
		// try market order first
		var marketOrderMsg ReceivedMarketOrderMessage
		err = json.Unmarshal(data, &marketOrderMsg)
		done := false
		if err != nil && !marketOrderMsg.Funds.IsZero() {
			msg = marketOrderMsg
			done = true
		}
		// try limit order
		if !done {
			var limitOrderMsg ReceivedLimitOrderMessage
			err = json.Unmarshal(data, &limitOrderMsg)
			if err == nil {
				msg = limitOrderMsg
			}
		}
	case "open":
		var openMsg OpenMessage
		err = json.Unmarshal(data, &openMsg)
		if err == nil {
			msg = openMsg
		}
	case "done":
		var doneMsg DoneMessage
		err = json.Unmarshal(data, &doneMsg)
		if err == nil {
			msg = doneMsg
		}
	case "match", "last_match":
		// authenticated stream
		done := false
		// try maker order first
		var makerMsg AuthMakerMatchMessage
		err = json.Unmarshal(data, &makerMsg)
		if err == nil && len(makerMsg.MakerUserID) > 0 {
			msg = makerMsg
			done = true
		}
		// try taker order
		if !done {
			var takerMsg AuthTakerMatchMessage
			err = json.Unmarshal(data, &takerMsg)
			if err == nil && len(takerMsg.TakerUserID) > 0 {
				msg = takerMsg
				done = true
			}
		}
		// public stream
		if !done {
			var publicMsg MatchMessage
			err = json.Unmarshal(data, &publicMsg)
			if err == nil {
				msg = publicMsg
			}
		}
	case "change":
		var changeMsg changeMessageType
		err = json.Unmarshal(data, &changeMsg)
		if err == nil {
			break
		}
		switch changeMsg.Reason {
		case "stp":
			var stpMsg StpChangeMessage
			err = json.Unmarshal(data, &stpMsg)
			if err == nil {
				msg = stpMsg
			}
		case "modify_order":
			var modifyMsg ModifyOrderChangeMessage
			err = json.Unmarshal(data, &modifyMsg)
			if err == nil {
				msg = modifyMsg
			}
		}
	case "active":
		var activeMsg ActiveMessage
		err = json.Unmarshal(data, &activeMsg)
		if err == nil {
			msg = activeMsg
		}
	default:
		err = errors.New(fmt.Sprintf("unknown message type: %s", baseMsg.Type))
	}
	return
}
