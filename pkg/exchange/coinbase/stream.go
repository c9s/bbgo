package coinbase

import (
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	// callbacks
	statusMessageCallbacks   []func(m *StatusMessage)
	auctionMessageCallbacks  []func(m *AuctionMessage)
	rfqMessageCallbacks      []func(m *RfqMessage)
	tickerMessageCallbacks   []func(m *TickerMessage)
	receivedMessageCallbacks []func(m *ReceivedMessage)
	openMessageCallbacks     []func(m *OpenMessage)
	doneMessageCallbacks     []func(m *DoneMessage)
	matchMessageCallbacks    []func(m *MatchMessage)
	changeMessageCallbacks   []func(m *ChangeMessage)
	activeMessageCallbacks   []func(m *ActiveMessage)
}

func NewStream() *Stream {
	s := Stream{
		StandardStream: types.NewStandardStream(),
	}
	s.SetParser(s.parseMessage)
	return &s
}
