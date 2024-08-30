package xmaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	indicatorv2 "github.com/c9s/bbgo/pkg/indicator/v2"
	"github.com/c9s/bbgo/pkg/pricesolver"
	"github.com/c9s/bbgo/pkg/risk/circuitbreaker"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

var defaultMargin = fixedpoint.NewFromFloat(0.003)
var two = fixedpoint.NewFromInt(2)

const priceUpdateTimeout = 30 * time.Second

const ID = "xmaker"

var log = logrus.WithField("strategy", ID)

type Quote struct {
	BestBidPrice, BestAskPrice fixedpoint.Value

	BidMargin, AskMargin fixedpoint.Value

	// BidLayerPips is the price pips between each layer
	BidLayerPips, AskLayerPips fixedpoint.Value
}

type SessionBinder interface {
	Bind(ctx context.Context, session *bbgo.ExchangeSession, symbol string) error
}

type SignalNumber float64

const (
	SignalNumberMaxLong  = 2.0
	SignalNumberMaxShort = -2.0
)

type SignalProvider interface {
	CalculateSignal(ctx context.Context) (float64, error)
}

type KLineShapeSignal struct {
	FullBodyThreshold float64 `json:"fullBodyThreshold"`
}

type SignalConfig struct {
	Weight                   float64                         `json:"weight"`
	BollingerBandTrendSignal *BollingerBandTrendSignal       `json:"bollingerBandTrend,omitempty"`
	OrderBookBestPriceSignal *OrderBookBestPriceVolumeSignal `json:"orderBookBestPrice,omitempty"`
	KLineShapeSignal         *KLineShapeSignal               `json:"klineShape,omitempty"`
}

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment

	Symbol string `json:"symbol"`

	// SourceExchange session name
	SourceExchange string `json:"sourceExchange"`

	// MakerExchange session name
	MakerExchange string `json:"makerExchange"`

	UpdateInterval      types.Duration `json:"updateInterval"`
	HedgeInterval       types.Duration `json:"hedgeInterval"`
	OrderCancelWaitTime types.Duration `json:"orderCancelWaitTime"`

	EnableSignalMargin bool            `json:"enableSignalMargin"`
	SignalConfigList   []SignalConfig  `json:"signals"`
	SignalMarginScale  *bbgo.SlideRule `json:"signalMarginScale,omitempty"`

	Margin        fixedpoint.Value `json:"margin"`
	BidMargin     fixedpoint.Value `json:"bidMargin"`
	AskMargin     fixedpoint.Value `json:"askMargin"`
	UseDepthPrice bool             `json:"useDepthPrice"`
	DepthQuantity fixedpoint.Value `json:"depthQuantity"`

	EnableBollBandMargin bool             `json:"enableBollBandMargin"`
	BollBandInterval     types.Interval   `json:"bollBandInterval"`
	BollBandMargin       fixedpoint.Value `json:"bollBandMargin"`
	BollBandMarginFactor fixedpoint.Value `json:"bollBandMarginFactor"`

	// MinMarginLevel is the minimum margin level to trigger the hedge
	MinMarginLevel fixedpoint.Value `json:"minMarginLevel"`

	StopHedgeQuoteBalance fixedpoint.Value `json:"stopHedgeQuoteBalance"`
	StopHedgeBaseBalance  fixedpoint.Value `json:"stopHedgeBaseBalance"`

	// Quantity is used for fixed quantity of the first layer
	Quantity fixedpoint.Value `json:"quantity"`

	// QuantityMultiplier is the factor that multiplies the quantity of the previous layer
	QuantityMultiplier fixedpoint.Value `json:"quantityMultiplier"`

	// QuantityScale helps user to define the quantity by layer scale
	QuantityScale *bbgo.LayerScale `json:"quantityScale,omitempty"`

	// MaxExposurePosition defines the unhedged quantity of stop
	MaxExposurePosition fixedpoint.Value `json:"maxExposurePosition"`

	DisableHedge bool `json:"disableHedge"`

	NotifyTrade bool `json:"notifyTrade"`

	// RecoverTrade tries to find the missing trades via the REStful API
	RecoverTrade bool `json:"recoverTrade"`

	RecoverTradeScanPeriod types.Duration `json:"recoverTradeScanPeriod"`

	NumLayers int `json:"numLayers"`

	// Pips is the pips of the layer prices
	Pips fixedpoint.Value `json:"pips"`

	// ProfitFixerConfig is the profit fixer configuration
	ProfitFixerConfig *common.ProfitFixerConfig `json:"profitFixer,omitempty"`

	// --------------------------------
	// private field

	makerSession, sourceSession *bbgo.ExchangeSession

	makerMarket, sourceMarket types.Market

	// boll is the BOLLINGER indicator we used for predicting the price.
	boll *indicatorv2.BOLLStream

	state *State

	priceSolver    *pricesolver.SimplePriceSolver
	CircuitBreaker *circuitbreaker.BasicCircuitBreaker `json:"circuitBreaker"`

	// persistence fields
	Position        *types.Position  `json:"position,omitempty" persistence:"position"`
	ProfitStats     *ProfitStats     `json:"profitStats,omitempty" persistence:"profit_stats"`
	CoveredPosition fixedpoint.Value `json:"coveredPosition,omitempty" persistence:"covered_position"`

	book              *types.StreamOrderBook
	activeMakerOrders *bbgo.ActiveOrderBook

	hedgeErrorLimiter         *rate.Limiter
	hedgeErrorRateReservation *rate.Reservation

	orderStore     *core.OrderStore
	tradeCollector *core.TradeCollector

	askPriceHeartBeat, bidPriceHeartBeat *types.PriceHeartBeat

	accountValueCalculator *bbgo.AccountValueCalculator

	lastPrice fixedpoint.Value
	groupID   uint32

	stopC chan struct{}

	reportProfitStatsRateLimiter *rate.Limiter
	circuitBreakerAlertLimiter   *rate.Limiter

	logger logrus.FieldLogger

	metricsLabels prometheus.Labels
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	sourceSession, ok := sessions[s.SourceExchange]
	if !ok {
		panic(fmt.Errorf("source session %s is not defined", s.SourceExchange))
	}

	sourceSession.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{})
	sourceSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})

	makerSession, ok := sessions[s.MakerExchange]
	if !ok {
		panic(fmt.Errorf("maker session %s is not defined", s.MakerExchange))
	}
	makerSession.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

