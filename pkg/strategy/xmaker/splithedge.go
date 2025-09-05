package xmaker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type SplitHedgeAlgo string

const (
	SplitHedgeAlgoProportion SplitHedgeAlgo = "proportion"
)

type SplitHedgeProportionMarket struct {
	Name string `json:"name"`

	Ratio fixedpoint.Value `json:"ratio"`

	MaxQuantity fixedpoint.Value `json:"maxQuantity"`

	MaxQuoteQuantity fixedpoint.Value `json:"maxQuoteQuantity"`
}

// CalculateQuantity calculates the order quantity for this market based on total quantity and price.
// It applies ratio, maxQuantity, and maxQuoteQuantity constraints.
func (m *SplitHedgeProportionMarket) CalculateQuantity(totalQuantity, price fixedpoint.Value) fixedpoint.Value {
	allocated := totalQuantity.Mul(m.Ratio)
	finalQuantity := allocated

	if m.MaxQuantity.Sign() > 0 {
		if finalQuantity.Compare(m.MaxQuantity) > 0 {
			finalQuantity = m.MaxQuantity
		}
	}

	if m.MaxQuoteQuantity.Sign() > 0 && price.Sign() > 0 {
		maxByQuote := m.MaxQuoteQuantity.Div(price)
		if finalQuantity.Compare(maxByQuote) > 0 {
			finalQuantity = maxByQuote
		}
	}

	log.Infof("calc quantity for market %s: ratio=%.4f, total=%s, allocated=%s, price=%s, finalQuantity=%s",
		m.Name,
		m.Ratio,
		totalQuantity.String(), allocated.String(), price.String(), finalQuantity.String())

	return finalQuantity
}

type SplitHedgeProportionAlgo struct {
	ProportionMarkets []*SplitHedgeProportionMarket `json:"markets"`
}

type SplitHedge struct {
	Enabled bool `json:"enabled"`

	Algo SplitHedgeAlgo `json:"algo"`

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

	// config validation
	if h.Enabled {
		switch h.Algo {
		case SplitHedgeAlgoProportion:
			if h.ProportionAlgo == nil || len(h.ProportionAlgo.ProportionMarkets) == 0 {
				return fmt.Errorf("splithedge: proportionAlgo.markets must not be empty when enabled and algo=proportion")
			}
			for i, m := range h.ProportionAlgo.ProportionMarkets {
				if m.Name == "" {
					return fmt.Errorf("splithedge: market at index %d missing name", i)
				}
				if i != len(h.ProportionAlgo.ProportionMarkets)-1 {
					if m.Ratio.Sign() <= 0 {
						return fmt.Errorf("splithedge: prior market %s ratio must be positive and can not be zero", m.Name)
					}
				}
			}
		default:
			return fmt.Errorf("splithedge: invalid algo: %s", h.Algo)
		}
	}

	return nil
}

// TODO: see if we can remove this *Strategy dependency, it's a huge effort to refactor it out
func (h *SplitHedge) InitializeAndBind(sessions map[string]*bbgo.ExchangeSession, strategy *Strategy) error {
	// a simple guard clause
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

		// handle position cover and close here
		hedgeMarket.positionExposure.OnCover(strategy.positionExposure.Cover)

		hedgeMarket.tradeCollector.OnTrade(func(
			trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value,
		) {
			strategy.positionExposure.Close(trade.PositionDelta())
		})
	}

	h.strategy = strategy
	h.logger = strategy.logger.WithField("component", "split_hedge")
	return nil
}

func (h *SplitHedge) hedgeWithProportionAlgo(
	ctx context.Context,
	uncoveredPosition fixedpoint.Value,
) error {
	if h.ProportionAlgo == nil || len(h.ProportionAlgo.ProportionMarkets) == 0 {
		return fmt.Errorf("proportion algo requires proportion markets")
	}

	delta := uncoveredToDelta(uncoveredPosition)
	side := deltaToSide(delta)
	remainingQuantity := delta.Abs()

	for _, proportionMarket := range h.ProportionAlgo.ProportionMarkets {
		hedgeMarket, ok := h.hedgeMarketInstances[proportionMarket.Name]
		if !ok {
			h.logger.Warnf("[splitHedge] hedge market %s not found", proportionMarket.Name)
			continue
		}

		bid, ask := hedgeMarket.getQuotePrice()
		price := sideTakerPrice(bid, ask, side)
		proportionQuantity := proportionMarket.CalculateQuantity(remainingQuantity, price)
		proportionQuantity = AdjustHedgeQuantityWithAvailableBalance(
			hedgeMarket.session.GetAccount(), hedgeMarket.market, side, proportionQuantity, price,
		)

		proportionQuantity = hedgeMarket.market.TruncateQuantity(proportionQuantity)
		if hedgeMarket.market.IsDustQuantity(proportionQuantity, price) {
			h.logger.Infof("skip dust quantity: %s @ price %f", proportionQuantity.String(), price.Float64())
			continue
		}

		if proportionQuantity.IsZero() {
			h.logger.Infof("skip zero quantity")
			continue
		}

		remainingQuantity = remainingQuantity.Sub(proportionQuantity)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case hedgeMarket.positionDeltaC <- quantityToDelta(proportionQuantity, side):
		}
	}

	return nil
}

func (h *SplitHedge) Hedge(
	ctx context.Context,
	uncoveredPosition fixedpoint.Value,
) error {
	if uncoveredPosition.IsZero() {
		return nil
	}

	h.logger.Infof("[splitHedge] split hedging with delta: %f", uncoveredPosition.Float64())

	switch h.Algo {
	case SplitHedgeAlgoProportion:
		return h.hedgeWithProportionAlgo(ctx, uncoveredPosition)

	default:
		return fmt.Errorf("invalid split hedge algo: %s", h.Algo)
	}
}

func (h *SplitHedge) Start(ctx context.Context) error {
	if !h.Enabled {
		return nil
	}

	instanceID := ID + "-splithedge"
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

func (h *SplitHedge) Stop(shutdownCtx context.Context) error {
	h.logger.Infof("[splitHedge] stopping synthetic hedge workers")
	for _, hedgeMarket := range h.hedgeMarketInstances {
		hedgeMarket.Stop(shutdownCtx)
	}

	instanceID := ID

	if h.strategy != nil {
		instanceID = h.strategy.InstanceID()
	}

	for _, hedgeMarket := range h.hedgeMarketInstances {
		hedgeMarket.Sync(hedgeMarket.tradingCtx, instanceID)
	}

	h.logger.Infof("[splitHedge] synthetic hedge workers stopped")
	return nil
}
