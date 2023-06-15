package skeleton

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// ID is the unique strategy ID, it needs to be in all lower case
// For example, grid strategy uses "grid"
const ID = "skeleton"

// log is a logrus.Entry that will be reused.
// This line attaches the strategy field to the logger with our ID, so that the logs from this strategy will be tagged with our ID
var log = logrus.WithField("strategy", ID)

var ten = fixedpoint.NewFromInt(10)

// init is a special function of golang, it will be called when the program is started
// importing this package will trigger the init function call.
func init() {
	// Register our struct type to BBGO
	// Note that you don't need to field the fields.
	// BBGO uses reflect to parse your type information.
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// State is a struct contains the information that we want to keep in the persistence layer,
// for example, redis or json file.
type State struct {
	Counter int `json:"counter,omitempty"`
}

// Strategy is a struct that contains the settings of your strategy.
// These settings will be loaded from the BBGO YAML config file "bbgo.yaml" automatically.
type Strategy struct {
	Symbol string `json:"symbol"`

	// State is a state of your strategy
	// When BBGO shuts down, everything in the memory will be dropped
	// If you need to store something and restore this information back,
	// Simply define the "persistence" tag
	State *State `persistence:"state"`
}

// ID should return the identity of this strategy
func (s *Strategy) ID() string {
	return ID
}

// InstanceID returns the identity of the current instance of this strategy.
// You may have multiple instance of a strategy, with different symbols and settings.
// This value will be used for persistence layer to separate the storage.
//
// Run:
//    redis-cli KEYS "*"
//
// And you will see how this instance ID is used in redis.
func (s *Strategy) InstanceID() string {
	return ID + ":" + s.Symbol
}

// Subscribe method subscribes specific market data from the given session.
// Before BBGO is connected to the exchange, we need to collect what we want to subscribe.
// Here the strategy needs kline data, so it adds the kline subscription.
func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// We want 1m kline data of the symbol
	// It will be BTCUSDT 1m if our s.Symbol is BTCUSDT
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

// This strategy simply spent all available quote currency to buy the symbol whenever kline gets closed
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	// Initialize the default value for state
	if s.State == nil {
		s.State = &State{Counter: 1}
	}

	indicators := session.StandardIndicatorSet(s.Symbol)
	atr := indicators.ATR(types.IntervalWindow{
		Interval: types.Interval1m,
		Window:   14,
	})

	// To get the market information from the current session
	// The market object provides the precision, MoQ (minimal of quantity) information
	market, ok := session.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("market %s not found", s.Symbol)
	}

	// here we define a kline callback
	// when a kline is closed, we will do something
	callback := func(kline types.KLine) {
		// get the latest ATR value from the indicator object that we just defined.
		atrValue := atr.Last(0)
		log.Infof("atr %f", atrValue)

		// Update our counter and sync the changes to the persistence layer on time
		// If you don't do this, BBGO will sync it automatically when BBGO shuts down.
		s.State.Counter++
		bbgo.Sync(ctx, s)

		// To check if we have the quote balance
		// When symbol = "BTCUSDT", the quote currency is USDT
		// We can get this information from the market object
		quoteBalance, ok := session.GetAccount().Balance(market.QuoteCurrency)
		if !ok {
			// if not ok, it means we don't have this currency in the account
			return
		}

		// For each balance, we have Available and Locked balance.
		// balance.Available is the balance you can use to place an order.
		// Note that the available balance is a fixed-point object, so you can not compare it with integer directly.
		// Instead, you should call valueA.Compare(valueB)
		quantityAmount := quoteBalance.Available
		if quantityAmount.Sign() <= 0 || quantityAmount.Compare(ten) < 0 {
			return
		}

		// Call LastPrice(symbol) If you need to get the latest price
		// Note this last price is updated by the closed kline
		currentPrice, ok := session.LastPrice(s.Symbol)
		if !ok {
			return
		}

		// totalQuantity = quantityAmount / currentPrice
		totalQuantity := quantityAmount.Div(currentPrice)

		// Place a market order to the exchange
		createdOrders, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:   kline.Symbol,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Price:    currentPrice,
			Quantity: totalQuantity,
		})

		if err != nil {
			log.WithError(err).Error("submit order error")
		}

		log.Infof("createdOrders: %+v", createdOrders)

		// send notification to slack or telegram if you have configured it
		bbgo.Notify("order created")
	}

	// register our kline event handler
	session.MarketDataStream.OnKLineClosed(callback)

	// if you need to do something when the user data stream is ready
	// note that you only receive order update, trade update, balance update when the user data stream is connect.
	session.UserDataStream.OnStart(func() {
		log.Infof("connected")
	})

	return nil
}
