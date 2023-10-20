package bitget

import (
	"context"
	"math"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "bitget"

const PlatformToken = "BGB"

var log = logrus.WithFields(logrus.Fields{
	"exchange": ID,
})

type Exchange struct {
	key, secret, passphrase string

	client *bitgetapi.RestClient
}

func New(key, secret, passphrase string) *Exchange {
	client := bitgetapi.NewClient()

	if len(key) > 0 && len(secret) > 0 {
		client.Auth(key, secret, passphrase)
	}

	return &Exchange{
		key:        key,
		secret:     secret,
		passphrase: passphrase,
		client:     client,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBitget
}

func (e *Exchange) PlatformFeeCurrency() string {
	return PlatformToken
}

func (e *Exchange) NewStream() types.Stream {
	// TODO implement me
	panic("implement me")
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	// TODO implement me
	req := e.client.NewGetSymbolsRequest()
	symbols, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, s := range symbols {
		symbol := toGlobalSymbol(s.SymbolName)
		markets[symbol] = types.Market{
			Symbol:          s.SymbolName,
			LocalSymbol:     s.Symbol,
			PricePrecision:  s.PriceScale,
			VolumePrecision: s.QuantityScale,
			QuoteCurrency:   s.QuoteCoin,
			BaseCurrency:    s.BaseCoin,
			MinNotional:     s.MinTradeUSDT,
			MinAmount:       s.MinTradeUSDT,
			MinQuantity:     s.MinTradeAmount,
			MaxQuantity:     s.MaxTradeAmount,
			StepSize:        fixedpoint.NewFromFloat(math.Pow10(-s.QuantityScale)),
			TickSize:        fixedpoint.NewFromFloat(math.Pow10(-s.PriceScale)),
			MinPrice:        fixedpoint.Zero,
			MaxPrice:        fixedpoint.Zero,
		}
	}

	return markets, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	req := e.client.NewGetTickerRequest()
	req.Symbol(symbol)
	ticker, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	return &types.Ticker{
		Time:   ticker.Ts.Time(),
		Volume: ticker.BaseVol,
		Last:   ticker.Close,
		Open:   ticker.OpenUtc0,
		High:   ticker.High24H,
		Low:    ticker.Low24H,
		Buy:    ticker.BuyOne,
		Sell:   ticker.SellOne,
	}, nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	// TODO implement me
	panic("implement me")
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	// TODO implement me
	panic("implement me")
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	req := e.client.NewGetAccountAssetsRequest()
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	bals := types.BalanceMap{}
	for _, asset := range resp {
		b := toGlobalBalance(asset)
		bals[asset.CoinName] = b
	}

	account := types.NewAccount()
	account.UpdateBalances(bals)
	return account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	// TODO implement me
	panic("implement me")
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	// TODO implement me
	panic("implement me")
}
