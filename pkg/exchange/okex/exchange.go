package okex

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// OKB is the platform currency of OKEx, pre-allocate static string here
const OKB = "OKB"

var log = logrus.WithFields(logrus.Fields{
	"exchange": "okex",
})

type Exchange struct {
	key, secret, passphrase string

	client *okexapi.RestClient
}

func New(key, secret, passphrase string) *Exchange {
	client := okexapi.NewClient()
	client.Auth(key, secret, passphrase)

	return &Exchange{
		key:        key,
		secret:     secret,
		passphrase: passphrase,
		client:     client,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeOKEx
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	instruments, err := e.client.PublicDataService.NewGetInstrumentsRequest().
		InstrumentType(okexapi.InstrumentTypeSpot).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, instrument := range instruments {
		symbol := toGlobalSymbol(instrument.InstrumentID)
		market := types.Market{
			Symbol:      symbol,
			LocalSymbol: instrument.InstrumentID,

			QuoteCurrency: instrument.QuoteCurrency,
			BaseCurrency:  instrument.BaseCurrency,

			// convert tick size OKEx to precision
			PricePrecision:  int(-math.Log10(instrument.TickSize.Float64())),
			VolumePrecision: int(-math.Log10(instrument.LotSize.Float64())),

			// TickSize: OKEx's price tick, for BTC-USDT it's "0.1"
			TickSize: instrument.TickSize.Float64(),

			// Quantity step size, for BTC-USDT, it's "0.00000001"
			StepSize: instrument.LotSize.Float64(),

			// for BTC-USDT, it's "0.00001"
			MinQuantity: instrument.MinSize.Float64(),

			// OKEx does not offer minimal notional, use 1 USD here.
			MinNotional: 1.0,
			MinAmount:   1.0,
		}
		markets[symbol] = market
	}

	return markets, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	symbol = toLocalSymbol(symbol)

	marketTicker, err := e.client.MarketTicker(symbol)
	if err != nil {
		return nil, err
	}

	return toGlobalTicker(*marketTicker), nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbols ...string) (map[string]types.Ticker, error) {
	marketTickers, err := e.client.MarketTickers(okexapi.InstrumentTypeSpot)
	if err != nil {
		return nil, err
	}

	tickers := make(map[string]types.Ticker)
	for _, marketTicker := range marketTickers {
		symbol := toGlobalSymbol(marketTicker.InstrumentID)
		ticker := toGlobalTicker(marketTicker)
		tickers[symbol] = *ticker
	}

	if len(symbols) == 0 {
		return tickers, nil
	}

	selectedTickers := make(map[string]types.Ticker, len(symbols))
	for _, symbol := range symbols {
		if ticker, ok := tickers[symbol]; ok {
			selectedTickers[symbol] = ticker
		} else {
			return selectedTickers, fmt.Errorf("ticker of symbol %s not found", symbols)
		}
	}

	return selectedTickers, nil
}

func (e *Exchange) PlatformFeeCurrency() string {
	return OKB
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	accountBalance, err := e.client.AccountBalances()
	if err != nil {
		return nil, err
	}

	var account = types.Account{
		AccountType: "SPOT",
	}

	var balanceMap = toGlobalBalance(accountBalance)
	account.UpdateBalances(balanceMap)
	return &account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	accountBalances, err := e.client.AccountBalances()
	if err != nil {
		return nil, err
	}

	var balanceMap = toGlobalBalance(accountBalances)
	return balanceMap, nil
}

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	var reqs []*okexapi.PlaceOrderRequest
	for _, order := range orders {
		orderReq := e.client.TradeService.NewPlaceOrderRequest()

		orderType, err := toLocalOrderType(order.Type)
		if err != nil {
			return nil, err
		}

		orderReq.InstrumentID(toLocalSymbol(order.Symbol))
		orderReq.Side(toLocalSideType(order.Side))

		if len(order.QuantityString) > 0 {
			orderReq.Quantity(order.QuantityString)
		} else if order.Market.Symbol != "" {
			orderReq.Quantity(order.Market.FormatQuantity(order.Quantity))
		} else {
			orderReq.Quantity(strconv.FormatFloat(order.Quantity, 'f', 8, 64))
		}

		// set price field for limit orders
		switch order.Type {
		case types.OrderTypeStopLimit, types.OrderTypeLimit:
			if len(order.PriceString) > 0 {
				orderReq.Price(order.PriceString)
			} else if order.Market.Symbol != "" {
				orderReq.Price(order.Market.FormatPrice(order.Price))
			}
		}

		switch order.TimeInForce {
		case "FOK":
			orderReq.OrderType(okexapi.OrderTypeFOK)
		case "IOC":
			orderReq.OrderType(okexapi.OrderTypeIOC)
		default:
			orderReq.OrderType(orderType)
		}

		reqs = append(reqs, orderReq)
	}

	batchReq := e.client.TradeService.NewBatchPlaceOrderRequest()
	batchReq.Add(reqs...)
	orderHeads, err := batchReq.Do(ctx)
	if err != nil {
		return nil, err
	}

	for idx, orderHead := range orderHeads {
		orderID, err := strconv.ParseInt(orderHead.OrderID, 10, 64)
		if err != nil {
			return createdOrders, err
		}

		submitOrder := orders[idx]
		createdOrders = append(createdOrders, types.Order{
			SubmitOrder:      submitOrder,
			Exchange:         types.ExchangeOKEx,
			OrderID:          uint64(orderID),
			Status:           types.OrderStatusNew,
			ExecutedQuantity: 0,
			IsWorking:        true,
			CreationTime:     types.Time(time.Now()),
			UpdateTime:       types.Time(time.Now()),
			IsMargin:         false,
			IsIsolated:       false,
		})
	}

	return createdOrders, nil
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	instrumentID := toLocalSymbol(symbol)
	req := e.client.TradeService.NewGetPendingOrderRequest().InstrumentType(okexapi.InstrumentTypeSpot).InstrumentID(instrumentID)
	orderDetails, err := req.Do(ctx)
	if err != nil {
		return orders, err
	}

	orders, err = toGlobalOrders(orderDetails)
	return orders, err
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	var reqs []*okexapi.CancelOrderRequest
	for _, order := range orders {
		if len(order.Symbol) == 0 {
			return errors.New("symbol is required for canceling an okex order")
		}

		req := e.client.TradeService.NewCancelOrderRequest()
		req.InstrumentID(toLocalSymbol(order.Symbol))
		req.OrderID(strconv.FormatUint(order.OrderID, 10))

		if len(order.ClientOrderID) > 0 {
			req.ClientOrderID(order.ClientOrderID)
		}
		reqs = append(reqs, req)
	}

	batchReq := e.client.TradeService.NewBatchCancelOrderRequest()
	batchReq.Add(reqs...)
	_, err := batchReq.Do(ctx)
	return err
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e.client)
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	return nil, nil
}