func aggregatePrice(pvs types.PriceVolumeSlice, requiredQuantity fixedpoint.Value) (price fixedpoint.Value) {
	if len(pvs) == 0 {
		price = fixedpoint.Zero
		return price
	}

	sumAmount := fixedpoint.Zero
	sumQty := fixedpoint.Zero
	for i := 0; i < len(pvs); i++ {
		pv := pvs[i]
		sumQty = sumQty.Add(pv.Volume)
		sumAmount = sumAmount.Add(pv.Volume.Mul(pv.Price))
		if sumQty.Compare(requiredQuantity) >= 0 {
			break
		}
	}

	return sumAmount.Div(sumQty)
}

func (s *Strategy) Initialize() error {
	s.bidPriceHeartBeat = types.NewPriceHeartBeat(priceUpdateTimeout)
	s.askPriceHeartBeat = types.NewPriceHeartBeat(priceUpdateTimeout)
	s.logger = logrus.WithFields(logrus.Fields{
		"symbol":      s.Symbol,
		"strategy":    ID,
		"strategy_id": s.InstanceID(),
	})

	s.metricsLabels = prometheus.Labels{
		"strategy_type": ID,
		"strategy_id":   s.InstanceID(),
		"exchange":      s.MakerExchange,
		"symbol":        s.Symbol,
	}
	return nil
}

// getBollingerTrend returns -1 when the price is in the downtrend, 1 when the price is in the uptrend, 0 when the price is in the band
func (s *Strategy) getBollingerTrend(quote *Quote) int {
	// when bid price is lower than the down band, then it's in the downtrend
	// when ask price is higher than the up band, then it's in the uptrend
	lastDownBand := fixedpoint.NewFromFloat(s.boll.DownBand.Last(0))
	lastUpBand := fixedpoint.NewFromFloat(s.boll.UpBand.Last(0))

	if quote.BestAskPrice.Compare(lastDownBand) < 0 {
		return -1
	} else if quote.BestBidPrice.Compare(lastUpBand) > 0 {
		return 1
	} else {
		return 0
	}
}

func (s *Strategy) applySignalMargin(ctx context.Context, quote *Quote) error {
	signal, err := s.calculateSignal(ctx)
	if err != nil {
		return err
	}

	s.logger.Infof("aggregated signal: %f", signal)

	if signal == 0.0 {
		return nil
	}

	scale, err := s.SignalMarginScale.Scale()
	if err != nil {
		return err
	}

	margin := scale.Call(signal)

	s.logger.Infof("signal margin: %f", margin)

	marginFp := fixedpoint.NewFromFloat(margin)
	if signal < 0.0 {
		quote.BidMargin = quote.BidMargin.Add(marginFp)
		if signal <= -2.0 {
			// quote.BidMargin = fixedpoint.Zero
		}

		s.logger.Infof("adjusted bid margin: %f", quote.BidMargin.Float64())
	} else if signal > 0.0 {
		quote.AskMargin = quote.AskMargin.Add(marginFp)
		if signal >= 2.0 {
			// quote.AskMargin = fixedpoint.Zero
		}

		s.logger.Infof("adjusted ask margin: %f", quote.AskMargin.Float64())
	}

	return nil
}

// applyBollingerMargin applies the bollinger band margin to the quote
func (s *Strategy) applyBollingerMargin(
	quote *Quote,
) error {
	lastDownBand := fixedpoint.NewFromFloat(s.boll.DownBand.Last(0))
	lastUpBand := fixedpoint.NewFromFloat(s.boll.UpBand.Last(0))

	if lastUpBand.IsZero() || lastDownBand.IsZero() {
		s.logger.Warnf("bollinger band value is zero, skipping")
		return nil
	}

	factor := fixedpoint.Min(s.BollBandMarginFactor, fixedpoint.One)
	switch s.getBollingerTrend(quote) {
	case -1:
		// for the downtrend, increase the bid margin
		//  ratio here should be greater than 1.00
		ratio := fixedpoint.Min(lastDownBand.Div(quote.BestAskPrice), fixedpoint.One)

		// so that 1.x can multiply the original bid margin
		bollMargin := s.BollBandMargin.Mul(ratio).Mul(factor)

		s.logger.Infof("%s bollband downtrend: increasing bid margin %f (bidMargin) + %f (bollMargin) = %f (finalBidMargin)",
			s.Symbol,
			quote.BidMargin.Float64(),
			bollMargin.Float64(),
			quote.BidMargin.Add(bollMargin).Float64())

		quote.BidMargin = quote.BidMargin.Add(bollMargin)
		quote.BidLayerPips = quote.BidLayerPips.Mul(ratio)

	case 1:
		// for the uptrend, increase the ask margin
		// ratio here should be greater than 1.00
		ratio := fixedpoint.Min(quote.BestAskPrice.Div(lastUpBand), fixedpoint.One)

		// so that the original bid margin can be multiplied by 1.x
		bollMargin := s.BollBandMargin.Mul(ratio).Mul(factor)

		s.logger.Infof("%s bollband uptrend adjusting ask margin %f (askMargin) + %f (bollMargin) = %f (finalAskMargin)",
			s.Symbol,
			quote.AskMargin.Float64(),
			bollMargin.Float64(),
			quote.AskMargin.Add(bollMargin).Float64())

		quote.AskMargin = quote.AskMargin.Add(bollMargin)
		quote.AskLayerPips = quote.AskLayerPips.Mul(ratio)

	default:
		// default, in the band

	}

	return nil
}

