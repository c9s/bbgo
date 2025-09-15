package bfxfunding

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/bitfinex"
	"github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "bfxfunding"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Currency string `json:"currency"`

	exchange *bitfinex.Exchange
	stream   *bitfinex.Stream

	client *bfxapi.Client

	logger logrus.FieldLogger
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s", ID, s.Currency)
}

func (s *Strategy) Defaults() error {
	return nil
}

func (s *Strategy) Initialize() error {
	s.logger = logrus.WithFields(logrus.Fields{"strategy": s.InstanceID(), "currency": s.Currency})
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// session.Subscribe(types.KLineChannel, s.Currency, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) handleFundingOfferSnapshot(e *bfxapi.FundingOfferSnapshotEvent) {
	s.logger.Infof("funding offer snapshot event: %+v", e)
}

func (s *Strategy) handleFundingOfferUpdate(e *bfxapi.FundingOfferUpdateEvent) {
	s.logger.Infof("funding offer update event: %+v", e)
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if session.ExchangeName != types.ExchangeBitfinex {
		return fmt.Errorf("bfxfunding strategy only works with bitfinex exchange")
	}

	ex, ok := session.Exchange.(*bitfinex.Exchange)
	if !ok {
		return fmt.Errorf("exchange is not bitfinex exchange")
	}

	s.exchange = ex
	s.client = s.exchange.GetApiClient()
	s.stream = session.UserDataStream.(*bitfinex.Stream)

	s.stream.OnFundingOfferSnapshotEvent(s.handleFundingOfferSnapshot)
	s.stream.OnFundingOfferUpdateEvent(s.handleFundingOfferUpdate)

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		bbgo.Sync(ctx, s)
	})

	return nil
}
