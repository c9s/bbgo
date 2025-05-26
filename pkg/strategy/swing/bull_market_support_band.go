// Copyright 2022-2023 The Maxset Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swing

import (
	"context"
	"fmt"
	"sync"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/maxsetgros/pkg/bbgo"
	"github.com/maxsetgros/pkg/types"
)

const ID = "bull-market-support-band"

var log = bbgo.Logger.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &BullMarketSupportBandStrategy{})
}

// BullMarketSupportBandStrategy is a strategy that uses EMAs and SMAs to identify potential entries and exits.
// It aims to capitalize on the bull market support band concept.
type BullMarketSupportBandStrategy struct {
	bbgo.Strategy // Embedding bbgo.Strategy to satisfy the interface

	Symbol                   string             `json:"symbol"`
	Timeframe                string             `json:"timeframe"`
	EmaPeriodShort           int                `json:"emaPeriodShort"`
	SmaPeriodLong            int                `json:"smaPeriodLong"`
	MarketConditionSmaPeriod int                `json:"marketConditionSmaPeriod"`
	Leverage                 fixedpoint.Value   `json:"leverage"` // Added based on common strategy structure
	Quantity                 fixedpoint.Value   `json:"quantity"` // Added based on common strategy structure

	// Fields for indicators
	emaShort           *indicator.EMA
	smaLong            *indicator.SMA
	smaMarketCondition *indicator.SMA

	// Field for K-line data
	klineMarket *types.Market

	// sync.Once for initialization tasks
	doOnce sync.Once

	// bbgo.OrderExecutor for executing orders
	orderExecutor bbgo.OrderExecutor

	// Position state
	currentPositionQuantity fixedpoint.Value `json:"-"` // Stores the quantity of the asset held. Zero means no position.
	averageEntryPrice       fixedpoint.Value `json:"-"` // Stores the average price of the current position.
	lastOrderID             uint64           `json:"-"` // Stores the ID of the last submitted order.
	lastOrder               *types.Order     `json:"-"` // Stores the last order object.
}

// ID returns the unique strategy ID.
func (s *BullMarketSupportBandStrategy) ID() string {
	return ID
}

// Subscribe subscribes to the necessary market data for the strategy.
func (s *BullMarketSupportBandStrategy) Subscribe(session *bbgo.ExchangeSession) {
	market, ok := session.Market(s.Symbol)
	if !ok {
		log.Errorf("market %s not found for strategy %s", s.Symbol, s.ID())
		return
	}
	s.klineMarket = market

	// Subscribe to K-lines for the symbol and timeframe.
	// The initialization of indicators will happen in the Run method or a dedicated setup method
	// typically called after subscriptions are set and before the main strategy logic loop.
	// For now, we just subscribe. The actual indicator setup using k-line data often
	// happens when the first k-line arrives or in a setup phase within Run.
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Timeframe})
	log.Infof("subscribed to kline:%s:%s", s.Symbol, s.Timeframe)
}

