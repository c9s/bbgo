package xalign

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/interact"
	"github.com/c9s/bbgo/pkg/livenote"
	"github.com/c9s/bbgo/pkg/pricesolver"
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	"github.com/c9s/bbgo/pkg/strategy/xalign/detector"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xalign"

var log = logrus.WithField("strategy", ID)

const defaultPriceQuoteCurrency = "USDT"

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type QuoteCurrencyPreference struct {
	Buy  []string `json:"buy"`
	Sell []string `json:"sell"`
}

type LargeAmountAlertConfig struct {
	Slack *slackalert.SlackAlert `json:"slack"`

	QuoteCurrency string           `json:"quoteCurrency"`
	Amount        fixedpoint.Value `json:"amount"`
}

type Strategy struct {
	*bbgo.Environment
	ActiveTransferInterval   types.Duration              `json:"interval"` // keep the same tag name for backward compatibility
	IdleInterval             *types.Duration             `json:"idleInterval,omitempty"`
	PreferredSessions        []string                    `json:"sessions"`
	PreferredQuoteCurrencies *QuoteCurrencyPreference    `json:"quoteCurrencies"`
	ExpectedBalances         map[string]fixedpoint.Value `json:"expectedBalances"`
	UseTakerOrder            bool                        `json:"useTakerOrder"`
	DryRun                   bool                        `json:"dryRun"`
	BalanceToleranceRange    fixedpoint.Value            `json:"balanceToleranceRange"`
	Duration                 types.Duration              `json:"for"`
	InstantAlignAmount       fixedpoint.Value            `json:"instantAlignAmount"`
	Disabled                 bool                        `json:"disabled"`
	// InteractiveOrderDelay is the delay duration for interactive order confirmation in Slack
	InteractiveOrderDelay types.Duration `json:"interactiveOrderDelay"`
	// InteractiveOrderEnabled enables interactive order confirmation in Slack
	InteractiveOrderEnabled bool `json:"interactiveOrderEnabled"`
	// isInteractiveOrderEnabled is true iff InteractiveOrderEnabled = true and the interactive order dispatcher is available
	isInteractiveOrderEnabled bool

	WarningDuration types.Duration `json:"warningFor"`

	SkipTransferCheck bool `json:"skipTransferCheck"`

	MaxQuantity map[string]fixedpoint.Value `json:"maxQuantity"`

	LargeAmountAlert *LargeAmountAlertConfig `json:"largeAmountAlert"`

	SlackNotify                bool             `json:"slackNotify"`
	SlackNotifyMentions        []string         `json:"slackNotifyMentions"`
	SlackNotifyThresholdAmount fixedpoint.Value `json:"slackNotifyThresholdAmount,omitempty"`

	SkipAlignOnAnyActiveTransfer bool `json:"skipAlignOnAnyActiveTransfer"`

	ActiveTransferTimeWindow types.Duration `json:"activeTransferTimeWindow"`

	deviationDetectors map[string]*detector.DeviationDetector[types.Balance]

	priceResolver *pricesolver.SimplePriceSolver

	sessions   bbgo.ExchangeSessionMap
	orderBooks map[string]*bbgo.ActiveOrderBook

	orderStore *core.OrderStore

	activeTransferNotificationLimiter *rate.Limiter

	deltaGaugesMap map[deltaGaugeKey]prometheus.Gauge

	ticker     *time.Ticker
	slackEvtID string
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	var cs []string

	if s.ExpectedBalances != nil {
		for cur := range s.ExpectedBalances {
			cs = append(cs, cur)
		}
	}

	return ID + strings.Join(s.PreferredSessions, "-") + strings.Join(cs, "-")
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
	s.subscribePrices(sessions)
}

