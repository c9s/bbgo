package xmaker

import (
	"context"
	"encoding/json"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type SplitHedgeProportionMarket struct {
	Name string `json:"name"`

	Ratio float64 `json:"ratio"`

	MaxQuantity fixedpoint.Value `json:"maxQuantity"`

	MaxQuoteQuantity fixedpoint.Value `json:"maxQuoteQuantity"`
}

type SplitHedgeProportionAlgo struct {
	ProportionMarkets []*SplitHedgeProportionMarket `json:"markets"`
}

type SplitHedge struct {
	Enabled bool `json:"enabled"`

	Algo string `json:"algo"`

	ProportionAlgo *SplitHedgeProportionAlgo `json:"proportionAlgo"`

	// HedgeMarkets stores session name to hedge market config mapping.
	HedgeMarkets map[string]*HedgeMarketConfig `json:"hedgeMarketInstances"`

	// hedgeMarketInstances stores session name to hedge market instance mapping.
	hedgeMarketInstances map[string]*HedgeMarket

	strategy *Strategy

	logger logrus.FieldLogger
}

func (h *SplitHedge) UnmarshalJSON(data []byte) error {
	type Alias SplitHedge
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(h),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if h.HedgeMarkets == nil {
		h.HedgeMarkets = make(map[string]*HedgeMarketConfig)
	}

	if h.hedgeMarketInstances == nil {
		h.hedgeMarketInstances = make(map[string]*HedgeMarket)
	}

	return nil
}

func (h *SplitHedge) InitializeAndBind(sessions map[string]*bbgo.ExchangeSession, strategy *Strategy) error {
	if !h.Enabled {
		return nil
	}

	for name, config := range h.HedgeMarkets {
		hedgeMarket, err := initializeHedgeMarketFromConfig(config, sessions)
		if err != nil {
			return err
		}

		h.hedgeMarketInstances[name] = hedgeMarket

		hedgeMarket.Position.StrategyInstanceID = strategy.InstanceID()

		userDataStream := hedgeMarket.session.UserDataStream
		strategy.orderStore.BindStream(userDataStream)
		strategy.tradeCollector.BindStream(userDataStream)
	}

	h.strategy = strategy
	h.logger = strategy.logger.WithField("component", "split_hedge")
	return nil
}

func (h *SplitHedge) Start(ctx context.Context) error {
	if !h.Enabled {
		return nil
	}

	instanceID := ID
	if h.strategy != nil {
		instanceID = h.strategy.InstanceID()
	}

	for name, hedgeMarket := range h.hedgeMarketInstances {
		if err := hedgeMarket.Restore(ctx, instanceID); err != nil {
			h.logger.WithError(err).Errorf("[splitHedge] failed to restore hedge market %s persistence", name)
			return err
		}

		if err := hedgeMarket.Start(ctx); err != nil {
			h.logger.WithError(err).Errorf("[splitHedge] failed to start hedge market %s", name)
			return err
		}
	}

	for _, hedgeMarket := range h.hedgeMarketInstances {
		hedgeMarket.WaitForReady(ctx)
	}

	h.logger.Infof("[splitHedge] source market and fiat market are ready")
	return nil
}