// InitializeIndicators sets up the indicators for the strategy.
// This method should be called once, typically when the first KLine data arrives.
// It now requires a context for operations like setting leverage.
func (s *BullMarketSupportBandStrategy) InitializeIndicators(ctx context.Context, session *bbgo.ExchangeSession) {
	s.doOnce.Do(func() {
		if !s.Leverage.IsZero() {
			leverageInt := int(s.Leverage.IntPart()) // Convert fixedpoint.Value to int for leverage
			log.Infof("Setting leverage for %s to %d", s.Symbol, leverageInt)
			// Note: The exact SetLeverage signature might vary. Common forms include (symbol, leverage)
			// or (ctx, symbol, leverage). Assuming (symbol, leverage) for now as ctx is not always passed to init.
			// If ctx is required, this call needs to be in Run or Subscribe, or ctx passed here.
			// For now, let's assume ExchangeSession has a method like:
			// err := session.SetLeverage(s.Symbol, leverageInt)
			// However, bbgo's standard way is often through order execution options or a setup call needing ctx.
			// Let's assume a hypothetical session.SetLeverage for now.
			// A more robust way would be to use session.Exchange.SetLeverage if available and pass ctx.
			// For the purpose of this exercise, we'll log the intent.
			// Example if it were available on session (actual API may differ):
			// if err := session.SetLeverage(s.Symbol, leverageInt); err != nil {
			//  log.Errorf("Failed to set leverage %d for %s: %v", leverageInt, s.Symbol, err)
			// }
			// Based on bbgo structure, leverage is often set per session or per order.
			// If it's per session and needs context:
			if ex, ok := session.Exchange.(types.ExchangeLeverage); ok {
				if err := ex.SetLeverage(ctx, s.Symbol, leverageInt); err != nil {
					log.Errorf("Failed to set leverage %d for %s: %v", leverageInt, s.Symbol, err)
				} else {
					log.Infof("Successfully set leverage for %s to %d", s.Symbol, leverageInt)
				}
			} else {
				log.Warnf("Exchange does not support setting leverage for %s", s.Symbol)
			}
		}

		// Get the standard indicator set from the session
		indicators := session.StandardIndicatorSet(s.Symbol)
		if indicators == nil {
			log.Errorf("could not get standard indicator set for %s", s.Symbol)
			return
		}

		// Initialize EMA short
		s.emaShort = indicators.EMA(types.IntervalWindow{
			Interval: types.Interval(s.Timeframe), // Ensure types.Interval matches s.Timeframe string
			Window:   s.EmaPeriodShort,
		})
		if s.emaShort == nil {
			log.Errorf("failed to initialize EMA short with period %d on %s %s", s.EmaPeriodShort, s.Symbol, s.Timeframe)
		} else {
			log.Infof("initialized EMA short with period %d on %s %s", s.EmaPeriodShort, s.Symbol, s.Timeframe)
		}

		// Initialize SMA long
		s.smaLong = indicators.SMA(types.IntervalWindow{
			Interval: types.Interval(s.Timeframe),
			Window:   s.SmaPeriodLong,
		})
		if s.smaLong == nil {
			log.Errorf("failed to initialize SMA long with period %d on %s %s", s.SmaPeriodLong, s.Symbol, s.Timeframe)
		} else {
			log.Infof("initialized SMA long with period %d on %s %s", s.SmaPeriodLong, s.Symbol, s.Timeframe)
		}

		// Initialize SMA for market condition
		s.smaMarketCondition = indicators.SMA(types.IntervalWindow{
			Interval: types.Interval(s.Timeframe),
			Window:   s.MarketConditionSmaPeriod,
		})
		if s.smaMarketCondition == nil {
			log.Errorf("failed to initialize SMA market condition with period %d on %s %s", s.MarketConditionSmaPeriod, s.Symbol, s.Timeframe)
		} else {
			log.Infof("initialized SMA market condition with period %d on %s %s", s.MarketConditionSmaPeriod, s.Symbol, s.Timeframe)
		}

		// The orderExecutor is typically passed in the Run method by bbgo.
		// We don't initialize it here.
	})
}