func (s *Strategy) Defaults() error {
	if s.BalanceToleranceRange == fixedpoint.Zero {
		s.BalanceToleranceRange = fixedpoint.NewFromFloat(0.01)
	}

	if s.Duration == 0 {
		s.Duration = types.Duration(15 * time.Minute)
	}

	if s.ExpectedBalances == nil {
		s.ExpectedBalances = make(map[string]fixedpoint.Value)
	}

	if s.ActiveTransferTimeWindow == 0 {
		s.ActiveTransferTimeWindow = types.Duration(48 * time.Hour)
	}

	if s.IdleInterval == nil {
		s.IdleInterval = &s.ActiveTransferInterval
	}

	if s.InstantAlignAmount.IsZero() {
		s.InstantAlignAmount = fixedpoint.NewFromFloat(50.0)
	}

	if s.InteractiveOrderDelay == 0 {
		s.InteractiveOrderDelay = types.Duration(5 * time.Minute)
	}

	return nil
}

func (s *Strategy) Initialize() error {
	s.activeTransferNotificationLimiter = rate.NewLimiter(rate.Every(5*time.Minute), 1)

	s.sessions = make(map[string]*bbgo.ExchangeSession)
	s.orderBooks = make(map[string]*bbgo.ActiveOrderBook)
	s.orderStore = core.NewOrderStore("")

	s.deviationDetectors = make(map[string]*detector.DeviationDetector[types.Balance])
	for currency, expectedValue := range s.ExpectedBalances {
		s.deviationDetectors[currency] = detector.NewDeviationDetector(
			types.Balance{Currency: currency, NetAsset: expectedValue}, // Expected value
			s.BalanceToleranceRange.Float64(),                          // Tolerance (1%)
			s.Duration.Duration(),                                      // Duration for sustained deviation
			// can switch from s.netBalanceValue to s.netBalance,
			s.netBalanceValue,
		)

		s.deviationDetectors[currency].SetLogger(log)
	}

	s.deltaGaugesMap = make(map[deltaGaugeKey]prometheus.Gauge)
	s.slackEvtID = uuid.NewString()

	return nil
}

// netBalanceValue returns the net balance value for a given balance
func (s *Strategy) netBalanceValue(b types.Balance) (float64, error) {
	if s.priceResolver == nil {
		return 0.0, errors.New("price resolver not initialized")
	}

	if assetPrice, ok := s.priceResolver.ResolvePrice(b.Currency, defaultPriceQuoteCurrency); ok {
		return b.Net().Float64() * assetPrice.Float64(), nil
	}

	return 0.0, fmt.Errorf("unable to resolve price for %s", b.Currency)
}

func (s *Strategy) netBalance(b types.Balance) (float64, error) {
	return b.Net().Float64(), nil
}

func (s *Strategy) Validate() error {
	if s.PreferredQuoteCurrencies == nil {
		return errors.New("quoteCurrencies is not defined")
	}

	if s.ExpectedBalances == nil || len(s.ExpectedBalances) == 0 {
		return errors.New("expectedBalances is not defined")
	}

	return nil
}