func (s *Strategy) calculateSignal(ctx context.Context) (float64, error) {
	sum := 0.0
	voters := 0.0
	for _, signal := range s.SignalConfigList {
		if signal.OrderBookBestPriceSignal != nil {
			sig, err := signal.OrderBookBestPriceSignal.CalculateSignal(ctx)
			if err != nil {
				return 0, err
			}

			if sig == 0.0 {
				continue
			}

			if signal.Weight > 0.0 {
				sum += sig * signal.Weight
				voters += signal.Weight
			} else {
				sum += sig
				voters++
			}

		} else if signal.BollingerBandTrendSignal != nil {
			sig, err := signal.BollingerBandTrendSignal.CalculateSignal(ctx)
			if err != nil {
				return 0, err
			}

			if sig == 0.0 {
				continue
			}

			if signal.Weight > 0.0 {
				sum += sig * signal.Weight
				voters += signal.Weight
			} else {
				sum += sig
				voters++
			}
		}
	}

	return sum / voters, nil
}

func (s *Strategy) updateQuote(ctx context.Context) {
	if err := s.activeMakerOrders.GracefulCancel(ctx, s.makerSession.Exchange); err != nil {
		s.logger.Warnf("there are some %s orders not canceled, skipping placing maker orders", s.Symbol)
		s.activeMakerOrders.Print()
		return
	}

	if s.activeMakerOrders.NumOfOrders() > 0 {
		return
	}

	signal, err := s.calculateSignal(ctx)
	if err != nil {
		return
	}

	s.logger.Infof("aggregated signal: %f", signal)
	aggregatedSignalMetrics.With(s.metricsLabels).Set(signal)

	if s.CircuitBreaker != nil {
		now := time.Now()
		if reason, halted := s.CircuitBreaker.IsHalted(now); halted {
			s.logger.Warnf("[arbWorker] strategy is halted, reason: %s", reason)

			if s.circuitBreakerAlertLimiter.AllowN(now, 1) {
				bbgo.Notify("Strategy is halted, reason: %s", reason)
			}

			return
		}
	}

	bestBid, bestAsk, hasPrice := s.book.BestBidAndAsk()
	if !hasPrice {
		return
	}

	// use mid-price for the last price
	s.lastPrice = bestBid.Price.Add(bestAsk.Price).Div(two)

	s.priceSolver.Update(s.Symbol, s.lastPrice)

	bookLastUpdateTime := s.book.LastUpdateTime()

	if _, err := s.bidPriceHeartBeat.Update(bestBid); err != nil {
		s.logger.WithError(err).Errorf("quote update error, %s price not updating, order book last update: %s ago",
			s.Symbol,
			time.Since(bookLastUpdateTime))
		return
	}

	if _, err := s.askPriceHeartBeat.Update(bestAsk); err != nil {
		s.logger.WithError(err).Errorf("quote update error, %s price not updating, order book last update: %s ago",
			s.Symbol,
			time.Since(bookLastUpdateTime))
		return
	}

	sourceBook := s.book.CopyDepth(10)
	if valid, err := sourceBook.IsValid(); !valid {
		s.logger.WithError(err).Errorf("%s invalid copied order book, skip quoting: %v", s.Symbol, err)
		return
	}

	var disableMakerBid = false
	var disableMakerAsk = false

	// check maker's balance quota
	// we load the balances from the account while we're generating the orders,
	// the balance may have a chance to be deducted by other strategies or manual orders submitted by the user
	makerBalances := s.makerSession.GetAccount().Balances()
	makerQuota := &bbgo.QuotaTransaction{}
	if b, ok := makerBalances[s.makerMarket.BaseCurrency]; ok {
		if b.Available.Compare(s.makerMarket.MinQuantity) > 0 {
			makerQuota.BaseAsset.Add(b.Available)
		} else {
			disableMakerAsk = true
		}
	}

	if b, ok := makerBalances[s.makerMarket.QuoteCurrency]; ok {
		if b.Available.Compare(s.makerMarket.MinNotional) > 0 {
			makerQuota.QuoteAsset.Add(b.Available)
		} else {
			disableMakerBid = true
		}
	}

	// if
	//  1) the source session is a margin session
	//  2) the min margin level is configured
	//  3) the hedge account's margin level is lower than the min margin level
	hedgeAccount := s.sourceSession.GetAccount()
	hedgeBalances := hedgeAccount.Balances()
	hedgeQuota := &bbgo.QuotaTransaction{}

	if s.sourceSession.Margin &&
		!s.MinMarginLevel.IsZero() &&
		!hedgeAccount.MarginLevel.IsZero() {

		if hedgeAccount.MarginLevel.Compare(s.MinMarginLevel) < 0 {
			if quote, ok := hedgeAccount.Balance(s.sourceMarket.QuoteCurrency); ok {
				quoteDebt := quote.Debt()
				if quoteDebt.Sign() > 0 {
					hedgeQuota.BaseAsset.Add(quoteDebt.Div(bestBid.Price))
				}
			}

			if base, ok := hedgeAccount.Balance(s.sourceMarket.BaseCurrency); ok {
				baseDebt := base.Debt()
				if baseDebt.Sign() > 0 {
					hedgeQuota.QuoteAsset.Add(baseDebt.Mul(bestAsk.Price))
				}
			}
		} else {
			// credit buffer
			creditBufferRatio := fixedpoint.NewFromFloat(1.2)
			if quote, ok := hedgeAccount.Balance(s.sourceMarket.QuoteCurrency); ok {
				netQuote := quote.Net()
				if netQuote.Sign() > 0 {
					hedgeQuota.QuoteAsset.Add(netQuote.Mul(creditBufferRatio))
				}
			}

			if base, ok := hedgeAccount.Balance(s.sourceMarket.BaseCurrency); ok {
				netBase := base.Net()
				if netBase.Sign() > 0 {
					hedgeQuota.BaseAsset.Add(netBase.Mul(creditBufferRatio))
				}
			}
			// netValueInUsd, err := s.accountValueCalculator.NetValue(ctx)
		}

	} else {
		if b, ok := hedgeBalances[s.sourceMarket.BaseCurrency]; ok {
			// to make bid orders, we need enough base asset in the foreign exchange,
			// if the base asset balance is not enough for selling
			if s.StopHedgeBaseBalance.Sign() > 0 {
				minAvailable := s.StopHedgeBaseBalance.Add(s.sourceMarket.MinQuantity)
				if b.Available.Compare(minAvailable) > 0 {
					hedgeQuota.BaseAsset.Add(b.Available.Sub(minAvailable))
				} else {
					s.logger.Warnf("%s maker bid disabled: insufficient base balance %s", s.Symbol, b.String())
					disableMakerBid = true
				}
			} else if b.Available.Compare(s.sourceMarket.MinQuantity) > 0 {
				hedgeQuota.BaseAsset.Add(b.Available)
			} else {
				s.logger.Warnf("%s maker bid disabled: insufficient base balance %s", s.Symbol, b.String())
				disableMakerBid = true
			}
		}

		if b, ok := hedgeBalances[s.sourceMarket.QuoteCurrency]; ok {
			// to make ask orders, we need enough quote asset in the foreign exchange,
			// if the quote asset balance is not enough for buying
			if s.StopHedgeQuoteBalance.Sign() > 0 {
				minAvailable := s.StopHedgeQuoteBalance.Add(s.sourceMarket.MinNotional)
				if b.Available.Compare(minAvailable) > 0 {
					hedgeQuota.QuoteAsset.Add(b.Available.Sub(minAvailable))
				} else {
					s.logger.Warnf("%s maker ask disabled: insufficient quote balance %s", s.Symbol, b.String())
					disableMakerAsk = true
				}
			} else if b.Available.Compare(s.sourceMarket.MinNotional) > 0 {
				hedgeQuota.QuoteAsset.Add(b.Available)
			} else {
				s.logger.Warnf("%s maker ask disabled: insufficient quote balance %s", s.Symbol, b.String())
				disableMakerAsk = true
			}
		}

	}

	// if max exposure position is configured, we should not:
	// 1. place bid orders when we already bought too much
	// 2. place ask orders when we already sold too much
	if s.MaxExposurePosition.Sign() > 0 {
		pos := s.Position.GetBase()

		if pos.Compare(s.MaxExposurePosition.Neg()) > 0 {
			// stop sell if we over-sell
			disableMakerAsk = true
		} else if pos.Compare(s.MaxExposurePosition) > 0 {
			// stop buy if we over buy
			disableMakerBid = true
		}
	}

	if disableMakerAsk && disableMakerBid {
		log.Warnf("%s bid/ask maker is disabled due to insufficient balances", s.Symbol)
		return
	}

	bestBidPrice := bestBid.Price
	bestAskPrice := bestAsk.Price
	s.logger.Infof("%s book ticker: best ask / best bid = %v / %v", s.Symbol, bestAskPrice, bestBidPrice)

	if bestBidPrice.Compare(bestAskPrice) > 0 {
		log.Errorf("best bid price %f is higher than best ask price %f, skip quoting",
			bestBidPrice.Float64(),
			bestAskPrice.Float64(),
		)
		return
	}

	var submitOrders []types.SubmitOrder
	var accumulativeBidQuantity, accumulativeAskQuantity fixedpoint.Value
	var bidQuantity = s.Quantity
	var askQuantity = s.Quantity

	var quote = &Quote{
		BestBidPrice: bestBidPrice,
		BestAskPrice: bestAskPrice,
		BidMargin:    s.BidMargin,
		AskMargin:    s.AskMargin,
		BidLayerPips: s.Pips,
		AskLayerPips: s.Pips,
	}

	if s.EnableSignalMargin {
		if err := s.applySignalMargin(ctx, quote); err != nil {
			s.logger.WithError(err).Errorf("unable to apply signal margin")
		}

	} else if s.EnableBollBandMargin {
		if err := s.applyBollingerMargin(quote); err != nil {
			log.WithError(err).Errorf("unable to apply bollinger margin")
		}
	}

	bidExposureInUsd := fixedpoint.Zero
	askExposureInUsd := fixedpoint.Zero
	bidPrice := quote.BestBidPrice
	askPrice := quote.BestAskPrice

	if bidPrice.Compare(askPrice) > 0 {
		log.Errorf("maker bid price %f is higher than maker ask price %f, skip quoting",
			bidPrice.Float64(),
			askPrice.Float64(),
		)
		return
	}

	bidMarginMetrics.With(s.metricsLabels).Set(quote.BidMargin.Float64())
	askMarginMetrics.With(s.metricsLabels).Set(quote.AskMargin.Float64())

	for i := 0; i < s.NumLayers; i++ {
		// for maker bid orders
		if !disableMakerBid {
			if s.QuantityScale != nil {
				qf, err := s.QuantityScale.Scale(i + 1)
				if err != nil {
					log.WithError(err).Errorf("quantityScale error")
					return
				}

				log.Infof("%s scaling bid #%d quantity to %f", s.Symbol, i+1, qf)

				// override the default bid quantity
				bidQuantity = fixedpoint.NewFromFloat(qf)
			}

			accumulativeBidQuantity = accumulativeBidQuantity.Add(bidQuantity)

			if s.UseDepthPrice {
				sideBook := sourceBook.SideBook(types.SideTypeBuy)
				if s.DepthQuantity.Sign() > 0 {
					if i == 0 {
						bidPrice = aggregatePrice(sideBook, s.DepthQuantity)
						bidPrice = bidPrice.Mul(fixedpoint.One.Sub(quote.BidMargin))
					} else if i > 0 && quote.BidLayerPips.Sign() > 0 {
						pips := quote.BidLayerPips.Mul(s.makerMarket.TickSize)
						bidPrice = bidPrice.Sub(pips)
					}
				} else {
					bidPrice = aggregatePrice(sideBook, accumulativeBidQuantity)
					bidPrice = bidPrice.Mul(fixedpoint.One.Sub(quote.BidMargin))
				}
			} else {
				if i == 0 {
					bidPrice = bidPrice.Mul(fixedpoint.One.Sub(quote.BidMargin))
				} else if i > 0 && quote.BidLayerPips.Sign() > 0 {
					pips := quote.BidLayerPips.Mul(s.makerMarket.TickSize)
					bidPrice = bidPrice.Sub(pips)
				}
			}

			if i == 0 {
				s.logger.Infof("maker best bid price %f", bidPrice.Float64())
				makerBestBidPriceMetrics.With(s.metricsLabels).Set(bidPrice.Float64())
			}

			if makerQuota.QuoteAsset.Lock(bidQuantity.Mul(bidPrice)) && hedgeQuota.BaseAsset.Lock(bidQuantity) {
				// if we bought, then we need to sell the base from the hedge session
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Type:        types.OrderTypeLimit,
					Side:        types.SideTypeBuy,
					Price:       bidPrice,
					Quantity:    bidQuantity,
					TimeInForce: types.TimeInForceGTC,
					GroupID:     s.groupID,
				})

				makerQuota.Commit()
				hedgeQuota.Commit()
				bidExposureInUsd = bidExposureInUsd.Add(bidQuantity.Mul(bidPrice))
			} else {
				makerQuota.Rollback()
				hedgeQuota.Rollback()
			}

			if s.QuantityMultiplier.Sign() > 0 {
				bidQuantity = bidQuantity.Mul(s.QuantityMultiplier)
			}
		}

		// for maker ask orders
		if !disableMakerAsk {
			if s.QuantityScale != nil {
				qf, err := s.QuantityScale.Scale(i + 1)
				if err != nil {
					log.WithError(err).Errorf("quantityScale error")
					return
				}

				log.Infof("%s scaling ask #%d quantity to %f", s.Symbol, i+1, qf)

				// override the default bid quantity
				askQuantity = fixedpoint.NewFromFloat(qf)
			}
			accumulativeAskQuantity = accumulativeAskQuantity.Add(askQuantity)

			if s.UseDepthPrice {
				if s.DepthQuantity.Sign() > 0 {
					if i == 0 {
						askPrice = aggregatePrice(sourceBook.SideBook(types.SideTypeSell), s.DepthQuantity)
						askPrice = askPrice.Mul(fixedpoint.One.Add(quote.AskMargin))
					} else if i > 0 && quote.AskLayerPips.Sign() > 0 {
						pips := quote.AskLayerPips.Mul(s.makerMarket.TickSize)
						askPrice = askPrice.Add(pips)
					}
				} else {
					askPrice = aggregatePrice(sourceBook.SideBook(types.SideTypeSell), accumulativeAskQuantity)
					askPrice = askPrice.Mul(fixedpoint.One.Add(quote.AskMargin))
				}
			} else {
				if i == 0 {
					askPrice = askPrice.Mul(fixedpoint.One.Add(quote.AskMargin))
				} else if i > 0 && quote.AskLayerPips.Sign() > 0 {
					pips := quote.AskLayerPips.Mul(s.makerMarket.TickSize)
					askPrice = askPrice.Add(pips)
				}
			}

			if i == 0 {
				s.logger.Infof("maker best ask price %f", askPrice.Float64())
				makerBestAskPriceMetrics.With(s.metricsLabels).Set(askPrice.Float64())
			}

			if makerQuota.BaseAsset.Lock(askQuantity) && hedgeQuota.QuoteAsset.Lock(askQuantity.Mul(askPrice)) {

				// if we bought, then we need to sell the base from the hedge session
				submitOrders = append(submitOrders, types.SubmitOrder{
					Symbol:      s.Symbol,
					Market:      s.makerMarket,
					Type:        types.OrderTypeLimit,
					Side:        types.SideTypeSell,
					Price:       askPrice,
					Quantity:    askQuantity,
					TimeInForce: types.TimeInForceGTC,
					GroupID:     s.groupID,
				})
				makerQuota.Commit()
				hedgeQuota.Commit()

				askExposureInUsd = askExposureInUsd.Add(askQuantity.Mul(askPrice))
			} else {
				makerQuota.Rollback()
				hedgeQuota.Rollback()
			}

			if s.QuantityMultiplier.Sign() > 0 {
				askQuantity = askQuantity.Mul(s.QuantityMultiplier)
			}
		}
	}

	if len(submitOrders) == 0 {
		log.Warnf("no orders generated")
		return
	}

	formattedOrders, err := s.makerSession.FormatOrders(submitOrders)
	if err != nil {
		return
	}

	orderCreateCallback := func(createdOrder types.Order) {
		s.orderStore.Add(createdOrder)
		s.activeMakerOrders.Add(createdOrder)
	}

	defer s.tradeCollector.Process()

	createdOrders, errIdx, err := bbgo.BatchPlaceOrder(ctx, s.makerSession.Exchange, orderCreateCallback, formattedOrders...)
	if err != nil {
		log.WithError(err).Errorf("unable to place maker orders: %+v", formattedOrders)
	}

	openOrderBidExposureInUsdMetrics.With(s.metricsLabels).Set(bidExposureInUsd.Float64())
	openOrderAskExposureInUsdMetrics.With(s.metricsLabels).Set(askExposureInUsd.Float64())

	_ = errIdx
	_ = createdOrders
}