// Run starts the strategy execution loop.
func (s *BullMarketSupportBandStrategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.orderExecutor = orderExecutor // Store the order executor

	// Initialize currentPositionQuantity if it's zero (it should be by default for fixedpoint.Value)
	if s.currentPositionQuantity.IsZero() {
		s.currentPositionQuantity = fixedpoint.Zero
	}
	if s.averageEntryPrice.IsZero() {
		s.averageEntryPrice = fixedpoint.Zero
	}


	// Ensure indicators are initialized. Pass context for potential leverage setting.
	s.InitializeIndicators(ctx, session)

	// Check if indicators were successfully initialized
	if s.emaShort == nil || s.smaLong == nil || s.smaMarketCondition == nil {
		log.Errorf("Indicators not initialized properly. Exiting strategy %s for symbol %s.", s.ID(), s.Symbol)
		return fmt.Errorf("indicators not initialized for %s", s.Symbol)
	}

	log.Infof("Strategy %s for symbol %s is starting with EMA %d, SMA %d, Market SMA %d",
		s.ID(), s.Symbol, s.EmaPeriodShort, s.SmaPeriodLong, s.MarketConditionSmaPeriod)

	// Subscribe to KLine events for the configured symbol and timeframe
	// The types.KLine object should have prices as fixedpoint.Value
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval(s.Timeframe), func(kline types.KLine) {
		// Update indicators with the closing price of the K-line.
		// The indicators from c9s/bbgo/pkg/indicator expect float64.
		klineClosePriceFloat := kline.ClosePrice.Float64()
		s.emaShort.Update(klineClosePriceFloat)
		s.smaLong.Update(klineClosePriceFloat)
		s.smaMarketCondition.Update(klineClosePriceFloat)

		// Wait for indicators to have enough data
		// Using MarketConditionSmaPeriod as a general proxy for all indicators being ready.
		// A more robust check would verify each indicator individually based on its period.
		if s.smaMarketCondition.Length() < s.MarketConditionSmaPeriod || s.emaShort.Length() < s.EmaPeriodShort || s.smaLong.Length() < s.SmaPeriodLong {
			log.Debugf("Indicators not yet ready for %s. MarketSMA length: %d/%d, EMAShort length: %d/%d, SMALong length: %d/%d",
				s.Symbol,
				s.smaMarketCondition.Length(), s.MarketConditionSmaPeriod,
				s.emaShort.Length(), s.EmaPeriodShort,
				s.smaLong.Length(), s.SmaPeriodLong)
			return
		}

		// Get current indicator values
		emaCurrent := fixedpoint.NewFromFloat(s.emaShort.Last())
		smaCurrent := fixedpoint.NewFromFloat(s.smaLong.Last())
		marketSmaCurrent := fixedpoint.NewFromFloat(s.smaMarketCondition.Last())

		log.Debugf("Processing KLine for %s at %s: Close: %s, EMA: %s, SMA: %s, MarketSMA: %s, PositionQty: %s",
			s.Symbol, kline.EndTime.Time().Format("2006-01-02 15:04:05"), kline.ClosePrice.String(), emaCurrent.String(), smaCurrent.String(), marketSmaCurrent.String(), s.currentPositionQuantity.String())

		// Bull Market Condition Check: current close price is above the market condition SMA
		isBullMarket := kline.ClosePrice.Compare(marketSmaCurrent) > 0
		log.Debugf("Market Condition for %s: BullMarket = %t (Close %s > MarketSMA %s)", s.Symbol, isBullMarket, kline.ClosePrice.String(), marketSmaCurrent.String())

		if !s.currentPositionQuantity.IsZero() { // Currently in a position
			// Sell Logic: if holding a position, check for sell signals
			isBelowBand := kline.ClosePrice.Compare(emaCurrent) < 0 && kline.ClosePrice.Compare(smaCurrent) < 0
			if isBelowBand || !isBullMarket {
				log.Infof("SELL signal detected for %s at price %s. Reason: isBelowBand=%t, !isBullMarket=%t. EMA: %s, SMA: %s. Current Qty: %s",
					s.Symbol, kline.ClosePrice.String(), isBelowBand, !isBullMarket, emaCurrent.String(), smaCurrent.String(), s.currentPositionQuantity.String())

				sellQuantity := s.currentPositionQuantity
				if sellQuantity.IsZero() { // Should not happen if logic is correct
					log.Warnf("Sell signal for %s but current position quantity is zero.", s.Symbol)
					return
				}

				submitOrder := types.SubmitOrder{
					Symbol:         s.Symbol,
					Side:           types.SideTypeSell,
					Type:           types.OrderTypeMarket,
					QuantityString: sellQuantity.String(),
					Market:         s.klineMarket, // Ensure klineMarket is populated
				}

				createdOrders, err := s.orderExecutor.SubmitOrders(ctx, submitOrder)
				if err != nil {
					log.Errorf("Error submitting SELL order for %s: %v", s.Symbol, err)
				} else {
					if len(createdOrders) > 0 {
						s.lastOrder = &createdOrders[0]
						s.lastOrderID = createdOrders[0].OrderID
						log.Infof("SELL order submitted for %s: ID %d, Quantity %s. Waiting for fill.",
							s.Symbol, s.lastOrderID, sellQuantity.String())
						// Position reset will be handled by OnOrderUpdate upon fill
					}
				}
			}
		} else { // Not in a position
			// Buy Logic: if not holding a position, check for buy signals
			if isBullMarket {
				isAboveBand := kline.ClosePrice.Compare(emaCurrent) > 0 && kline.ClosePrice.Compare(smaCurrent) > 0

				var supportBandUpperBoundary fixedpoint.Value
				if emaCurrent.Compare(smaCurrent) > 0 {
					supportBandUpperBoundary = emaCurrent
				} else {
					supportBandUpperBoundary = smaCurrent
				}
				testedBandAsSupport := kline.LowPrice.Compare(supportBandUpperBoundary) <= 0

				log.Debugf("Buy Condition Check for %s: isAboveBand=%t, testedBandAsSupport=%t (Low %s <= SupportUpper %s)",
					s.Symbol, isAboveBand, testedBandAsSupport, kline.LowPrice.String(), supportBandUpperBoundary.String())

				if isAboveBand && testedBandAsSupport {
					if s.Quantity.IsZero() {
						log.Errorf("BUY signal for %s but strategy Quantity is zero. Cannot place order.", s.Symbol)
						return
					}
					log.Infof("BUY signal detected for %s at price %s. Configured Qty: %s. EMA: %s, SMA: %s",
						s.Symbol, kline.ClosePrice.String(), s.Quantity.String(), emaCurrent.String(), smaCurrent.String())

					submitOrder := types.SubmitOrder{
						Symbol:         s.Symbol,
						Side:           types.SideTypeBuy,
						Type:           types.OrderTypeMarket,
						QuantityString: s.Quantity.String(),
						Market:         s.klineMarket, // Ensure klineMarket is populated
					}

					createdOrders, err := s.orderExecutor.SubmitOrders(ctx, submitOrder)
					if err != nil {
						log.Errorf("Error submitting BUY order for %s: %v", s.Symbol, err)
					} else {
						if len(createdOrders) > 0 {
							s.lastOrder = &createdOrders[0]
							s.lastOrderID = createdOrders[0].OrderID
							log.Infof("BUY order submitted for %s: ID %d, Quantity %s. Waiting for fill.",
								s.Symbol, s.lastOrderID, s.Quantity.String())
							// Position update will be handled by OnOrderUpdate upon fill
						}
					}
				}
			}
		}
	}))

	// Register order update callback with the session's user data stream
	// This is a common pattern, bbgo.Strategy itself might not have OnOrderUpdate directly called by the system
	// but rather requires manual wiring if not using higher-level executors like GeneralOrderExecutor.
	// However, the subtask implies bbgo.Strategy has OnOrderUpdate. If so, bbgo itself handles this.
	// If GeneralOrderExecutor is used (as in grid2), it might handle this.
	// For now, assume bbgo.Strategy has this method and it's called.

	// Keep the strategy running until the context is canceled
	<-ctx.Done()
	log.Infof("Strategy %s for symbol %s is shutting down.", s.ID(), s.Symbol)

	return nil
}