func (s *Strategy) selectSessionForCurrency(
	ctx context.Context, sessions map[string]*bbgo.ExchangeSession, currency string, changeQuantity fixedpoint.Value,
) (*bbgo.ExchangeSession, *types.SubmitOrder) {
	var taker = s.UseTakerOrder
	var side = s.selectAdjustmentOrderSide(changeQuantity)
	var quoteCurrencies []string
	if changeQuantity.Sign() > 0 {
		quoteCurrencies = s.PreferredQuoteCurrencies.Buy
	} else {
		quoteCurrencies = s.PreferredQuoteCurrencies.Sell
	}

	for _, sessionName := range s.PreferredSessions {
		session, ok := sessions[sessionName]
		if !ok {
			log.Errorf("session %s not found, please check the preferredSessions settings", sessionName)
			continue
		}

		account, err := session.UpdateAccount(ctx)
		if err != nil {
			log.WithError(err).Errorf("unable to update account for session %s", sessionName)
			continue
		}

		for _, fromQuoteCurrency := range quoteCurrencies {
			// skip the same currency, because there is no such USDT/USDT market
			if currency == fromQuoteCurrency {
				continue
			}

			// check both fromQuoteCurrency/currency and currency/fromQuoteCurrency
			reversed := false
			baseCurrency := currency
			quoteCurrency := fromQuoteCurrency
			symbol := currency + quoteCurrency
			market, ok := session.Market(symbol)
			if !ok {
				// for TWD in USDT/TWD market, buy TWD means sell USDT
				baseCurrency = fromQuoteCurrency
				quoteCurrency = currency
				symbol = baseCurrency + currency
				market, ok = session.Market(symbol)
				if !ok {
					continue
				}

				// reverse side
				side = side.Reverse()
				reversed = true
			}

			ticker, err := session.Exchange.QueryTicker(ctx, symbol)
			if err != nil {
				log.WithError(err).Errorf("unable to query ticker on %s (%s)", symbol, session.Name)
				continue
			}

			spread := ticker.Sell.Sub(ticker.Buy)

			// changeQuantity > 0 = buy
			// changeQuantity < 0 = sell
			q := changeQuantity.Abs()

			// a fast filtering
			if reversed {
				if q.Compare(market.MinNotional) < 0 {
					log.Debugf("skip dust notional: %f", q.Float64())
					continue
				}
			} else {
				if q.Compare(market.MinQuantity) < 0 {
					log.Debugf("skip dust quantity: %f", q.Float64())
					continue
				}
			}

			log.Infof("%s changeQuantity: %f ticker: %+v market: %+v", symbol, changeQuantity.Float64(), ticker, market)

			switch side {

			case types.SideTypeBuy:
				var price fixedpoint.Value
				if taker {
					price = ticker.Sell
				} else if spread.Compare(market.TickSize) > 0 {
					price = ticker.Sell.Sub(market.TickSize)
				} else {
					price = ticker.Buy
				}

				quoteBalance, ok := account.Balance(quoteCurrency)
				if !ok || quoteBalance.Available.IsZero() {
					continue
				}

				requiredQuoteAmount := fixedpoint.Zero
				if reversed {
					requiredQuoteAmount = q
				} else {
					requiredQuoteAmount = q.Mul(price)
				}

				requiredQuoteAmount = requiredQuoteAmount.Round(market.PricePrecision, fixedpoint.Up)
				if requiredQuoteAmount.Compare(quoteBalance.Available) > 0 {
					log.Warnf("required quote amount %f > quote balance %v, adjusting", requiredQuoteAmount.Float64(), quoteBalance)

					q = quoteBalance.Available.Div(price).Round(market.VolumePrecision, fixedpoint.Down)
					requiredQuoteAmount = q.Mul(price)
				}

				// for currency = TWD in market USDT/TWD
				// since the side is reversed, the quote currency is also "TWD" here.
				//
				// for currency = BTC in market BTC/USDT and the side is buy
				// we want to check if the quote currency USDT used up another expected balance.
				if quoteCurrency != currency {
					if expectedQuoteBalance, ok := s.ExpectedBalances[quoteCurrency]; ok {
						rest := quoteBalance.Total().Sub(requiredQuoteAmount)
						if rest.Compare(expectedQuoteBalance) < 0 {
							log.Warnf("required quote amount %f will use up the expected balance %f, skip", requiredQuoteAmount.Float64(), expectedQuoteBalance.Float64())
							continue
						}
					}
				}

				maxQuantity, ok := s.MaxQuantity[market.QuoteCurrency]
				if ok && requiredQuoteAmount.Compare(maxQuantity) > 0 {
					log.Infof("adjusted required quote ammount %f %s by max amount %f %s", requiredQuoteAmount.Float64(), market.QuoteCurrency, maxQuantity.Float64(), market.QuoteCurrency)
					requiredQuoteAmount = maxQuantity
				}

				if quantity, ok := market.GreaterThanMinimalOrderQuantity(side, price, requiredQuoteAmount); ok {
					return session, &types.SubmitOrder{
						Symbol:      symbol,
						Side:        side,
						Type:        types.OrderTypeLimit,
						Quantity:    quantity,
						Price:       price,
						Market:      market,
						TimeInForce: types.TimeInForceGTC,
					}
				} else {
					log.Warnf("The amount %f is not greater than the minimal order quantity for %s", requiredQuoteAmount.Float64(), market.Symbol)
				}

			case types.SideTypeSell:
				var price fixedpoint.Value
				if taker {
					price = ticker.Buy
				} else if spread.Compare(market.TickSize) > 0 {
					price = ticker.Buy.Add(market.TickSize)
				} else {
					price = ticker.Sell
				}

				if reversed {
					q = q.Div(price)
				}

				baseBalance, ok := account.Balance(baseCurrency)
				if !ok || baseBalance.Available.IsZero() {
					continue
				}

				if q.Compare(baseBalance.Available) > 0 {
					log.Warnf("required base amount %f < available base balance %v, skip", q.Float64(), baseBalance)

					q = baseBalance.Available
				}

				maxAmount, ok := s.MaxQuantity[market.QuoteCurrency]
				if ok {
					q = bbgo.AdjustQuantityByMaxAmount(q, price, maxAmount)
					log.Infof("adjusted quantity %f %s by max amount %f %s", q.Float64(), market.BaseCurrency, maxAmount.Float64(), market.QuoteCurrency)
				}

				if quantity, ok := market.GreaterThanMinimalOrderQuantity(side, price, q); ok {
					return session, &types.SubmitOrder{
						Symbol:      symbol,
						Side:        side,
						Type:        types.OrderTypeLimit,
						Quantity:    quantity,
						Price:       price,
						Market:      market,
						TimeInForce: types.TimeInForceGTC,
					}
				} else {
					log.Warnf("The amount %f is not greater than the minimal order quantity for %s", q.Float64(), market.Symbol)
				}
			}

		}
	}

	return nil, nil
}

