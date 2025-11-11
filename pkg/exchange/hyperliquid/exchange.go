package hyperliquid

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
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

// clientOrderIdRegex is 128-bit hex string starting with 0x and followed by 32 hex characters
var clientOrderIdRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{32}$`)

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

	balances, err := e.querySpotAccountBalance(ctx)
	if err != nil {
		return nil, err
	}

	account := types.NewAccount()
	account.UpdateBalances(toGlobalBalance(balances))

	return account, nil
}

func (e *Exchange) QueryAccountBalances(ctx context.Context) (types.BalanceMap, error) {
	balances, err := e.querySpotAccountBalance(ctx)
	if err != nil {
		return nil, err
	}

	return toGlobalBalance(balances), nil
}

func (e *Exchange) querySpotAccountBalance(ctx context.Context) (*hyperapi.Account, error) {
	if err := restSharedLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("account rate limiter wait error: %w", err)
	}

	return e.client.NewGetAccountBalanceRequest().User(e.client.UserAddress()).Do(ctx)
}

func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (createdOrder *types.Order, err error) {
	// Validate order parameters
	if len(order.Market.Symbol) == 0 {
		return nil, fmt.Errorf("order.Market.Symbol is required: %+v", order)
	}
	if order.Quantity.IsZero() {
		return nil, fmt.Errorf("order.Quantity is required: %+v", order)
	}

	// Build order request
	_, assetIdx := e.getLocalSymbol(order.Symbol)
	reqOrder := hyperapi.Order{
		Asset:      strconv.Itoa(assetIdx),
		IsBuy:      order.Side == types.SideTypeBuy,
		Size:       order.Quantity.String(),
		Price:      order.Price.String(),
		ReduceOnly: order.ReduceOnly,
	}

	// Validate and set client order ID if provided
	if len(order.ClientOrderID) > 0 {
		if !clientOrderIdRegex.MatchString(order.ClientOrderID) {
			return nil, fmt.Errorf("client order id should be a 128-bit hex string starting with 0x and followed by 32 hex characters: %s", order.ClientOrderID)
		}
		reqOrder.ClientOrderID = &order.ClientOrderID
	}

	// Set order type and time in force
	switch order.Type {
	case types.OrderTypeLimit, types.OrderTypeLimitMaker:
		tif := hyperapi.TimeInForceGTC
		switch order.TimeInForce {
		case types.TimeInForceIOC:
			tif = hyperapi.TimeInForceIOC
		case types.TimeInForceALO:
			tif = hyperapi.TimeInForceALO
		}
		reqOrder.OrderType = hyperapi.OrderType{Limit: hyperapi.LimitOrderType{Tif: tif}}

	case types.OrderTypeMarket:
		reqOrder.OrderType = hyperapi.OrderType{
			Trigger: hyperapi.TriggerOrderType{
				IsMarket:  true,
				TriggerPx: "0",
			},
		}

	case types.OrderTypeStopLimit:
		reqOrder.OrderType = hyperapi.OrderType{
			Trigger: hyperapi.TriggerOrderType{
				IsMarket:  false,
				TriggerPx: order.StopPrice.String(),
				Tpsl:      "sl",
			},
		}

	case types.OrderTypeStopMarket, types.OrderTypeTakeProfitMarket:
		tpsl := "sl"
		if order.Type == types.OrderTypeTakeProfitMarket {
			tpsl = "tp"
		}
		reqOrder.OrderType = hyperapi.OrderType{
			Trigger: hyperapi.TriggerOrderType{
				IsMarket:  true,
				TriggerPx: order.StopPrice.String(),
				Tpsl:      tpsl,
			},
		}

	default:
		return nil, fmt.Errorf("unsupported order type: %v", order.Type)
	}

	if err := restSharedLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("submit order rate limiter wait error: %w", err)
	}

	// Submit order to exchange
	resp, err := e.client.NewPlaceOrderRequest().Orders([]hyperapi.Order{reqOrder}).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to place order: %w", err)
	}

	// Validate response
	if resp == nil || len(resp.Statuses) == 0 {
		return nil, fmt.Errorf("invalid order response: resp=%v, statuses=%d", resp != nil, len(resp.Statuses))
	}

	status := resp.Statuses[0]
	if status.Error != "" {
		return nil, fmt.Errorf("order submission error: %s", status.Error)
	}

	// Extract order information from response
	var (
		orderID          int
		orderStatus      = types.OrderStatusNew
		executedQuantity = fixedpoint.Zero
	)

	if status.Resting != nil {
		orderID = status.Resting.Oid
	} else if status.Filled != nil {
		orderID = status.Filled.Oid
		orderStatus = types.OrderStatusFilled
		executedQuantity = status.Filled.TotalSz
	}

	// Build and return order object
	timeNow := time.Now()
	return &types.Order{
		SubmitOrder:      order,
		Exchange:         types.ExchangeHyperliquid,
		OrderID:          uint64(orderID),
		Status:           orderStatus,
		ExecutedQuantity: executedQuantity,
		IsWorking:        true,
		CreationTime:     types.Time(timeNow),
		UpdateTime:       types.Time(timeNow),
	}, nil
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
	if err := restSharedLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("query k line rate limiter wait error: %w", err)
	}

	localSymbol, _ := e.getLocalSymbol(symbol)
	intervalParam, err := toLocalInterval(interval)
	if err != nil {
		return nil, fmt.Errorf("failed to get interval: %w", err)
	}

	candleReq := hyperapi.CandleRequest{
		Interval: intervalParam,
		Coin:     localSymbol,
	}

	if options.StartTime != nil {
		candleReq.StartTime = options.StartTime.UnixMilli()
	}

	if options.EndTime != nil {
		candleReq.EndTime = options.EndTime.UnixMilli()
	}

	candles, err := e.client.NewGetCandlesRequest().CandleRequest(candleReq).Do(ctx)
	if err != nil {
		return nil, err
	}
	var kLines []types.KLine
	for _, candle := range candles {
		kLines = append(kLines, kLineToGlobal(candle, interval, symbol))
	}

	return kLines, nil
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

func (e *Exchange) getLocalSymbol(symbol string) (string, int) {
	if e.IsFutures {
		return toLocalFuturesSymbol(symbol)
	}

	return toLocalSpotSymbol(symbol)
}
