package xmaker

import (
	"encoding/json"

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
	}

	h.strategy = strategy
	return nil
}
