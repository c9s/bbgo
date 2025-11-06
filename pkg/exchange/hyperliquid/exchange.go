package hyperliquid

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/hyperliquid/hyperapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

const (
	HYPE = "HYPE"

	ID types.ExchangeName = "hyperliquid"
)

// REST requests share an aggregated weight limit of 1200 per minute.
var restSharedLimiter = rate.NewLimiter(rate.Every(50*time.Millisecond), 1)

var log = logrus.WithFields(logrus.Fields{
	"exchange": ID,
})

type Exchange struct {
	types.FuturesSettings

	secret string

	client *hyperapi.Client
}

func New(secret, vaultAddress string) *Exchange {
	client := hyperapi.NewClient()
	if len(secret) > 0 {
		client.Auth(secret)
	}

	if len(vaultAddress) > 0 {
		client.SetVaultAddress(vaultAddress)
	}
	return &Exchange{
		secret: secret,
		client: client,
	}
}

func (e *Exchange) Name() types.ExchangeName {
	return types.ExchangeHyperliquid
}

func (e *Exchange) PlatformFeeCurrency() string {
	return HYPE
}

func (e *Exchange) Initialize(ctx context.Context) error {
	if err := e.syncSymbolsToMap(ctx); err != nil {
		return err
	}

	return nil
}

func (e *Exchange) QueryMarkets(ctx context.Context) (types.MarketMap, error) {
	if err := restSharedLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("markets rate limiter wait error: %w", err)
	}

	if e.IsFutures {
		return e.queryFuturesMarkets(ctx)
	}

	meta, err := e.client.NewSpotGetMetaRequest().Do(ctx)
	if err != nil {
		return nil, err
	}

	markets := types.MarketMap{}
	for _, s := range meta.Universe {
		market := toGlobalSpotMarket(s, meta.Tokens)
		markets.Add(market)
	}

	return markets, nil
}

func (e *Exchange) QueryAccount(ctx context.Context) (*types.Account, error) {
	if e.IsFutures {
		return e.queryFuturesAccount(ctx)
	}

	spotAccount, err := e.client.NewGetAccountBalanceRequest().User(e.client.UserAddress()).Do(ctx)
	if err != nil {
		return nil, err
	}

	balances := toGlobalBalance(spotAccount)
	account := types.NewAccount()
	account.UpdateBalances(balances)

	return account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	// TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	// TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) QueryOpenOrders(ctx context.Context, symbol string) (orders []types.Order, err error) {
	// TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) CancelOrders(ctx context.Context, orders ...types.Order) error {
	//TODO implement
	return fmt.Errorf("not implemented")
}

func (e *Exchange) NewStream() types.Stream {
	return NewStream(e.client, e)
}

func (e *Exchange) QueryTicker(ctx context.Context, symbol string) (*types.Ticker, error) {
	//TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) QueryTickers(ctx context.Context, symbol ...string) (map[string]types.Ticker, error) {
	//TODO implement
	return nil, fmt.Errorf("not implemented")
}

func (e *Exchange) QueryKLines(ctx context.Context, symbol string, interval types.Interval, options types.KLineQueryOptions) ([]types.KLine, error) {
	//TODO implement
	return nil, fmt.Errorf("not implemented")
}

// DefaultFeeRates returns the hyperliquid base fee schedule
// See futures fee at: https://hyperliquid.gitbook.io/hyperliquid-docs/trading/fees
func (e *Exchange) DefaultFeeRates() types.ExchangeFee {
	if e.IsFutures {
		return types.ExchangeFee{
			MakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.0150), // 0.0150%
			TakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.0450), // 0.0450%
		}
	}

	return types.ExchangeFee{
		MakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.040), // 0.040%
		TakerFeeRate: fixedpoint.NewFromFloat(0.01 * 0.070), // 0.070%
	}
}

func (e *Exchange) SupportedInterval() map[types.Interval]int {
	return SupportedIntervals
}
func (e *Exchange) IsSupportedInterval(interval types.Interval) bool {
	_, ok := SupportedIntervals[interval]
	return ok
}

func (e *Exchange) syncSymbolsToMap(ctx context.Context) error {
	markets, err := e.QueryMarkets(ctx)
	if err != nil {
		return err
	}

	symbolMap := func() *sync.Map {
		if e.IsFutures {
			return &futuresSymbolSyncMap
		}
		return &spotSymbolSyncMap
	}()

	// Mark all valid symbols
	existing := make(map[string]struct{}, len(markets))
	for symbol, market := range markets {
		symbolMap.Store(symbol, market.LocalSymbol)
		existing[symbol] = struct{}{}
	}

	// Remove outdated symbols
	symbolMap.Range(func(key, _ interface{}) bool {
		if symbol, ok := key.(string); !ok || existing[symbol] == struct{}{} {
			return true
		} else if _, found := existing[symbol]; !found {
			symbolMap.Delete(symbol)
		}
		return true
	})

	return nil
}