func (s *Strategy) selectAdjustmentOrderSide(changeQuantity fixedpoint.Value) (side types.SideType) {
	if changeQuantity.Sign() > 0 {
		side = types.SideTypeBuy
	} else {
		side = types.SideTypeSell
	}
	return side
}

func (s *Strategy) CrossRun(
	ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession,
) error {
	instanceID := s.InstanceID()

	if err := dynamic.InitializeConfigMetrics(ID, instanceID, s); err != nil {
		return err
	}

	s.sessions = sessions

	s.sessions = s.sessions.Filter(s.PreferredSessions)
	for sessionName, session := range s.sessions {
		// bind session stream to the global order store
		s.orderStore.BindStream(session.UserDataStream)

		orderBook := bbgo.NewActiveOrderBook("")
		orderBook.BindStream(session.UserDataStream)
		s.orderBooks[sessionName] = orderBook
	}

	s.priceResolver = s.initializePriceResolver(s.sessions.CollectMarkets(s.PreferredSessions))

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		for n, session := range s.sessions {
			if ob, ok := s.orderBooks[n]; ok {
				_ = ob.GracefulCancel(ctx, session.Exchange)
			}
		}
		cancelAllInteractiveOrders()
	})

	// setup interactive order
	dispatcher, err := interact.GetDispatcher()
	if err != nil {
		bbgo.Notify("[xalign] interactive order is not enabled: %s", err.Error())
	} else if s.InteractiveOrderEnabled {
		s.isInteractiveOrderEnabled = true
		bbgo.Notify("[xalign] interactive order is enabled: %s", s.InstanceID())
		setupSlackInteractionCallback(s.slackEvtID, dispatcher)
	}

	go s.monitor(ctx)
	return nil
}

func (s *Strategy) monitor(ctx context.Context) {
	activeTransferExists := s.align(ctx, s.sessions)
	s.ticker = time.NewTicker(time.Second) // dummy initial duration, will be updated later
	defer s.ticker.Stop()

	currentInterval, detectDuration := s.nextDectectParams(activeTransferExists)
	s.updateDurations(currentInterval, detectDuration)

	for {
		log.Infof("current xalign monitor interval: %s", currentInterval)
		select {
		case <-ctx.Done():
			log.Infof("xalign monitor exiting...")
			return
		case <-s.ticker.C:
			activeTransferExists = s.align(ctx, s.sessions)
			currentInterval, detectDuration = s.nextDectectParams(activeTransferExists)
			s.updateDurations(currentInterval, detectDuration)
		}
	}
}

func (s *Strategy) updateDurations(tickDuration, detectDuration time.Duration) {
	s.ticker.Reset(tickDuration)
	log.Infof("deviation detection duration set to %s", detectDuration)
	for _, d := range s.deviationDetectors {
		d.SetDuration(detectDuration)
	}
}

