package coinbase

import (
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type Stream
type Stream struct {
	types.StandardStream

	// callbacks
	statusMessageCallbacks              []func(m *StatusMessage)
	auctionMessageCallbacks             []func(m *AuctionMessage)
	rfqMessageCallbacks                 []func(m *RfqMessage)
	tickerMessageCallbacks              []func(m *TickerMessage)
	receivedLimitOrderMessageCallbacks  []func(m *ReceivedLimitOrderMessage)
	receivedMarketOrderMessageCallbacks []func(m *ReceivedMarketOrderMessage)
	openMessageCallbacks                []func(m *OpenMessage)
	doneMessageCallbacks                []func(m *DoneMessage)
	matchMessageCallbacks               []func(m *MatchMessage)
	authMakerMatchMessageCallbacks      []func(m *AuthMakerMatchMessage)
	authTakerMatchMessageCallbacks      []func(m *AuthTakerMatchMessage)
	stpChangeMessageCallbacks           []func(m *StpChangeMessage)
	modifyOrderChangeMessageCallbacks   []func(m *ModifyOrderChangeMessage)
	activeMessageCallbacks              []func(m *ActiveMessage)
}

func NewStream() *Stream {
	s := Stream{
		StandardStream: types.NewStandardStream(),
	}
	return &s
}

// func (s *Stream) handleAuth() {
// 	return
// }
