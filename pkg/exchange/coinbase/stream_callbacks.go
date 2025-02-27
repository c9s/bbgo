// Code generated by "callbackgen -type Stream"; DO NOT EDIT.

package coinbase

import ()

func (S *Stream) OnStatusMessage(cb func(m *StatusMessage)) {
	S.statusMessageCallbacks = append(S.statusMessageCallbacks, cb)
}

func (S *Stream) EmitStatusMessage(m *StatusMessage) {
	for _, cb := range S.statusMessageCallbacks {
		cb(m)
	}
}

func (S *Stream) OnAuctionMessage(cb func(m *AuctionMessage)) {
	S.auctionMessageCallbacks = append(S.auctionMessageCallbacks, cb)
}

func (S *Stream) EmitAuctionMessage(m *AuctionMessage) {
	for _, cb := range S.auctionMessageCallbacks {
		cb(m)
	}
}

func (S *Stream) OnRfqMessage(cb func(m *RfqMessage)) {
	S.rfqMessageCallbacks = append(S.rfqMessageCallbacks, cb)
}

func (S *Stream) EmitRfqMessage(m *RfqMessage) {
	for _, cb := range S.rfqMessageCallbacks {
		cb(m)
	}
}

func (S *Stream) OnTickerMessage(cb func(m *TickerMessage)) {
	S.tickerMessageCallbacks = append(S.tickerMessageCallbacks, cb)
}

func (S *Stream) EmitTickerMessage(m *TickerMessage) {
	for _, cb := range S.tickerMessageCallbacks {
		cb(m)
	}
}

func (S *Stream) OnReceivedMessage(cb func(m *ReceivedMessage)) {
	S.receivedMessageCallbacks = append(S.receivedMessageCallbacks, cb)
}

func (S *Stream) EmitReceivedMessage(m *ReceivedMessage) {
	for _, cb := range S.receivedMessageCallbacks {
		cb(m)
	}
}

func (S *Stream) OnOpenMessage(cb func(m *OpenMessage)) {
	S.openMessageCallbacks = append(S.openMessageCallbacks, cb)
}

func (S *Stream) EmitOpenMessage(m *OpenMessage) {
	for _, cb := range S.openMessageCallbacks {
		cb(m)
	}
}

func (S *Stream) OnDoneMessage(cb func(m *DoneMessage)) {
	S.doneMessageCallbacks = append(S.doneMessageCallbacks, cb)
}

func (S *Stream) EmitDoneMessage(m *DoneMessage) {
	for _, cb := range S.doneMessageCallbacks {
		cb(m)
	}
}

func (S *Stream) OnMatchMessage(cb func(m *MatchMessage)) {
	S.matchMessageCallbacks = append(S.matchMessageCallbacks, cb)
}

func (S *Stream) EmitMatchMessage(m *MatchMessage) {
	for _, cb := range S.matchMessageCallbacks {
		cb(m)
	}
}

func (S *Stream) OnChangeMessage(cb func(m *ChangeMessage)) {
	S.changeMessageCallbacks = append(S.changeMessageCallbacks, cb)
}

func (S *Stream) EmitChangeMessage(m *ChangeMessage) {
	for _, cb := range S.changeMessageCallbacks {
		cb(m)
	}
}

func (S *Stream) OnActiveMessage(cb func(m *ActiveMessage)) {
	S.activeMessageCallbacks = append(S.activeMessageCallbacks, cb)
}

func (S *Stream) EmitActiveMessage(m *ActiveMessage) {
	for _, cb := range S.activeMessageCallbacks {
		cb(m)
	}
}