func (s *Strategy) adjustHedgeQuantityWithAvailableBalance(
	account *types.Account, side types.SideType, quantity, lastPrice fixedpoint.Value,
) fixedpoint.Value {
	switch side {

	case types.SideTypeBuy:
		// check quote quantity
		if quote, ok := account.Balance(s.sourceMarket.QuoteCurrency); ok {
			if quote.Available.Compare(s.sourceMarket.MinNotional) < 0 {
				// adjust price to higher 0.1%, so that we can ensure that the order can be executed
				availableQuote := s.sourceMarket.TruncateQuoteQuantity(quote.Available)
				quantity = bbgo.AdjustQuantityByMaxAmount(quantity, lastPrice, availableQuote)

			}
		}

	case types.SideTypeSell:
		// check quote quantity
		if base, ok := account.Balance(s.sourceMarket.BaseCurrency); ok {
			if base.Available.Compare(quantity) < 0 {
				quantity = base.Available
			}
		}
	}

	// truncate the quantity to the supported precision
	return s.sourceMarket.TruncateQuantity(quantity)
}

func (s *Strategy) Hedge(ctx context.Context, pos fixedpoint.Value) {
	side := types.SideTypeBuy
	if pos.IsZero() {
		return
	}

	quantity := pos.Abs()

	if pos.Sign() < 0 {
		side = types.SideTypeSell
	}

	lastPrice := s.lastPrice
	sourceBook := s.book.CopyDepth(1)
	switch side {

	case types.SideTypeBuy:
		if bestAsk, ok := sourceBook.BestAsk(); ok {
			lastPrice = bestAsk.Price
		}

	case types.SideTypeSell:
		if bestBid, ok := sourceBook.BestBid(); ok {
			lastPrice = bestBid.Price
		}
	}

	account := s.sourceSession.GetAccount()
	if s.sourceSession.Margin {
		// check the margin level
		if !s.MinMarginLevel.IsZero() && !account.MarginLevel.IsZero() && account.MarginLevel.Compare(s.MinMarginLevel) < 0 {
			log.Errorf("margin level %f is too low (< %f), skip hedge", account.MarginLevel.Float64(), s.MinMarginLevel.Float64())
			return
		}
	} else {
		quantity = s.adjustHedgeQuantityWithAvailableBalance(account, side, quantity, lastPrice)
	}

	// truncate quantity for the supported precision
	quantity = s.sourceMarket.TruncateQuantity(quantity)

	if s.sourceMarket.IsDustQuantity(quantity, lastPrice) {
		log.Warnf("skip dust quantity: %s @ price %f", quantity.String(), lastPrice.Float64())
		return
	}

	if s.hedgeErrorRateReservation != nil {
		if !s.hedgeErrorRateReservation.OK() {
			return
		}
		bbgo.Notify("Hit hedge error rate limit, waiting...")
		time.Sleep(s.hedgeErrorRateReservation.Delay())
		s.hedgeErrorRateReservation = nil
	}

	bbgo.Notify("Submitting %s hedge order %s %v", s.Symbol, side.String(), quantity)

	submitOrders := []types.SubmitOrder{
		{
			Market:           s.sourceMarket,
			Symbol:           s.Symbol,
			Type:             types.OrderTypeMarket,
			Side:             side,
			Quantity:         quantity,
			MarginSideEffect: types.SideEffectTypeMarginBuy,
		},
	}

	formattedOrders, err := s.sourceSession.FormatOrders(submitOrders)
	if err != nil {
		log.WithError(err).Errorf("unable to format hedge orders")
		return
	}

	orderCreateCallback := func(createdOrder types.Order) {
		s.orderStore.Add(createdOrder)
	}

	defer s.tradeCollector.Process()

	createdOrders, _, err := bbgo.BatchPlaceOrder(ctx, s.sourceSession.Exchange, orderCreateCallback, formattedOrders...)
	if err != nil {
		s.hedgeErrorRateReservation = s.hedgeErrorLimiter.Reserve()
		log.WithError(err).Errorf("market order submit error: %s", err.Error())
		return
	}

	log.Infof("submitted hedge orders: %+v", createdOrders)

	// if it's selling, then we should add a positive position
	if side == types.SideTypeSell {
		s.CoveredPosition = s.CoveredPosition.Add(quantity)
	} else {
		s.CoveredPosition = s.CoveredPosition.Add(quantity.Neg())
	}
}

