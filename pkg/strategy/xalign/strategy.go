package xalign

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/core"
	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/pricesolver"
	"github.com/c9s/bbgo/pkg/slack/slackalert"
	"github.com/c9s/bbgo/pkg/strategy/xalign/detector"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xalign"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
	log.Logger.SetReportCaller(true)
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
	Interval                 types.Interval              `json:"interval"`
	PreferredSessions        []string                    `json:"sessions"`
	PreferredQuoteCurrencies *QuoteCurrencyPreference    `json:"quoteCurrencies"`
	ExpectedBalances         map[string]fixedpoint.Value `json:"expectedBalances"`
	UseTakerOrder            bool                        `json:"useTakerOrder"`
	DryRun                   bool                        `json:"dryRun"`
	BalanceToleranceRange    fixedpoint.Value            `json:"balanceToleranceRange"`
	Duration                 types.Duration              `json:"for"`

	WarningDuration types.Duration `json:"warningFor"`

	MaxAmounts       map[string]fixedpoint.Value `json:"maxAmounts"`
	LargeAmountAlert *LargeAmountAlertConfig     `json:"largeAmountAlert"`

	SlackNotify                bool             `json:"slackNotify"`
	SlackNotifyMentions        []string         `json:"slackNotifyMentions"`
	SlackNotifyThresholdAmount fixedpoint.Value `json:"slackNotifyThresholdAmount,omitempty"`

	deviationDetectors map[string]*detector.DeviationDetector[types.Balance]

	priceResolver *pricesolver.SimplePriceSolver

	sessions   map[string]*bbgo.ExchangeSession
	orderBooks map[string]*bbgo.ActiveOrderBook

	orderStore *core.OrderStore

	activeTransferNotificationLimiter *rate.Limiter
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	var cs []string

	for cur := range s.ExpectedBalances {
		cs = append(cs, cur)
	}

	return ID + strings.Join(s.PreferredSessions, "-") + strings.Join(cs, "-")
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
}

func (s *Strategy) CrossSubscribe(sessions map[string]*bbgo.ExchangeSession) {
}

func (s *Strategy) Defaults() error {
	if s.BalanceToleranceRange == fixedpoint.Zero {
		s.BalanceToleranceRange = fixedpoint.NewFromFloat(0.01)
	}

	if s.Duration == 0 {
		s.Duration = types.Duration(15 * time.Minute)
	}

	return nil
}

func (s *Strategy) Initialize() error {
	s.activeTransferNotificationLimiter = rate.NewLimiter(rate.Every(5*time.Minute), 1)
	s.deviationDetectors = make(map[string]*detector.DeviationDetector[types.Balance])
	return nil
}

func (s *Strategy) Validate() error {
	if s.PreferredQuoteCurrencies == nil {
		return errors.New("quoteCurrencies is not defined")
	}

	return nil
}

func (s *Strategy) aggregateBalances(
	ctx context.Context, sessions map[string]*bbgo.ExchangeSession,
) (totalBalances types.BalanceMap, sessionBalances map[string]types.BalanceMap) {
	totalBalances = make(types.BalanceMap)
	sessionBalances = make(map[string]types.BalanceMap)

	// iterate the sessions and record them
	for sessionName, session := range sessions {
		// update the account balances and the margin information
		if _, err := session.UpdateAccount(ctx); err != nil {
			log.WithError(err).Errorf("can not update account")
			return
		}

		account := session.GetAccount()
		balances := account.Balances()

		sessionBalances[sessionName] = balances
		totalBalances = totalBalances.Add(balances)
	}

	return totalBalances, sessionBalances
}

func (s *Strategy) detectActiveWithdraw(
	ctx context.Context,
	sessions map[string]*bbgo.ExchangeSession,
) (*types.Withdraw, error) {
	var err2 error
	until := time.Now()
	since := until.Add(-time.Hour * 24)
	for _, session := range sessions {
		transferService, ok := session.Exchange.(types.WithdrawHistoryService)
		if !ok {
			continue
		}

		withdraws, err := transferService.QueryWithdrawHistory(ctx, "", since, until)
		if err != nil {
			log.WithError(err).Errorf("unable to query withdraw history")
			err2 = err
			continue
		}

		for _, withdraw := range withdraws {
			log.Infof("checking withdraw status: %s", withdraw.String())
			switch withdraw.Status {
			case types.WithdrawStatusSent, types.WithdrawStatusProcessing, types.WithdrawStatusAwaitingApproval:
				return &withdraw, nil
			}
		}
	}

	return nil, err2
}

func (s *Strategy) detectActiveDeposit(
	ctx context.Context,
	sessions map[string]*bbgo.ExchangeSession,
) (*types.Deposit, error) {
	var err2 error
	until := time.Now()
	since := until.Add(-time.Hour * 24)
	for _, session := range sessions {
		transferService, ok := session.Exchange.(types.DepositHistoryService)
		if !ok {
			continue
		}

		deposits, err := transferService.QueryDepositHistory(ctx, "", since, until)
		if err != nil {
			log.WithError(err).Errorf("unable to query deposit history")
			err2 = err
			continue
		}

		for _, deposit := range deposits {
			log.Infof("checking deposit status: %s", deposit.String())
			switch deposit.Status {
			case types.DepositPending:
				return &deposit, nil
			}
		}
	}

	return nil, err2
}