func (s *Strategy) nextDectectParams(foundActiveTransfer bool) (interval, detectDuration time.Duration) {
	activeInterval := s.ActiveTransferInterval.Duration()
	idleInterval := s.IdleInterval.Duration()
	if foundActiveTransfer {
		return activeInterval, 2 * time.Hour
	}
	return idleInterval, s.Duration.Duration()
}

func (s *Strategy) initializePriceResolver(allMarkets types.MarketMap) *pricesolver.SimplePriceSolver {
	priceResolver := pricesolver.NewSimplePriceResolver(allMarkets)
	for _, session := range s.sessions {
		for symbol, price := range session.LastPrices() {
			priceResolver.Update(symbol, price)
		}

		session.MarketDataStream.OnKLineClosed(func(k types.KLine) {
			priceResolver.Update(k.Symbol, k.Close)
		})
	}

	return priceResolver
}

func (s *Strategy) subscribePrices(sessions map[string]*bbgo.ExchangeSession) {
	if s.ExpectedBalances == nil {
		return
	}

	possibleQuoteCurrencies := append(s.PreferredQuoteCurrencies.Buy, s.PreferredQuoteCurrencies.Sell...)
	for asset := range s.ExpectedBalances {
		for _, quote := range possibleQuoteCurrencies {
			for _, sId := range s.PreferredSessions {
				if session, ok := sessions[sId]; ok {
					markets := session.Markets()
					if len(markets) == 0 {
						panic("session has no markets")
					}

					market, foundMarket := markets.FindPair(asset, quote)
					if !foundMarket {
						// skip to next market
						continue
					}

					session.Subscribe(types.KLineChannel, market.Symbol, types.SubscribeOptions{Interval: types.Interval5m})
				}
			}
		}
	}
}

func (s *Strategy) resetFaultBalanceRecords(currency string) {
	if s.deviationDetectors == nil {
		return
	}

	if d, ok := s.deviationDetectors[currency]; ok {
		d.ClearRecords()
	}
}

// recordBalances records the balances for each currency in the totalBalances map.
func (s *Strategy) recordBalances(totalBalances types.BalanceMap, now time.Time) {
	if s.ExpectedBalances == nil {
		return
	}

	for currency := range s.ExpectedBalances {
		balance, hasBal := totalBalances[currency]
		if d, ok := s.deviationDetectors[currency]; ok {
			if hasBal {
				d.AddRecord(now, balance)
			} else {
				d.AddRecord(now, types.NewZeroBalance(currency))
			}
		}
	}
}