func (s *Strategy) tradeRecover(ctx context.Context) {
	tradeScanInterval := s.RecoverTradeScanPeriod.Duration()
	if tradeScanInterval == 0 {
		tradeScanInterval = 30 * time.Minute
	}

	tradeScanOverlapBufferPeriod := 5 * time.Minute

	tradeScanTicker := time.NewTicker(tradeScanInterval)
	defer tradeScanTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-tradeScanTicker.C:
			log.Infof("scanning trades from %s ago...", tradeScanInterval)

			if s.RecoverTrade {
				startTime := time.Now().Add(-tradeScanInterval).Add(-tradeScanOverlapBufferPeriod)

				if err := s.tradeCollector.Recover(ctx, s.sourceSession.Exchange.(types.ExchangeTradeHistoryService), s.Symbol, startTime); err != nil {
					log.WithError(err).Errorf("query trades error")
				}

				if err := s.tradeCollector.Recover(ctx, s.makerSession.Exchange.(types.ExchangeTradeHistoryService), s.Symbol, startTime); err != nil {
					log.WithError(err).Errorf("query trades error")
				}
			}
		}
	}
}

func (s *Strategy) Defaults() error {
	if s.BollBandInterval == "" {
		s.BollBandInterval = types.Interval1m
	}

	if s.BollBandMarginFactor.IsZero() {
		s.BollBandMarginFactor = fixedpoint.One
	}
	if s.BollBandMargin.IsZero() {
		s.BollBandMargin = fixedpoint.NewFromFloat(0.001)
	}

	// configure default values
	if s.UpdateInterval == 0 {
		s.UpdateInterval = types.Duration(time.Second)
	}

	if s.HedgeInterval == 0 {
		s.HedgeInterval = types.Duration(10 * time.Second)
	}

	if s.NumLayers == 0 {
		s.NumLayers = 1
	}

	if s.MinMarginLevel.IsZero() {
		s.MinMarginLevel = fixedpoint.NewFromFloat(3.0)
	}

	if s.BidMargin.IsZero() {
		if !s.Margin.IsZero() {
			s.BidMargin = s.Margin
		} else {
			s.BidMargin = defaultMargin
		}
	}

	if s.AskMargin.IsZero() {
		if !s.Margin.IsZero() {
			s.AskMargin = s.Margin
		} else {
			s.AskMargin = defaultMargin
		}
	}

	if s.CircuitBreaker == nil {
		s.CircuitBreaker = circuitbreaker.NewBasicCircuitBreaker(ID, s.InstanceID())
	}

	// circuitBreakerAlertLimiter is for CircuitBreaker alerts
	s.circuitBreakerAlertLimiter = rate.NewLimiter(rate.Every(3*time.Minute), 2)
	s.reportProfitStatsRateLimiter = rate.NewLimiter(rate.Every(3*time.Minute), 1)
	s.hedgeErrorLimiter = rate.NewLimiter(rate.Every(1*time.Minute), 1)
	return nil
}