func (s *Strategy) selectSessionForCurrency(
	ctx context.Context, sessions map[string]*bbgo.ExchangeSession, currency string, changeQuantity fixedpoint.Value,
) (*bbgo.ExchangeSession, *types.SubmitOrder) {
	var taker = s.UseTakerOrder
	var side types.SideType
	var quoteCurrencies []string
	if changeQuantity.Sign() > 0 {
		quoteCurrencies = s.PreferredQuoteCurrencies.Buy
		side = types.SideTypeBuy
	} else {
		quoteCurrencies = s.PreferredQuoteCurrencies.Sell
		side = types.SideTypeSell
	}

	for _, sessionName := range s.PreferredSessions {
		session := sessions[sessionName]

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
				log.WithError(err).Errorf("unable to query ticker on %s", symbol)
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

				quoteBalance, ok := session.Account.Balance(quoteCurrency)
				if !ok {
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
					log.Warnf("required quote amount %f > quote balance %v, skip", requiredQuoteAmount.Float64(), quoteBalance)
					continue
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

				maxAmount, ok := s.MaxAmounts[market.QuoteCurrency]
				if ok && requiredQuoteAmount.Compare(maxAmount) > 0 {
					log.Infof("adjusted required quote ammount %f %s by max amount %f %s", requiredQuoteAmount.Float64(), market.QuoteCurrency, maxAmount.Float64(), market.QuoteCurrency)

					requiredQuoteAmount = maxAmount
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

				baseBalance, ok := session.Account.Balance(baseCurrency)
				if !ok {
					continue
				}

				if q.Compare(baseBalance.Available) > 0 {
					log.Warnf("required base amount %f < available base balance %v, skip", q.Float64(), baseBalance)
					continue
				}

				maxAmount, ok := s.MaxAmounts[market.QuoteCurrency]
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

func (s *Strategy) CrossRun(ctx context.Context, _ bbgo.OrderExecutionRouter, sessions map[string]*bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()

	s.sessions = make(map[string]*bbgo.ExchangeSession)
	s.orderBooks = make(map[string]*bbgo.ActiveOrderBook)
	s.orderStore = core.NewOrderStore("")

	if err := dynamic.InitializeConfigMetrics(ID, instanceID, s); err != nil {
		return err
	}

	for currency, expectedValue := range s.ExpectedBalances {
		s.deviationDetectors[currency] = detector.NewDeviationDetector(
			types.Balance{Currency: currency, NetAsset: expectedValue}, // Expected value
			s.BalanceToleranceRange.Float64(),                          // Tolerance (1%)
			s.Duration.Duration(),                                      // Duration for sustained deviation
			func(b types.Balance) float64 {
				return b.Net().Float64()
			},
		)

		s.deviationDetectors[currency].SetLogger(log)
	}

	markets := types.MarketMap{}
	for _, sessionName := range s.PreferredSessions {
		session, ok := sessions[sessionName]
		if !ok {
			return fmt.Errorf("incorrect preferred session name: %s is not defined", sessionName)
		}

		s.orderStore.BindStream(session.UserDataStream)

		orderBook := bbgo.NewActiveOrderBook("")
		orderBook.BindStream(session.UserDataStream)
		s.orderBooks[sessionName] = orderBook

		s.sessions[sessionName] = session

		for _, market := range session.Markets() {
			markets.Add(market)
		}
	}

	s.priceResolver = pricesolver.NewSimplePriceResolver(markets)
	for _, session := range s.sessions {
		// bind on trade to update price
		session.UserDataStream.OnTradeUpdate(s.priceResolver.UpdateFromTrade)
	}

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		for n, session := range s.sessions {
			if ob, ok := s.orderBooks[n]; ok {
				_ = ob.GracefulCancel(ctx, session.Exchange)
			}
		}
	})

	go func() {
		s.align(ctx, s.sessions)

		ticker := time.NewTicker(s.Interval.Duration())
		defer ticker.Stop()

		for {
			select {

			case <-ctx.Done():
				return

			case <-ticker.C:
				s.align(ctx, s.sessions)
			}
		}
	}()

	return nil
}

func (s *Strategy) resetFaultBalanceRecords(currency string) {
	if d, ok := s.deviationDetectors[currency]; ok {
		d.ClearRecords()
	}
}

func (s *Strategy) recordBalance(totalBalances types.BalanceMap) {
	now := time.Now()

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

func (s *Strategy) align(ctx context.Context, sessions map[string]*bbgo.ExchangeSession) {
	for sessionName, session := range sessions {
		ob, ok := s.orderBooks[sessionName]
		if !ok {
			log.Errorf("orderbook on session %s not found", sessionName)
			return
		}
		if ok {
			if err := ob.GracefulCancel(ctx, session.Exchange); err != nil {
				log.WithError(err).Errorf("unable to cancel order")
			}
		}
	}

	pendingWithdraw, err := s.detectActiveWithdraw(ctx, sessions)
	if err != nil {
		log.WithError(err).Errorf("unable to check active transfers (withdraw)")
	} else if pendingWithdraw != nil {
		log.Warnf("found active transfer (%f %s withdraw), skip balance align check",
			pendingWithdraw.Amount.Float64(),
			pendingWithdraw.Asset)

		s.resetFaultBalanceRecords(pendingWithdraw.Asset)

		if s.activeTransferNotificationLimiter.Allow() {
			bbgo.Notify("Found active %s withdraw, skip balance align",
				pendingWithdraw.Asset,
				pendingWithdraw)
		}

		return
	}

	pendingDeposit, err := s.detectActiveDeposit(ctx, sessions)
	if err != nil {
		log.WithError(err).Errorf("unable to check active transfers (deposit)")
	} else if pendingDeposit != nil {
		log.Warnf("found active transfer (%f %s deposit), skip balance align check",
			pendingDeposit.Amount.Float64(),
			pendingDeposit.Asset)

		s.resetFaultBalanceRecords(pendingDeposit.Asset)

		if s.activeTransferNotificationLimiter.Allow() {
			bbgo.Notify("Found active %s deposit, skip balance align",
				pendingDeposit.Asset,
				pendingDeposit)
		}
		return
	}

	totalBalances, sessionBalances := s.aggregateBalances(ctx, sessions)
	_ = sessionBalances

	s.recordBalance(totalBalances)

	log.Debugf("checking all fault balance records...")

	amountQuoteCurrency := "USDT"
	if s.LargeAmountAlert != nil {
		amountQuoteCurrency = s.LargeAmountAlert.QuoteCurrency
	}

	for currency, expectedBalance := range s.ExpectedBalances {
		q := s.calculateRefillQuantity(totalBalances, currency, expectedBalance)
		quantity := q.Abs()

		price, ok := s.priceResolver.ResolvePrice(currency, amountQuoteCurrency)

		if !ok {
			log.Warnf("unable to resolve price for %s, skip alert checking", currency)
		}

		amount := price.Mul(quantity)
		if price.Sign() > 0 {
			log.Infof("resolved price for currency: %s, price: %f, quantity: %f, amount: %f", currency, price.Float64(), quantity.Float64(), amount.Float64())
		}

		log.Debugf("checking %s fault balance records...", currency)

		if d, ok := s.deviationDetectors[currency]; ok {
			should, sustainedDuration := d.ShouldFix()
			if sustainedDuration > 0 {
				log.Infof("%s sustained deviation for %s...", currency, sustainedDuration)
			}

			if s.WarningDuration > 0 && sustainedDuration >= s.WarningDuration.Duration() {
				if s.LargeAmountAlert != nil && price.Sign() > 0 {
					if amount.Compare(s.LargeAmountAlert.Amount) > 0 {
						bbgo.Notify(&CriticalBalanceDiscrepancyAlert{
							Warning:           true,
							SlackAlert:        s.LargeAmountAlert.Slack,
							QuoteCurrency:     s.LargeAmountAlert.QuoteCurrency,
							AlertAmount:       s.LargeAmountAlert.Amount,
							BaseCurrency:      currency,
							SustainedDuration: sustainedDuration,

							Price:    price,
							Delta:    q,
							Quantity: quantity,
							Amount:   amount,
						})
					}
				}
			}

			if !should {
				continue
			}
		}

		if s.LargeAmountAlert != nil && price.Sign() > 0 {
			if amount.Compare(s.LargeAmountAlert.Amount) > 0 {
				bbgo.Notify(&CriticalBalanceDiscrepancyAlert{
					SlackAlert:        s.LargeAmountAlert.Slack,
					QuoteCurrency:     s.LargeAmountAlert.QuoteCurrency,
					AlertAmount:       s.LargeAmountAlert.Amount,
					BaseCurrency:      currency,
					Delta:             q,
					SustainedDuration: s.Duration.Duration(),
					Price:             price,
					Quantity:          quantity,
					Amount:            amount,
				})
			}
		}

		selectedSession, submitOrder := s.selectSessionForCurrency(ctx, sessions, currency, q)
		if selectedSession != nil && submitOrder != nil {
			log.Infof("placing %s order on %s: %+v", submitOrder.Symbol, selectedSession.Name, submitOrder)

			bbgo.Notify("Aligning %s position on exchange session %s, delta: %f %s, expected balance: %f %s",
				currency, selectedSession.Name,
				q.Float64(), currency,
				expectedBalance.Float64(), currency,
				submitOrder)

			if s.DryRun {
				return
			}

			createdOrder, err := selectedSession.Exchange.SubmitOrder(ctx, *submitOrder)
			if err != nil {
				log.WithError(err).Errorf("can not place order: %+v", submitOrder)
				return
			}

			if createdOrder != nil {
				if ob, ok := s.orderBooks[selectedSession.Name]; ok {
					ob.Add(*createdOrder)
				} else {
					log.Errorf("orderbook %s not found", selectedSession.Name)
				}

				s.orderBooks[selectedSession.Name].Add(*createdOrder)
			}
		}
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