func (s *Strategy) align(ctx context.Context, sessions bbgo.ExchangeSessionMap) (activeTransferExists bool) {
	if s.Disabled {
		bbgo.Notify("xalign strategy is disabled, skipping alignment")
		return false
	}

	for sessionName, session := range sessions {
		ob, ok := s.orderBooks[sessionName]
		if !ok {
			log.Errorf("orderbook on session %s not found", sessionName)
			return false
		}

		if err := ob.GracefulCancel(ctx, session.Exchange); err != nil {
			log.WithError(err).Errorf("unable to cancel order")
		}
	}

	// detect if there is any active transfer for this currency. Skip aligning if there is any.
	activeTransfers, err := s.getActiveTransfers(ctx, sessions)
	if err != nil {
		log.WithError(err).Errorf("unable to transfer activities")
		return false
	}

	activeTransferExists = len(activeTransfers) > 0
	if s.SkipAlignOnAnyActiveTransfer && activeTransferExists {
		log.Info("balance alignment will be skipped due to active transfer")
		if s.activeTransferNotificationLimiter.Allow() {
			assetNames := ""
			for asset := range activeTransfers {
				assetNames += asset + " "
			}
			bbgo.Notify(
				"Balance alignment will be skipped due to active transfer on assets: %s",
				assetNames,
			)
		}
	}

	totalBalances, _, err := sessions.AggregateBalances(ctx, false)
	if err != nil {
		log.WithError(err).Errorf("unable to aggregate balances")
		return activeTransferExists
	}

	s.recordBalances(totalBalances, time.Now())

	log.Debugf("checking all fault balance records...")

	amountQuoteCurrency := defaultPriceQuoteCurrency
	if s.LargeAmountAlert != nil {
		amountQuoteCurrency = s.LargeAmountAlert.QuoteCurrency
	}

	for currency, expectedBalance := range s.ExpectedBalances {
		q := s.calculateRefillQuantity(totalBalances, currency, expectedBalance)
		quantity := q.Abs()

		price, ok := s.priceResolver.ResolvePrice(currency, amountQuoteCurrency)
		if !ok {
			log.Warnf("unable to resolve price for %s, skip alert checking", currency)
			continue
		}

		amount := price.Mul(quantity)
		if price.Sign() > 0 {
			log.Infof("resolved price for currency: %s, price: %f, quantity: %f, amount: %f", currency, price.Float64(), quantity.Float64(), amount.Float64())
		}

		s.updateMetrics(
			currency,
			s.selectAdjustmentOrderSide(q).String(),
			quantity.Float64(),
			amount.Float64(),
		)
		if s.SkipAlignOnAnyActiveTransfer && activeTransferExists {
			// if there is any active transfer and SkipAlignOnAnyActiveTransfer is true, skip all alignment
			continue
		}

		if activeTransferExists {
			if transfer, ok := activeTransfers[currency]; ok {
				log.Infof("skip balance alignment due to active transfer: %s", currency)
				if s.activeTransferNotificationLimiter.Allow() {
					bbgo.Notify(
						"Skip balance alignment for %s due to active transfer",
						currency,
						transfer,
					)
				}
				continue
			}
		}

		log.Debugf("checking %s fault balance records...", currency)

		isInstantAmount := amount.Compare(s.InstantAlignAmount) <= 0
		if d, ok := s.deviationDetectors[currency]; ok {
			should, sustainedDuration := d.ShouldFix()
			if sustainedDuration > 0 {
				log.Infof("%s sustained deviation for %s...", currency, sustainedDuration)
			}
			// always adjust for small amount discrepancies
			if isInstantAmount {
				should = true
			}

			if s.WarningDuration > 0 && sustainedDuration >= s.WarningDuration.Duration() {
				if s.LargeAmountAlert != nil && price.Sign() > 0 {
					if amount.Compare(s.LargeAmountAlert.Amount) > 0 {
						cbd := &CriticalBalanceDiscrepancyAlert{
							Warning:           true,
							SlackAlert:        s.LargeAmountAlert.Slack,
							QuoteCurrency:     s.LargeAmountAlert.QuoteCurrency,
							AlertAmount:       s.LargeAmountAlert.Amount,
							BaseCurrency:      currency,
							SustainedDuration: sustainedDuration,

							Side:     s.selectAdjustmentOrderSide(q),
							Price:    price,
							Delta:    q,
							Quantity: quantity,
							Amount:   amount,
						}
						bbgo.PostLiveNote(
							cbd,
							livenote.Channel(s.LargeAmountAlert.Slack.Channel),
							livenote.OneTimeMention(s.LargeAmountAlert.Slack.Mentions...),
							cbd.Comment(),
						)
					}
				}
			}

			if !should {
				continue
			}
		}

		if s.LargeAmountAlert != nil && price.Sign() > 0 {
			if amount.Compare(s.LargeAmountAlert.Amount) > 0 {
				cbd := &CriticalBalanceDiscrepancyAlert{
					SlackAlert:        s.LargeAmountAlert.Slack,
					QuoteCurrency:     s.LargeAmountAlert.QuoteCurrency,
					AlertAmount:       s.LargeAmountAlert.Amount,
					BaseCurrency:      currency,
					SustainedDuration: s.Duration.Duration(),

					Side:     s.selectAdjustmentOrderSide(q),
					Price:    price,
					Delta:    q,
					Quantity: quantity,
					Amount:   amount,
				}
				bbgo.PostLiveNote(
					cbd,
					livenote.Channel(s.LargeAmountAlert.Slack.Channel),
					livenote.OneTimeMention(s.LargeAmountAlert.Slack.Mentions...),
					cbd.Comment(),
				)
			}
		}

		selectedSession, submitOrder := s.selectSessionForCurrency(ctx, sessions, currency, q)
		if selectedSession != nil && submitOrder != nil {
			if market, hasMarket := selectedSession.Market(submitOrder.Symbol); hasMarket && submitOrder.Price.Sign() > 0 {
				if market.IsDustQuantity(q.Abs(), submitOrder.Price) {
					log.Infof("skip placing dust order for %s: %+v", currency, submitOrder)
					return activeTransferExists
				}
			}

			log.Infof("placing %s order on %s: %+v", submitOrder.Symbol, selectedSession.Name, submitOrder)

			bbgo.Notify("Aligning %s position on exchange session %s, delta: %f %s, expected balance: %f %s",
				currency, selectedSession.Name,
				q.Float64(), currency,
				expectedBalance.Float64(), currency)

			if s.DryRun {
				return activeTransferExists
			}
			if isInstantAmount || !s.isInteractiveOrderEnabled {
				// place the order immediately if one of the conditions met:
				// 1. the amount is small enough (<= InstantAlignAmount)
				// 2. interactive order is not enabled
				createdOrder, err := selectedSession.Exchange.SubmitOrder(ctx, *submitOrder)
				s.onSubmittedOrderCallback(selectedSession, submitOrder, createdOrder, err)
			} else {
				// submit the order via interactive order with slack confirmation
				// first we check if there is already a delayed interactive order for the same symbol and slack event ID
				foundDelayedOrder := false
				interactOrderRegistry.Range(func(k any, v any) bool {
					o, ok := v.(*InteractiveSubmitOrder)
					if ok {
						if o.submitOrder.Symbol == submitOrder.Symbol && o.slackEvtID == s.slackEvtID {
							foundDelayedOrder = true
							return false
						}
					}
					return true
				})
				if foundDelayedOrder {
					bbgo.Notify("found existing delayed interactive order for %s, skip placing another one", submitOrder.Symbol)
					continue
				}
				// place interactive order with slack confirmation
				// the order will be placed after confirmed in slack or after delay timeout
				mentions := append([]string{}, s.SlackNotifyMentions...)
				if s.LargeAmountAlert != nil && s.LargeAmountAlert.Slack != nil {
					mentions = append(mentions, s.LargeAmountAlert.Slack.Mentions...)
				}
				itOrder := NewInteractiveSubmitOrder(
					*submitOrder,
					s.InteractiveOrderDelay.Duration(),
					mentions,
					s.slackEvtID,
				)
				itOrder.AsyncSubmit(ctx, selectedSession, s.onSubmittedOrderCallback)
			}
		}
	}
	return activeTransferExists
}

