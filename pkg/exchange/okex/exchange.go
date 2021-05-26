package okex

import (
	"context"
	"fmt"
	"math"

	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/types"
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

	selectedTickers := make(map[string]types.Ticker)
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
	balanceSummaries, err := e.client.AccountBalances()
	if err != nil {
		return nil, err
	}

	var account = types.Account{
		AccountType: "SPOT",
	}

	var balanceMap = toGlobalBalance(balanceSummaries)
	account.UpdateBalances(balanceMap)
	return &account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	balanceSummaries, err := e.client.AccountBalances()
	if err != nil {
		return nil, err
	}

	var balanceMap = toGlobalBalance(balanceSummaries)
	return balanceMap, nil
}

func (e *Exchange) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (createdOrders types.OrderSlice, err error) {
	panic("implement me")
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	panic("implement me")
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	panic("implement me")
}
