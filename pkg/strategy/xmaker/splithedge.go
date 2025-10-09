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
	allocated := totalQuantity
	if m.Ratio.Sign() > 0 {
		allocated = totalQuantity.Mul(m.Ratio)
	}

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

	log.Infof("splitHedge: calculated split quantity for market %s: ratio=%s, total=%s, allocated=%s, price=%s, finalQuantity=%s",
		m.Name,
		m.Ratio.String(),
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
	HedgeMarkets map[string]*HedgeMarketConfig `json:"hedgeMarkets"`

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
		return fmt.Errorf("hedgeMarkets must be provided")
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

				// ensure the hedge market config exists
				if _, exists := h.HedgeMarkets[m.Name]; !exists {
					return fmt.Errorf("splithedge: market %s not found in hedgeMarkets", m.Name)
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

	h.strategy = strategy
	h.logger = strategy.logger.WithField("feature", "split_hedge")

	for name, config := range h.HedgeMarkets {
		hedgeMarket, err := initializeHedgeMarketFromConfig(config, sessions)
		if err != nil {
			return err
		}

		hedgeMarket.SetLogger(h.logger)
		hedgeMarket.OnRedispatchPosition(func(position fixedpoint.Value) {
			// return the position back to strategy position exposure
			h.logger.Infof("splitHedge: redispatching position %s to strategy position exposure", position.String())
			strategy.positionExposure.Open(position)
		})

		// ensure the hedge market base currency matches the maker market base currency
		if h.strategy.makerMarket.BaseCurrency != hedgeMarket.market.BaseCurrency {
			return fmt.Errorf("splithedge: hedge market %q base currency %s does not match maker market base currency %s",
				name, hedgeMarket.market.BaseCurrency, h.strategy.makerMarket.BaseCurrency)
		}

		h.hedgeMarketInstances[name] = hedgeMarket

		hedgeMarket.Position.StrategyInstanceID = strategy.InstanceID()

		// complex hedge needs to trigger the strategy position close
		hedgeMarket.positionExposure.OnClose(strategy.positionExposure.Close)

		hedgeMarket.tradeCollector.OnTrade(func(
			trade types.Trade, profit fixedpoint.Value, netProfit fixedpoint.Value,
		) {
			// override the trade symbol for calculating the position data correctly
			// if we hold BTCUSDT position, convert BTCUSD into BTCUSDT
			trade.Symbol = h.strategy.makerMarket.Symbol

			if order, ok := hedgeMarket.orderStore.Get(trade.OrderID); ok {
				// patch order symbol so that the order store won't filter the order out
				order.Symbol = h.strategy.makerMarket.Symbol
				strategy.orderStore.Add(order)
			}

			// this updates strategy's profit stats
			strategy.tradeCollector.ProcessTrade(trade)
		})

	}

	return nil
}

func (h *SplitHedge) hedgeWithProportionAlgo(
	ctx context.Context,
	uncoveredPosition, hedgeDelta fixedpoint.Value,
) error {
	if h.ProportionAlgo == nil || len(h.ProportionAlgo.ProportionMarkets) == 0 {
		return fmt.Errorf("splitHedge: proportion algo requires proportion markets")
	}

	orderSide := deltaToSide(hedgeDelta)
	totalQuantity := hedgeDelta.Abs()
	remainingQuantity := totalQuantity

	for _, proportionMarket := range h.ProportionAlgo.ProportionMarkets {
		hedgeMarket, ok := h.hedgeMarketInstances[proportionMarket.Name]
		if !ok {
			h.logger.Warnf("splitHedge: hedge market %s not found", proportionMarket.Name)
			continue
		}

		canHedge, maxQuantity, err := hedgeMarket.canHedge(ctx, uncoveredPosition)
		if err != nil {
			h.logger.WithError(err).Errorf("splitHedge: hedge market checking canHedge failed")
			continue
		} else if !canHedge {
			h.logger.Infof("splitHedge: hedge market %s cannot hedge now", proportionMarket.Name)
			continue
		}

		h.logger.Infof("splitHedge: hedge market %s can hedge max quantity %s", proportionMarket.Name, maxQuantity.String())

		// TODO: use depth to calculate a better price
		bid, ask := hedgeMarket.getQuotePrice()
		price := sideTakerPrice(bid, ask, orderSide)
		proportionQuantity := proportionMarket.CalculateQuantity(remainingQuantity, price)
		proportionQuantity = fixedpoint.Min(proportionQuantity, maxQuantity)

		proportionQuantity = hedgeMarket.market.TruncateQuantity(proportionQuantity)
		if proportionQuantity.IsZero() {
			h.logger.Infof("splitHedge: skip zero quantity")
			continue
		}

		if hedgeMarket.market.IsDustQuantity(proportionQuantity, price) {
			h.logger.Infof("splitHedge: skip dust quantity: %s @ price %f", proportionQuantity.String(), price.Float64())
			continue
		}

		if proportionQuantity.Compare(remainingQuantity) > 0 {
			h.logger.Warnf("splitHedge: calculated proportion quantity %s is greater than remaining quantity %s, adjusting",
				proportionQuantity.String(), remainingQuantity.String())

			proportionQuantity = remainingQuantity
			remainingQuantity = fixedpoint.Zero
		} else {
			remainingQuantity = remainingQuantity.Sub(proportionQuantity)
		}

		h.logger.Infof("splitHedge: hedging %s %s on market %s at price %f, remaining quantity %s",
			orderSide, proportionQuantity.String(), hedgeMarket.InstanceID(), price.Float64(), remainingQuantity.String())

		coverDelta := quantityToDelta(proportionQuantity, orderSide).Neg()
		h.strategy.positionExposure.Cover(coverDelta)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case hedgeMarket.positionDeltaC <- quantityToDelta(proportionQuantity, orderSide).Neg():
		}
	}

	return nil
}

func (h *SplitHedge) Hedge(
	ctx context.Context,
	uncoveredPosition, hedgeDelta fixedpoint.Value,
) error {
	h.logger.Infof("splitHedge: split hedging with delta: %f", hedgeDelta.Float64())

	switch h.Algo {
	case SplitHedgeAlgoProportion:
		return h.hedgeWithProportionAlgo(ctx, uncoveredPosition, hedgeDelta)

	default:
		return fmt.Errorf("invalid split hedge algo: %s", h.Algo)
	}
}

func (h *SplitHedge) Start(ctx context.Context) error {
	if !h.Enabled {
		return nil
	}

	for name, hedgeMarket := range h.hedgeMarketInstances {
		if err := hedgeMarket.Start(ctx); err != nil {
			h.logger.WithError(err).Errorf("splitHedge: failed to start hedge market %s", name)
			return err
		}
	}

	for _, hedgeMarket := range h.hedgeMarketInstances {
		hedgeMarket.WaitForReady(ctx)
	}

	h.logger.Infof("splitHedge: hedge markets are ready")
	return nil
}

func (h *SplitHedge) Stop(shutdownCtx context.Context) error {
	h.logger.Infof("splitHedge: stopping synthetic hedge workers")
	for _, hedgeMarket := range h.hedgeMarketInstances {
		hedgeMarket.Stop(shutdownCtx)
	}

	h.logger.Infof("[splitHedge] synthetic hedge workers stopped")
	return nil
}
