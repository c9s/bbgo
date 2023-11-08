package bitget

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi"
	v2 "github.com/c9s/bbgo/pkg/exchange/bitget/bitgetapi/v2"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	ID = "bitget"

	PlatformToken = "BGB"

	queryOpenOrdersLimit = 100
)

var log = logrus.WithFields(logrus.Fields{
	"exchange": ID,
})

var (
	// queryMarketRateLimiter has its own rate limit. https://bitgetlimited.github.io/apidoc/en/spot/#get-symbols
	queryMarketRateLimiter = rate.NewLimiter(rate.Every(time.Second/10), 5)
	// queryAccountRateLimiter has its own rate limit. https://bitgetlimited.github.io/apidoc/en/spot/#get-account-assets
	queryAccountRateLimiter = rate.NewLimiter(rate.Every(time.Second/5), 5)
	// queryTickerRateLimiter has its own rate limit. https://bitgetlimited.github.io/apidoc/en/spot/#get-single-ticker
	queryTickerRateLimiter = rate.NewLimiter(rate.Every(time.Second/10), 5)
	// queryTickersRateLimiter has its own rate limit. https://bitgetlimited.github.io/apidoc/en/spot/#get-all-tickers
	queryTickersRateLimiter = rate.NewLimiter(rate.Every(time.Second/10), 5)
	// queryOpenOrdersRateLimiter has its own rate limit. https://www.bitget.com/zh-CN/api-doc/spot/trade/Get-Unfilled-Orders
	queryOpenOrdersRateLimiter = rate.NewLimiter(rate.Every(time.Second/10), 5)
)

type Exchange struct {
	key, secret, passphrase string

	client   *bitgetapi.RestClient
	v2Client *v2.Client
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
		v2Client:   v2.NewClient(client),
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
	if err := queryMarketRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("markets rate limiter wait error: %w", err)
	}

	req := e.client.NewGetSymbolsRequest()
	symbols, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, s := range symbols {
		symbol := toGlobalSymbol(s.SymbolName)
		markets[symbol] = toGlobalMarket(s)
	}

	return markets, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	if err := queryTickerRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("ticker rate limiter wait error: %w", err)
	}

	req := e.client.NewGetTickerRequest()
	req.Symbol(symbol)
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query ticker: %w", err)
	}

	ticker := toGlobalTicker(*resp)
	return &ticker, nil
}

func (e *Exchange) QueryTickers(ctx context.Context, symbols ...string) (map[string]types.Ticker, error) {
	tickers := map[string]types.Ticker{}
	if len(symbols) > 0 {
		for _, s := range symbols {
			t, err := e.QueryTicker(ctx, s)
			if err != nil {
				return nil, err
			}

			tickers[s] = *t
		}

		return tickers, nil
	}

	if err := queryTickersRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("tickers rate limiter wait error: %w", err)
	}

	resp, err := e.client.NewGetAllTickersRequest().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query tickers: %w", err)
	}

	for _, s := range resp {
		tickers[s.Symbol] = toGlobalTicker(s)
	}

	return tickers, nil
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	// TODO implement me
	panic("implement me")
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	bals, err := e.QueryAccountBalances(ctx)
	if err != nil {
		return nil, err
	}

	account := types.NewAccount()
	account.UpdateBalances(bals)
	return account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	if err := queryAccountRateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("account rate limiter wait error: %w", err)
	}

	req := e.client.NewGetAccountAssetsRequest()
	resp, err := req.Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query account assets: %w", err)
	}

	bals := types.BalanceMap{}
	for _, asset := range resp {
		b := toGlobalBalance(asset)
		bals[asset.CoinName] = b
	}

	return bals, nil
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	// TODO implement me
	panic("implement me")
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	var nextCursor types.StrInt64
	for {
		if err := queryOpenOrdersRateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("open order rate limiter wait error: %w", err)
		}

		req := e.v2Client.NewGetUnfilledOrdersRequest().
			Symbol(symbol).
			Limit(strconv.FormatInt(queryOpenOrdersLimit, 10))
		if nextCursor != 0 {
			req.IdLessThan(strconv.FormatInt(int64(nextCursor), 10))
		}

		openOrders, err := req.Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query open orders: %w", err)
		}

		for _, o := range openOrders {
			order, err := unfilledOrderToGlobalOrder(o)
			if err != nil {
				return nil, fmt.Errorf("failed to convert order, err: %v", err)
			}

			orders = append(orders, *order)
		}

		orderLen := len(openOrders)
		// a defensive programming to ensure the length of order response is expected.
		if orderLen > queryOpenOrdersLimit {
			return nil, fmt.Errorf("unexpected open orders length %d", orderLen)
		}

		if orderLen < queryOpenOrdersLimit {
			break
		}
		nextCursor = openOrders[orderLen-1].OrderId
	}

	return orders, nil
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	// TODO implement me
	panic("implement me")
}