func (s *Strategy) Validate() error {
	if s.Quantity.IsZero() || s.QuantityScale == nil {
		return errors.New("quantity or quantityScale can not be empty")
	}

	if !s.QuantityMultiplier.IsZero() && s.QuantityMultiplier.Sign() < 0 {
		return errors.New("quantityMultiplier can not be a negative number")
	}

	if len(s.Symbol) == 0 {
		return errors.New("symbol is required")
	}

	return nil
}

func (s *Strategy) quoteWorker(ctx context.Context) {
	ticker := time.NewTicker(util.MillisecondsJitter(s.UpdateInterval.Duration(), 200))
	defer ticker.Stop()

	defer func() {
		if err := s.activeMakerOrders.GracefulCancel(context.Background(), s.makerSession.Exchange); err != nil {
			log.WithError(err).Errorf("can not cancel %s orders", s.Symbol)
		}
	}()

	for {
		select {

		case <-s.stopC:
			log.Warnf("%s maker goroutine stopped, due to the stop signal", s.Symbol)
			return

		case <-ctx.Done():
			log.Warnf("%s maker goroutine stopped, due to the cancelled context", s.Symbol)
			return

		case <-ticker.C:
			s.updateQuote(ctx)

		}
	}
}

func (s *Strategy) accountUpdater(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if _, err := s.sourceSession.UpdateAccount(ctx); err != nil {
				log.WithError(err).Errorf("unable to update account")
			}

			if err := s.accountValueCalculator.UpdatePrices(ctx); err != nil {
				log.WithError(err).Errorf("unable to update account value with prices")
				return
			}

			netValue, err := s.accountValueCalculator.NetValue(ctx)
			if err != nil {
				log.WithError(err).Errorf("unable to update account")
				return
			}

			s.logger.Infof("hedge session net value ~= %f USD", netValue.Float64())
		}
	}
}

