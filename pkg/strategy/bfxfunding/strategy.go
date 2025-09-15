package bfxfunding

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/bitfinex"
	"github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "bfxfunding"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Currency string           `json:"currency"`
	Amount   fixedpoint.Value `json:"amount"`
	Period   int              `json:"period"`

	MinRate fixedpoint.Value `json:"minRate"`

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
	s.Currency = "fUSD"
	s.MinRate = fixedpoint.NewFromFloat(0.01 * 0.01)
	return nil
}

func (s *Strategy) Validate() error {
	if s.Currency == "" {
		return fmt.Errorf("currency is required")
	}

	if s.Amount.IsZero() {
		return fmt.Errorf("amount must be greater than 0")
	}

	if s.Period <= 0 {
		return fmt.Errorf("period must be greater than 0")
	}

	if s.MinRate.IsZero() {
		return fmt.Errorf("minRate must be greater than 0")
	}

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

func (s *Strategy) bookFunding(rate fixedpoint.Value) {
	req := s.client.Funding().NewSubmitFundingOfferRequest()
	req.Symbol(s.Currency).
		Amount(s.Amount.String()).
		OfferType(bfxapi.FundingOfferTypeLimit).
		Rate(rate.String()).
		Period(s.Period).Notify(true).
		Hidden(false).
		AutoRenew(true)
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