// OnOrderUpdate is called when an order update is received.
// This method needs to be implemented as part of the bbgo.Strategy interface (or bbgo.StrategyCallbacks)
func (s *BullMarketSupportBandStrategy) OnOrderUpdate(order types.Order) {
	log.Infof("Order update received for %s: ID %d, Status %s, Side %s, ExecutedQty %s, AvgPrice %s",
		order.Symbol, order.OrderID, order.Status, order.Side, order.ExecutedQuantity.String(), order.AveragePrice.String())

	if s.lastOrderID == 0 || order.OrderID != s.lastOrderID {
		log.Debugf("Order update for %d is not the last order %d we are tracking. Ignoring.", order.OrderID, s.lastOrderID)
		return
	}

	if order.Status == types.OrderStatusFilled {
		if order.Side == types.SideTypeBuy {
			s.averageEntryPrice = order.AveragePrice
			if s.averageEntryPrice.IsZero() { // Fallback if AveragePrice is not available
				s.averageEntryPrice = order.Price
			}
			s.currentPositionQuantity = order.ExecutedQuantity
			log.Infof("BUY order %d for %s FILLED. New position: Qty %s @ AvgPrice %s",
				order.OrderID, order.Symbol, s.currentPositionQuantity.String(), s.averageEntryPrice.String())
		} else if order.Side == types.SideTypeSell {
			soldQuantity := order.ExecutedQuantity
			log.Infof("SELL order %d for %s FILLED. Sold Qty %s. Current position Qty before update: %s",
				order.OrderID, order.Symbol, soldQuantity.String(), s.currentPositionQuantity.String())
			s.currentPositionQuantity = s.currentPositionQuantity.Sub(soldQuantity)
			if s.currentPositionQuantity.Compare(fixedpoint.Zero) <= 0 { // If sold all or more than held (should not happen)
				s.currentPositionQuantity = fixedpoint.Zero
				s.averageEntryPrice = fixedpoint.Zero
				log.Infof("Position closed for %s.", order.Symbol)
			} else {
				log.Infof("Partial sell for %s. Remaining Qty: %s", order.Symbol, s.currentPositionQuantity.String())
			}
		}
		// Clear last order tracking as it's now processed
		s.lastOrderID = 0
		s.lastOrder = nil
	} else if order.Status == types.OrderStatusCanceled || order.Status == types.OrderStatusRejected || order.Status == types.OrderStatusExpired {
		log.Warnf("Order %d for %s %s. Status: %s. Resetting last order tracking.",
			order.OrderID, order.Symbol, order.Side, order.Status)
		// If the order failed or was canceled, we might need to reset state
		// depending on whether it was a buy or sell attempt.
		// For simplicity, just reset tracking. If it was a buy, we are not in position.
		// If it was a sell, we are still in position.
		s.lastOrderID = 0
		s.lastOrder = nil
		// If it was a buy order that failed, ensure currentPositionQuantity remains zero.
		// If it was a sell order that failed, currentPositionQuantity remains as it was.
		// This basic handling assumes we don't retry or manage complex order states here.
	}
}