func (s *Strategy) hedgeWorker(ctx context.Context) {
	ticker := time.NewTicker(util.MillisecondsJitter(s.HedgeInterval.Duration(), 200))
	defer ticker.Stop()

	profitChanged := false
	reportTicker := time.NewTicker(5 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// For positive position and positive covered position:
			// uncover position = +5 - +3 (covered position) = 2
			//
			// For positive position and negative covered position:
			// uncover position = +5 - (-3) (covered position) = 8
			//
			// meaning we bought 5 on MAX and sent buy order with 3 on binance
			//
			// For negative position:
			// uncover position = -5 - -3 (covered position) = -2
			s.tradeCollector.Process()

			position := s.Position.GetBase()

			uncoverPosition := position.Sub(s.CoveredPosition)
			absPos := uncoverPosition.Abs()
			if !s.DisableHedge && absPos.Compare(s.sourceMarket.MinQuantity) > 0 {
				log.Infof("%s base position %v coveredPosition: %v uncoverPosition: %v",
					s.Symbol,
					position,
					s.CoveredPosition,
					uncoverPosition,
				)

				s.Hedge(ctx, uncoverPosition.Neg())
				profitChanged = true
			}

		case <-reportTicker.C:
			if profitChanged {
				if s.reportProfitStatsRateLimiter.Allow() {
					bbgo.Notify(s.ProfitStats)
				}

				profitChanged = false
			}
		}
	}
}

