package bybit

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/exchange/bybit/bybitapi"
	"github.com/c9s/bbgo/pkg/types"
)

// https://bybit-exchange.github.io/docs/zh-TW/v5/rate-limit
// sharedRateLimiter indicates that the API belongs to the public API.
//
// The default order limiter apply 2 requests per second and a 2 initial bucket
// this includes QueryMarkets, QueryTicker
var (
	sharedRateLimiter = rate.NewLimiter(rate.Every(time.Second/2), 2)
	tradeRateLimiter  = rate.NewLimiter(rate.Every(time.Second/5), 5)

	log = logrus.WithFields(logrus.Fields{
		"exchange": "bybit",
	})
)

type Exchange struct {
	key, secret string
	client      *bybitapi.RestClient
}

func New(key, secret string) (*Exchange, error) {
	client, err := bybitapi.NewClient()
	if err != nil {
		return nil, err
	}

	if len(key) > 0 && len(secret) > 0 {
		client.Auth(key, secret)
	}

	return &Exchange{
		key: key,
		// pragma: allowlist nextline secret
		secret: secret,
		client: client,
	}, nil
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeBybit
}

// PlatformFeeCurrency returns empty string. The platform does not support "PlatformFeeCurrency" but instead charges
// fees using the native token.
func (e *Exchange) PlatformFeeCurrency() string {
	return ""
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	if err := sharedRateLimiter.Wait(ctx); err != nil {
		log.WithError(err).Errorf("markets rate limiter wait error")
		return nil, err
	}

	instruments, err := e.client.NewGetInstrumentsInfoRequest().Do(ctx)
	if err != nil {
		log.Warnf("failed to query instruments, err: %v", err)
		return nil, err
	}

	marketMap := types.MarketMap{}
	for _, s := range instruments.List {
		marketMap.Add(toGlobalMarket(s))
	}

	return marketMap, nil
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	if err := sharedRateLimiter.Wait(ctx); err != nil {
		log.WithError(err).Errorf("ticker rate limiter wait error")
		return nil, err
	}

	s, err := e.client.NewGetTickersRequest().Symbol(symbol).DoWithResponseTime(ctx)
	if err != nil {
		log.Warnf("failed to get tickers, symbol: %s, err: %v", symbol, err)
		return nil, err
	}

	if len(s.List) != 1 {
		log.Warnf("unexpected ticker length, exp: 1, got: %d", len(s.List))
		return nil, fmt.Errorf("unexpected ticker lenght, exp:1, got:%d", len(s.List))
	}

	ticker := toGlobalTicker(s.List[0], s.ClosedTime.Time())
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

	if err := sharedRateLimiter.Wait(ctx); err != nil {
		log.WithError(err).Errorf("ticker rate limiter wait error")
		return nil, err
	}
	allTickers, err := e.client.NewGetTickersRequest().DoWithResponseTime(ctx)
	if err != nil {
		log.Warnf("failed to get tickers, err: %v", err)
		return nil, err
	}

	for _, s := range allTickers.List {
		tickers[s.Symbol] = toGlobalTicker(s, allTickers.ClosedTime.Time())
	}

	return tickers, nil
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	cursor := ""
	for {
		req := e.client.NewGetOpenOrderRequest().Symbol(symbol)
		if len(cursor) != 0 {
			// the default limit is 20.
			req = req.Cursor(cursor)
		}

		if err = tradeRateLimiter.Wait(ctx); err != nil {
			log.WithError(err).Errorf("trade rate limiter wait error")
			return nil, err
		}
		res, err := req.Do(ctx)
		if err != nil {
			log.Warnf("failed to get open order, cursor: %s, err: %v", cursor, err)
			return nil, err
		}

		for _, order := range res.List {
			order, err := toGlobalOrder(order)
			if err != nil {
				log.Warnf("failed to convert order, err: %v", err)
				return nil, err
			}

			orders = append(orders, *order)
		}

		if len(res.NextPageCursor) == 0 {
			break
		}
		cursor = res.NextPageCursor
	}

	return orders, nil
}