func (s *Strategy) onSubmittedOrderCallback(session *bbgo.ExchangeSession, submitOrder *types.SubmitOrder, createdOrder *types.Order, err error) {
	if err != nil {
		log.WithError(err).WithFields(submitOrder.LogFields()).Errorf("can not place order: %+v", submitOrder)
	} else if createdOrder != nil {
		s.resetFaultBalanceRecords(createdOrder.Market.BaseCurrency)
		if ob, ok := s.orderBooks[session.Name]; ok {
			ob.Add(*createdOrder)
		} else {
			log.Errorf("orderbook %s not found", session.Name)
		}
		bbgo.Notify("Aligning order submitted", createdOrder)
	}
}

func (s *Strategy) calculateRefillQuantity(
	totalBalances types.BalanceMap, currency string, expectedBalance fixedpoint.Value,
) fixedpoint.Value {
	if b, ok := totalBalances[currency]; ok {
		netBalance := b.Net()
		return expectedBalance.Sub(netBalance)
	}
	return expectedBalance
}

func (s *Strategy) aggregateBalances(
	ctx context.Context, sessions map[string]*bbgo.ExchangeSession,
) (types.BalanceMap, map[string]types.BalanceMap, error) {
	totalBalances := make(types.BalanceMap)
	sessionBalances := make(map[string]types.BalanceMap)

	// iterate the sessions and record them
	for sessionName, session := range sessions {
		// update the account balances and the margin information
		account, err := session.UpdateAccount(ctx)
		if err != nil {
			log.WithError(err).Errorf("unable to update account")
			return nil, nil, err
		}

		balances := account.Balances()
		sessionBalances[sessionName] = balances
		totalBalances = totalBalances.Add(balances)
	}

	return totalBalances, sessionBalances, nil
}