func (s *Strategy) CrossRun(
	ctx context.Context, orderExecutionRouter bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession,
) error {
	instanceID := s.InstanceID()

	// configure sessions
	sourceSession, ok := sessions[s.SourceExchange]
	if !ok {
		return fmt.Errorf("source exchange session %s is not defined", s.SourceExchange)
	}

	s.sourceSession = sourceSession

	// initialize the price resolver
	sourceMarkets := s.sourceSession.Markets()
	s.priceSolver = pricesolver.NewSimplePriceResolver(sourceMarkets)

	makerSession, ok := sessions[s.MakerExchange]
	if !ok {
		return fmt.Errorf("maker exchange session %s is not defined", s.MakerExchange)
	}

	s.makerSession = makerSession

	s.sourceMarket, ok = s.sourceSession.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("source session market %s is not defined", s.Symbol)
	}

	s.makerMarket, ok = s.makerSession.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("maker session market %s is not defined", s.Symbol)
	}

	s.accountValueCalculator = bbgo.NewAccountValueCalculator(s.sourceSession, s.sourceMarket.QuoteCurrency)

	indicators := s.sourceSession.Indicators(s.Symbol)

	s.boll = indicators.BOLL(types.IntervalWindow{
		Interval: s.BollBandInterval,
		Window:   21,
	}, 1.0)

	// restore state
	s.groupID = util.FNV32(instanceID)
	log.Infof("using group id %d from fnv(%s)", s.groupID, instanceID)

	configLabels := prometheus.Labels{"strategy_id": s.InstanceID(), "strategy_type": ID, "symbol": s.Symbol}
	configNumOfLayersMetrics.With(configLabels).Set(float64(s.NumLayers))
	configMaxExposureMetrics.With(configLabels).Set(s.MaxExposurePosition.Float64())
	configBidMarginMetrics.With(configLabels).Set(s.BidMargin.Float64())
	configAskMarginMetrics.With(configLabels).Set(s.AskMargin.Float64())

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.makerMarket)
		s.Position.Strategy = ID
		s.Position.StrategyInstanceID = instanceID
	}

	bbgo.Notify("xmaker: %s position is restored", s.Symbol, s.Position)

	if s.ProfitStats == nil {
		s.ProfitStats = &ProfitStats{
			ProfitStats:   types.NewProfitStats(s.makerMarket),
			MakerExchange: s.makerSession.ExchangeName,
		}
	}

	if s.CoveredPosition.IsZero() {
		if s.state != nil && !s.CoveredPosition.IsZero() {
			s.CoveredPosition = s.state.CoveredPosition
		}
	}

	if s.makerSession.MakerFeeRate.Sign() > 0 || s.makerSession.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(types.ExchangeName(s.MakerExchange), types.ExchangeFee{
			MakerFeeRate: s.makerSession.MakerFeeRate,
			TakerFeeRate: s.makerSession.TakerFeeRate,
		})
	}

	if s.sourceSession.MakerFeeRate.Sign() > 0 || s.sourceSession.TakerFeeRate.Sign() > 0 {
		s.Position.SetExchangeFeeRate(types.ExchangeName(s.SourceExchange), types.ExchangeFee{
			MakerFeeRate: s.sourceSession.MakerFeeRate,
			TakerFeeRate: s.sourceSession.TakerFeeRate,
		})
	}

	s.sourceSession.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, types.Interval1m, func(k types.KLine) {
		s.priceSolver.Update(k.Symbol, k.Close)
		feeToken := s.sourceSession.Exchange.PlatformFeeCurrency()
		if feePrice, ok := s.priceSolver.ResolvePrice(feeToken, "USDT"); ok {
			s.Position.SetFeeAverageCost(feeToken, feePrice)
		}
	}))

	if s.ProfitFixerConfig != nil {
		bbgo.Notify("Fixing %s profitStats and position...", s.Symbol)

		log.Infof("profitFixer is enabled, checking checkpoint: %+v", s.ProfitFixerConfig.TradesSince)

		if s.ProfitFixerConfig.TradesSince.Time().IsZero() {
			return errors.New("tradesSince time can not be zero")
		}

		makerMarket, _ := makerSession.Market(s.Symbol)
		position := types.NewPositionFromMarket(makerMarket)
		profitStats := types.NewProfitStats(makerMarket)

		fixer := common.NewProfitFixer()
		// fixer.ConverterManager = s.ConverterManager

		if ss, ok := makerSession.Exchange.(types.ExchangeTradeHistoryService); ok {
			log.Infof("adding makerSession %s to profitFixer", makerSession.Name)
			fixer.AddExchange(makerSession.Name, ss)
		}

		if ss, ok := sourceSession.Exchange.(types.ExchangeTradeHistoryService); ok {
			log.Infof("adding hedgeSession %s to profitFixer", sourceSession.Name)
			fixer.AddExchange(sourceSession.Name, ss)
		}

		if err2 := fixer.Fix(ctx, makerMarket.Symbol,
			s.ProfitFixerConfig.TradesSince.Time(),
			time.Now(),
			profitStats,
			position); err2 != nil {
			return err2
		}

		bbgo.Notify("Fixed %s position", s.Symbol, position)
		bbgo.Notify("Fixed %s profitStats", s.Symbol, profitStats)

		s.Position = position
		s.ProfitStats.ProfitStats = profitStats
	}

	s.book = types.NewStreamBook(s.Symbol, s.sourceSession.ExchangeName)
	s.book.BindStream(s.sourceSession.MarketDataStream)

	if s.EnableSignalMargin {
		scale, err := s.SignalMarginScale.Scale()
		if err != nil {
			return err
		}
		if solveErr := scale.Solve(); solveErr != nil {
			return solveErr
		}
	}

	for _, signalConfig := range s.SignalConfigList {
		if signalConfig.OrderBookBestPriceSignal != nil {
			signalConfig.OrderBookBestPriceSignal.book = s.book
			if err := signalConfig.OrderBookBestPriceSignal.Bind(ctx, s.sourceSession, s.Symbol); err != nil {
				return err
			}
		} else if signalConfig.BollingerBandTrendSignal != nil {
			if err := signalConfig.BollingerBandTrendSignal.Bind(ctx, s.sourceSession, s.Symbol); err != nil {
				return err
			}
		}
	}

	s.activeMakerOrders = bbgo.NewActiveOrderBook(s.Symbol)
	s.activeMakerOrders.BindStream(s.makerSession.UserDataStream)

	s.orderStore = core.NewOrderStore(s.Symbol)
	s.orderStore.BindStream(s.sourceSession.UserDataStream)
	s.orderStore.BindStream(s.makerSession.UserDataStream)

	s.tradeCollector = core.NewTradeCollector(s.Symbol, s.Position, s.orderStore)

	if s.NotifyTrade {
		s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
			bbgo.Notify(trade)
		})
	}

	s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
		c := trade.PositionChange()
		if trade.Exchange == s.sourceSession.ExchangeName {
			s.CoveredPosition = s.CoveredPosition.Add(c)
		}

		s.ProfitStats.AddTrade(trade)

		if profit.Compare(fixedpoint.Zero) == 0 {
			s.Environment.RecordPosition(s.Position, trade, nil)
		}
	})

	// TODO: remove this nil value behavior, check all OnProfit usage and remove the EmitProfit call with nil profit
	s.tradeCollector.OnProfit(func(trade types.Trade, profit *types.Profit) {
		if profit != nil {
			if s.CircuitBreaker != nil {
				s.CircuitBreaker.RecordProfit(profit.Profit, trade.Time.Time())
			}

			bbgo.Notify(profit)

			s.ProfitStats.AddProfit(*profit)
			s.Environment.RecordPosition(s.Position, trade, profit)
		}
	})

	s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
		bbgo.Notify(position)
	})

	s.tradeCollector.OnRecover(func(trade types.Trade) {
		bbgo.Notify("Recovered trade", trade)
	})

	// bind two user data streams so that we can collect the trades together
	s.tradeCollector.BindStream(s.sourceSession.UserDataStream)
	s.tradeCollector.BindStream(s.makerSession.UserDataStream)

	s.stopC = make(chan struct{})

	if s.RecoverTrade {
		go s.tradeRecover(ctx)
	}

	go s.accountUpdater(ctx)
	go s.hedgeWorker(ctx)
	go s.quoteWorker(ctx)

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		// the ctx here is the shutdown context (not the strategy context)

		// defer work group done to mark the strategy as stopped
		defer wg.Done()

		// send stop signal to the quoteWorker
		close(s.stopC)

		// wait for the quoter to stop
		time.Sleep(s.UpdateInterval.Duration())

		if err := s.activeMakerOrders.GracefulCancel(ctx, s.makerSession.Exchange); err != nil {
			log.WithError(err).Errorf("graceful cancel error")
		}

		bbgo.Notify("Shutting down %s %s", ID, s.Symbol, s.Position)
	})

	return nil
}
