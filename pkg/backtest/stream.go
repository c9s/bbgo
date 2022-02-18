package backtest

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/types"
)

var log = logrus.WithField("cmd", "backtest")

type Stream struct {
	types.StandardStream

	exchange *Exchange
}

func (s *Stream) Connect(ctx context.Context) error {
	if s.PublicOnly {
		if s.exchange.marketDataStream != nil {
			panic("you should not set up more than 1 market data stream in back-test")
		}
		s.exchange.marketDataStream = s
	} else {

		// assign user data stream back
		if s.exchange.userDataStream != nil {
			panic("you should not set up more than 1 user data stream in back-test")
		}
		s.exchange.userDataStream = s
	}

	s.EmitConnect()
	s.EmitStart()
	return nil
}

func (s *Stream) Close() error {
	return nil
}
