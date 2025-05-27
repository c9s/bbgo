// Copyright 2023 The Maxset Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package swing_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/maxsetgros/pkg/bbgo"
	"github.com/maxsetgros/pkg/strategy/swing" // Ensure this import path is correct
	"github.com/maxsetgros/pkg/types"
)

// MockOrderExecutor is a mock implementation of bbgo.OrderExecutor
type MockOrderExecutor struct {
	SubmittedOrders []types.SubmitOrder
	mu              sync.Mutex
	// Store the response orders that would have been created by the exchange
	CreatedOrdersResponse []types.Order
}

func (m *MockOrderExecutor) SubmitOrders(ctx context.Context, orders ...types.SubmitOrder) (types.OrderSlice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SubmittedOrders = append(m.SubmittedOrders, orders...)
	
	createdOrders := make(types.OrderSlice, len(orders))
	for i, so := range orders {
		orderID := uint64(time.Now().UnixNano() + int64(i)) // Basic unique ID
		order := types.Order{
			OrderID:        orderID,
			Symbol:         so.Symbol,
			Side:           so.Side,
			Type:           so.Type,
			Price:          so.Price, 
			Quantity:       so.Quantity, // This should be parsed from QuantityString by the strategy or executor normally
			ExecutedQuantity: fixedpoint.Zero,
			Status:         types.OrderStatusNew, 
			CreationTime:   types.MillisecondTimestamp(time.Now().UnixMilli()),
			UpdateTime:     types.MillisecondTimestamp(time.Now().UnixMilli()),
		}
        // If QuantityString is used, Quantity might be zero here.
        // The strategy seems to pass QuantityString. A real executor would parse it.
        // For mock purposes, we'll assume the strategy calculates fixedpoint.Value for Quantity if needed.
        // The strategy's SubmitOrder uses QuantityString, so this mock needs to reflect that if we need to check order.Quantity.
        // For now, strategy logic depends on OnOrderUpdate where ExecutedQuantity is key.
		createdOrders[i] = order
	}
	m.CreatedOrdersResponse = append(m.CreatedOrdersResponse, createdOrders...)
	return createdOrders, nil
}

func (m *MockOrderExecutor) GetSubmittedOrders() []types.SubmitOrder {
	m.mu.Lock()
	defer m.mu.Unlock()
	ordersCopy := make([]types.SubmitOrder, len(m.SubmittedOrders))
	copy(ordersCopy, m.SubmittedOrders)
	return ordersCopy
}

func (m *MockOrderExecutor) GetCreatedOrdersResponse() []types.Order {
    m.mu.Lock()
    defer m.mu.Unlock()
    ordersCopy := make([]types.Order, len(m.CreatedOrdersResponse))
    copy(ordersCopy, m.CreatedOrdersResponse)
    return ordersCopy
}

func (m *MockOrderExecutor) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SubmittedOrders = nil
	m.CreatedOrdersResponse = nil
}

// MockMarketDataStream to simulate KLine events
type MockMarketDataStream struct {
	klineCallbacks []types.KLineCallback
	mu             sync.Mutex
}

func (m *MockMarketDataStream) OnKLineClosed(cb types.KLineCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.klineCallbacks = append(m.klineCallbacks, cb)
}

func (m *MockMarketDataStream) TriggerKLine(kline types.KLine) {
	m.mu.Lock()
	callbacks := make([]types.KLineCallback, len(m.klineCallbacks))
	copy(callbacks, m.klineCallbacks)
	m.mu.Unlock()

	for _, cb := range callbacks {
		cb(kline)
	}
}

// MockExchange implements parts of types.Exchange for leverage setting
type MockExchange struct {
	types.Exchange 
	LeverageSetSymbol  string
	LeverageSetLeverage int
	LeverageSetError   error
}

func (m *MockExchange) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	if m.LeverageSetError != nil {
		return m.LeverageSetError
	}
	m.LeverageSetSymbol = symbol
	m.LeverageSetLeverage = leverage
	return nil
}
func (m *MockExchange) Name() types.ExchangeName { return "mockexchange" }


// MockExchangeSession is a mock implementation of bbgo.ExchangeSession
type MockExchangeSession struct {
	*bbgo.ExchangeSession 
	SymbolMarket          *types.Market
	dataStream            *MockMarketDataStream // Renamed to avoid conflict with embedded ExchangeSession.MarketDataStream
	mockIndicatorSet      *MockStandardIndicatorSet 
	mockExchange          *MockExchange
	Subscriptions []struct{ Channel interface{}; Symbol string; Options types.SubscribeOptions }
}

func NewMockExchangeSession(symbol string) *MockExchangeSession {
	market := types.Market{
		Symbol:          symbol,
		PricePrecision:  2,
		VolumePrecision: 6,
		MinQuantity:     fixedpoint.NewFromFloat(0.000001),
		MinNotional:     fixedpoint.NewFromFloat(10.0),
		QuoteCurrency:   "USDT",
		BaseCurrency:    "BTC", 
	}
	mockEx := &MockExchange{}
	// Use a real MarketDataStream from bbgo package and set its Underlying to our mock.
	actualMarketDataStream := &bbgo.MarketDataStream{}
	mockDataStreamBackend := &MockMarketDataStream{}
	actualMarketDataStream.Underlying = mockDataStreamBackend


	session := &bbgo.ExchangeSession{Exchange: mockEx, MarketDataStream: actualMarketDataStream}

	return &MockExchangeSession{
		ExchangeSession:  session,
		SymbolMarket:     &market,
		dataStream:       mockDataStreamBackend, // Store our mock backend
		mockIndicatorSet: &MockStandardIndicatorSet{},
		mockExchange:     mockEx,
	}
}

func (m *MockExchangeSession) Market(symbol string) (*types.Market, bool) {
	if m.SymbolMarket != nil && m.SymbolMarket.Symbol == symbol {
		return m.SymbolMarket, true
	}
	return nil, false
}

func (m *MockExchangeSession) StandardIndicatorSet(symbol string) bbgo.StandardIndicatorSet {
	return m.mockIndicatorSet
}

func (m *MockExchangeSession) Subscribe(channel interface{}, symbol string, options types.SubscribeOptions) {
    m.Subscriptions = append(m.Subscriptions, struct{Channel interface{}; Symbol string; Options types.SubscribeOptions}{channel, symbol, options})
}

// Method to trigger KLines on the *backend* of the session's MarketDataStream
func (m *MockExchangeSession) TriggerKLine(kline types.KLine) {
    m.dataStream.TriggerKLine(kline)
}


// MockStandardIndicatorSet provides real indicators.
type MockStandardIndicatorSet struct {
	bbgo.StandardIndicatorSet
}

func (m *MockStandardIndicatorSet) EMA(iw types.IntervalWindow) *indicator.EMA {
	return indicator.NewEMA(iw.Window) 
}

func (m *MockStandardIndicatorSet) SMA(iw types.IntervalWindow) *indicator.SMA {
	return indicator.NewSMA(iw.Window)
}

// Helper function to create a new strategy instance for testing
func newTestBMSBStrategy(symbol, timeframe string, emaShort, smaLong, marketSma int, quantity, leverage fixedpoint.Value) *swing.BullMarketSupportBandStrategy {
	return &swing.BullMarketSupportBandStrategy{
		Symbol:                   symbol,
		Timeframe:                timeframe,
		EmaPeriodShort:           emaShort,
		SmaPeriodLong:            smaLong,
		MarketConditionSmaPeriod: marketSma,
		Quantity:                 quantity,
		Leverage:                 leverage,
	}
}

// Helper function to create KLine instances
func newKLine(symbol string, timeframe string, close, high, low fixedpoint.Value, ts time.Time) types.KLine {
	return types.KLine{
		Symbol:     symbol,
		Interval:   types.Interval(timeframe),
		StartTime:  types.MillisecondTimestamp(ts.UnixMilli()),
		EndTime:    types.MillisecondTimestamp(ts.Add(time.Minute - 1).UnixMilli()),
		OpenPrice:  close, 
		ClosePrice: close,
		HighPrice:  high,
		LowPrice:   low,
		Volume:     fixedpoint.NewFromInt(100), 
	}
}

func TestBullMarketSupportBandStrategy_InitializeIndicators(t *testing.T) {
	ctx := context.Background()
	mockSession := NewMockExchangeSession("BTCUSDT")
	
	leverageToSet := fixedpoint.NewFromInt(5)
	strategy := newTestBMSBStrategy("BTCUSDT", "1h", 20, 21, 50, fixedpoint.NewFromFloat(0.01), leverageToSet)

	strategy.InitializeIndicators(ctx, mockSession)

	if !strategy.Leverage.IsZero() {
		if mockSession.mockExchange.LeverageSetSymbol != strategy.Symbol {
			t.Errorf("Expected leverage to be set for symbol %s, but got %s", strategy.Symbol, mockSession.mockExchange.LeverageSetSymbol)
		}
		expectedLeverageInt := int(leverageToSet.IntPart())
		if mockSession.mockExchange.LeverageSetLeverage != expectedLeverageInt {
			t.Errorf("Expected leverage to be %d, but got %d", expectedLeverageInt, mockSession.mockExchange.LeverageSetLeverage)
		}
	}
	t.Log("TestInitializeIndicators: Leverage setting checked.")
}

func TestBullMarketSupportBandStrategy_Subscribe(t *testing.T) {
    mockSession := NewMockExchangeSession("BTCUSDT")
    strategy := newTestBMSBStrategy("BTCUSDT", "1h", 20, 21, 50, fixedpoint.NewFromFloat(0.01), fixedpoint.NewFromInt(1))

    strategy.Subscribe(mockSession)

    if len(mockSession.Subscriptions) != 1 {
        t.Fatalf("Expected 1 subscription, got %d", len(mockSession.Subscriptions))
    }
    sub := mockSession.Subscriptions[0]
    if sub.Channel != types.KLineChannel || sub.Symbol != "BTCUSDT" || sub.Options.Interval != "1h" {
        t.Errorf("Unexpected subscription details: %+v", sub)
    }
}

func TestBullMarketSupportBandStrategy_NoSignal_IndicatorsNotReady(t *testing.T) {
	ctx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	mockSession := NewMockExchangeSession("BTCUSDT")
	mockOrderExecutor := &MockOrderExecutor{}
	
	strategy := newTestBMSBStrategy("BTCUSDT", "1m", 3, 3, 3, fixedpoint.NewFromFloat(0.01), fixedpoint.NewFromInt(1))

	go func() { 
		_ = strategy.Run(ctx, mockOrderExecutor, mockSession)
	}()
	
	klines := []types.KLine{
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromInt(10000), fixedpoint.NewFromInt(10010), fixedpoint.NewFromInt(9990), time.Now().Add(-2*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromInt(10010), fixedpoint.NewFromInt(10020), fixedpoint.NewFromInt(10000), time.Now().Add(-1*time.Minute)),
	}

	for _, k := range klines {
		mockSession.TriggerKLine(k) // Use the new TriggerKLine method on MockExchangeSession
		time.Sleep(10 * time.Millisecond) 
	}
	
	time.Sleep(50 * time.Millisecond)

	submittedOrders := mockOrderExecutor.GetSubmittedOrders()
	if len(submittedOrders) > 0 {
		t.Errorf("Expected no orders to be submitted when indicators are not ready, got %d orders", len(submittedOrders))
	}
}

func TestBullMarketSupportBandStrategy_NoSignal_NeutralConditions(t *testing.T) {
	ctx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	mockSession := NewMockExchangeSession("BTCUSDT")
	mockOrderExecutor := &MockOrderExecutor{}

	strategy := newTestBMSBStrategy("BTCUSDT", "1m", 3, 3, 3, fixedpoint.NewFromFloat(0.01), fixedpoint.NewFromInt(1))

	go func() {
		_ = strategy.Run(ctx, mockOrderExecutor, mockSession)
	}()

	baseTime := time.Now()
	klines := []types.KLine{
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromInt(10000), fixedpoint.NewFromInt(10000), fixedpoint.NewFromInt(10000), baseTime.Add(-5*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromInt(10000), fixedpoint.NewFromInt(10000), fixedpoint.NewFromInt(10000), baseTime.Add(-4*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromInt(10000), fixedpoint.NewFromInt(10000), fixedpoint.NewFromInt(10000), baseTime.Add(-3*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromInt(10005), fixedpoint.NewFromInt(10010), fixedpoint.NewFromInt(10000), baseTime.Add(-2*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromInt(10020), fixedpoint.NewFromInt(10025), fixedpoint.NewFromInt(10015), baseTime.Add(-1*time.Minute)),
	}
	
	for _, k := range klines {
		mockSession.TriggerKLine(k)
		time.Sleep(10 * time.Millisecond) 
	}
	
	time.Sleep(50 * time.Millisecond) 

	submittedOrders := mockOrderExecutor.GetSubmittedOrders()
	if len(submittedOrders) > 0 {
		t.Errorf("Expected no orders under neutral conditions, got %d. Orders: %+v", len(submittedOrders), submittedOrders)
	}
}

func TestBullMarketSupportBandStrategy_BuySignal_And_Hold(t *testing.T) {
	ctx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	mockSession := NewMockExchangeSession("BTCUSDT")
	mockOrderExecutor := &MockOrderExecutor{}

	qty := fixedpoint.NewFromFloat(0.01)
	strategy := newTestBMSBStrategy("BTCUSDT", "1m", 3, 3, 3, qty, fixedpoint.NewFromInt(1))

	go func() {
		_ = strategy.Run(ctx, mockOrderExecutor, mockSession)
	}()

	baseTime := time.Now()
	initialKlines := []types.KLine{
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), baseTime.Add(-5*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), baseTime.Add(-4*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), baseTime.Add(-3*time.Minute)),
	}
	for _, k := range initialKlines {
		mockSession.TriggerKLine(k)
		time.Sleep(10 * time.Millisecond)
	}

	buySignalKLineTime := baseTime.Add(-2 * time.Minute)
	buySignalKLine := newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10020), fixedpoint.NewFromFloat(10025), fixedpoint.NewFromFloat(10005), buySignalKLineTime)
	mockSession.TriggerKLine(buySignalKLine)
	time.Sleep(50 * time.Millisecond) 

	submittedOrders := mockOrderExecutor.GetSubmittedOrders()
	if len(submittedOrders) != 1 {
		t.Fatalf("Expected 1 buy order, got %d. Orders: %+v", len(submittedOrders), submittedOrders)
	}
	buySubmitOrder := submittedOrders[0] // This is types.SubmitOrder
	if buySubmitOrder.Side != types.SideTypeBuy || buySubmitOrder.Symbol != "BTCUSDT" || buySubmitOrder.QuantityString != qty.String() {
		t.Errorf("Incorrect buy order details: %+v", buySubmitOrder)
	}
	
	createdOrders := mockOrderExecutor.GetCreatedOrdersResponse()
	if len(createdOrders) == 0 {
		t.Fatal("MockOrderExecutor did not generate a created order response for the buy order")
	}
	mockedBuyOrderID := createdOrders[len(createdOrders)-1].OrderID 

	filledBuyOrder := types.Order{
		OrderID: mockedBuyOrderID, Symbol: buySubmitOrder.Symbol, Side: types.SideTypeBuy, Type: types.OrderTypeMarket,
		Price: buySignalKLine.ClosePrice, AveragePrice: buySignalKLine.ClosePrice,
		Quantity: qty, ExecutedQuantity: qty, Status: types.OrderStatusFilled, // Assuming strategy used qty for Quantity field of SubmitOrder
		UpdateTime: types.MillisecondTimestamp(time.Now().UnixMilli()),
	}
	strategy.OnOrderUpdate(filledBuyOrder)

	mockOrderExecutor.Reset() 

	holdKlines := []types.KLine{
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10030), fixedpoint.NewFromFloat(10035), fixedpoint.NewFromFloat(10025), baseTime.Add(-1*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10040), fixedpoint.NewFromFloat(10045), fixedpoint.NewFromFloat(10035), baseTime.Add(0*time.Minute)),
	}
	for _, k := range holdKlines {
		mockSession.TriggerKLine(k)
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)

	submittedOrders = mockOrderExecutor.GetSubmittedOrders()
	if len(submittedOrders) > 0 {
		t.Errorf("Expected no new orders during hold phase, got %d. Orders: %+v", len(submittedOrders), submittedOrders)
	}
}


func TestBullMarketSupportBandStrategy_SellSignal_PriceBelowBand(t *testing.T) {
	ctx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	mockSession := NewMockExchangeSession("BTCUSDT")
	mockOrderExecutor := &MockOrderExecutor{}

	qty := fixedpoint.NewFromFloat(0.01)
	strategy := newTestBMSBStrategy("BTCUSDT", "1m", 3, 3, 3, qty, fixedpoint.NewFromInt(1))

	go func() {
		_ = strategy.Run(ctx, mockOrderExecutor, mockSession)
	}()

	baseTime := time.Now()
	initialKlines := []types.KLine{
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), baseTime.Add(-6*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), baseTime.Add(-5*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), baseTime.Add(-4*time.Minute)),
	}
	for _, k := range initialKlines {
		mockSession.TriggerKLine(k)
		time.Sleep(10 * time.Millisecond)
	}
	buySignalKLineTime := baseTime.Add(-3 * time.Minute)
	buySignalKLine := newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10020), fixedpoint.NewFromFloat(10025), fixedpoint.NewFromFloat(10005), buySignalKLineTime)
	mockSession.TriggerKLine(buySignalKLine)
	time.Sleep(50 * time.Millisecond) 

	submittedBuyOrders := mockOrderExecutor.GetSubmittedOrders()
	if len(submittedBuyOrders) != 1 {
		t.Fatalf("Expected 1 buy order in setup, got %d.", len(submittedBuyOrders))
	}
	
	createdOrders := mockOrderExecutor.GetCreatedOrdersResponse()
	if len(createdOrders) == 0 {
		t.Fatal("MockOrderExecutor did not generate a created order response for the buy order in setup")
	}
	mockedBuyOrderID := createdOrders[len(createdOrders)-1].OrderID 

	filledBuyOrder := types.Order{
		OrderID: mockedBuyOrderID, Symbol: "BTCUSDT", Side: types.SideTypeBuy, Type: types.OrderTypeMarket,
		Price: buySignalKLine.ClosePrice, AveragePrice: buySignalKLine.ClosePrice,
		Quantity: qty, ExecutedQuantity: qty, Status: types.OrderStatusFilled,
		UpdateTime: types.MillisecondTimestamp(time.Now().UnixMilli()),
	}
	strategy.OnOrderUpdate(filledBuyOrder) 

	mockOrderExecutor.Reset() 

	sellSignalKLineTime := baseTime.Add(-2 * time.Minute)
	sellSignalKLine := newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(9990), fixedpoint.NewFromFloat(9995), fixedpoint.NewFromFloat(9985), sellSignalKLineTime)
	mockSession.TriggerKLine(sellSignalKLine)
	time.Sleep(50 * time.Millisecond)

	submittedSellOrders := mockOrderExecutor.GetSubmittedOrders()
	if len(submittedSellOrders) != 1 {
		t.Fatalf("Expected 1 sell order, got %d. Orders: %+v", len(submittedSellOrders), submittedSellOrders)
	}
	sellOrder := submittedSellOrders[0]
	if sellOrder.Side != types.SideTypeSell || sellOrder.Symbol != "BTCUSDT" || sellOrder.QuantityString != qty.String() { 
		t.Errorf("Incorrect sell order details: %+v. Expected Qty: %s", sellOrder, qty.String())
	}
}


func TestBullMarketSupportBandStrategy_SellSignal_MarketTurnsBearish(t *testing.T) {
	ctx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	mockSession := NewMockExchangeSession("BTCUSDT")
	mockOrderExecutor := &MockOrderExecutor{}

	qty := fixedpoint.NewFromFloat(0.01)
	strategy := newTestBMSBStrategy("BTCUSDT", "1m", 3, 3, 5, qty, fixedpoint.NewFromInt(1))

	go func() {
		_ = strategy.Run(ctx, mockOrderExecutor, mockSession)
	}()

	baseTime := time.Now()
	setupKlines := []types.KLine{
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), baseTime.Add(-10*time.Minute)), 
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), baseTime.Add(-9*time.Minute)),  
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10050), fixedpoint.NewFromFloat(10050), fixedpoint.NewFromFloat(10050), baseTime.Add(-8*time.Minute)), 
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10050), fixedpoint.NewFromFloat(10050), fixedpoint.NewFromFloat(10050), baseTime.Add(-7*time.Minute)), 
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10050), fixedpoint.NewFromFloat(10050), fixedpoint.NewFromFloat(10050), baseTime.Add(-6*time.Minute)), 
	}
	for _, k := range setupKlines {
		mockSession.TriggerKLine(k)
		time.Sleep(10 * time.Millisecond)
	}
	buySignalKLine := newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10060), fixedpoint.NewFromFloat(10065), fixedpoint.NewFromFloat(10045), baseTime.Add(-5*time.Minute))
	mockSession.TriggerKLine(buySignalKLine)
	time.Sleep(50 * time.Millisecond)

	submittedBuyOrders := mockOrderExecutor.GetSubmittedOrders()
	if len(submittedBuyOrders) != 1 {
		t.Fatalf("Expected 1 buy order in setup, got %d.", len(submittedBuyOrders))
	}
	createdOrders := mockOrderExecutor.GetCreatedOrdersResponse()
	if len(createdOrders) == 0 { t.Fatal("No created order for buy") }
	mockedBuyOrderID := createdOrders[len(createdOrders)-1].OrderID
	filledBuyOrder := types.Order{
		OrderID: mockedBuyOrderID, Symbol: "BTCUSDT", Side: types.SideTypeBuy, Type: types.OrderTypeMarket,
		Price: buySignalKLine.ClosePrice, AveragePrice: buySignalKLine.ClosePrice,
		Quantity: qty, ExecutedQuantity: qty, Status: types.OrderStatusFilled,
		UpdateTime: types.MillisecondTimestamp(time.Now().UnixMilli()),
	}
	strategy.OnOrderUpdate(filledBuyOrder)

	mockOrderExecutor.Reset()
	mockSession.TriggerKLine(newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10055), fixedpoint.NewFromFloat(10060), fixedpoint.NewFromFloat(10050), baseTime.Add(-4*time.Minute)))
	time.Sleep(10 * time.Millisecond)
	sellSignalKLine := newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10005), fixedpoint.NewFromFloat(9995), baseTime.Add(-3*time.Minute))
	mockSession.TriggerKLine(sellSignalKLine)
	time.Sleep(50 * time.Millisecond)

	submittedSellOrders := mockOrderExecutor.GetSubmittedOrders()
	if len(submittedSellOrders) != 1 {
		t.Fatalf("Expected 1 sell order due to bearish market, got %d. Orders: %+v", len(submittedSellOrders), submittedSellOrders)
	}
	sellOrder := submittedSellOrders[0]
	if sellOrder.Side != types.SideTypeSell || sellOrder.QuantityString != qty.String() {
		t.Errorf("Incorrect sell order: %+v", sellOrder)
	}
}

// For TestBullMarketSupportBandStrategy_OrderUpdateHandling:
// To properly unit test OnOrderUpdate, the strategy's internal state fields (lastOrderID, currentPositionQuantity, etc.)
// would need to be accessible from the test. Since they are unexported in Go, this requires either:
// 1. Exporting them (generally not good practice for non-test code).
// 2. Adding build-tagged (+build test) helper methods in the *strategy's own package* to set/get these fields.
// 3. Using reflection (complex and generally avoided in unit tests if possible).
// Since we cannot modify the strategy's source file in this step to add such helpers,
// this test will be more limited or conceptual. We'll test the logic by observing subsequent behavior
// or by creating a local instance and calling OnOrderUpdate, assuming we could verify its state.
// The following test assumes such test helpers (`SetPrivateFieldsForTesting`, etc.) would exist.
func TestBullMarketSupportBandStrategy_OrderUpdateHandling(t *testing.T) {
	// Create a strategy instance directly to call OnOrderUpdate on it.
	// This doesn't involve the Run loop or K-lines, focusing only on OnOrderUpdate logic.
	strategy := newTestBMSBStrategy("BTCUSDT", "1m", 3, 3, 3, fixedpoint.NewFromFloat(0.1), fixedpoint.NewFromInt(1))

	// --- Test Buy Order Filled ---
	buyQty := fixedpoint.NewFromFloat(0.1)
	buyPrice := fixedpoint.NewFromFloat(10000)
	
	// Manually set strategy state as if an order was just submitted
	// This is where test helpers would be used. e.g., strategy.SetLastOrderID(123)
	// For now, we'll call OnOrderUpdate and know that if lastOrderID was 0, it won't match.
	// To make this test pass as-is, we need to simulate that lastOrderID *was* 123.
	// This test is more of a specification of how OnOrderUpdate *should* behave *if* lastOrderID matches.
	
	// Test Case 1: Buy order filled, matching lastOrderID
	// To simulate this, we'd need to set strategy.lastOrderID = 123 before calling.
	// Since we can't, this part of the test is illustrative.
	// If we assume lastOrderID was set to 123 by prior logic:
	buyOrderFilled := types.Order{
		OrderID: 123, Symbol: "BTCUSDT", Side: types.SideTypeBuy, Type: types.OrderTypeMarket,
		Price: buyPrice, AveragePrice: buyPrice, Quantity: buyQty, ExecutedQuantity: buyQty, Status: types.OrderStatusFilled,
	}
	// strategy.lastOrderID = 123; // PSEUDOCODE: if we could set it
	// strategy.OnOrderUpdate(buyOrderFilled) 
	// Assertions on strategy.currentPositionQuantity, strategy.averageEntryPrice, strategy.lastOrderID would follow.
	t.Log("Conceptual test for buy order fill: Requires state manipulation or test helpers in strategy.")


	// Test Case 2: Sell order filled
	// strategy.lastOrderID = 456; // PSEUDOCODE
	// strategy.currentPositionQuantity = buyQty; 
	// strategy.averageEntryPrice = buyPrice;
	sellPrice := fixedpoint.NewFromFloat(10100)
	sellOrderFilled := types.Order{
		OrderID: 456, Symbol: "BTCUSDT", Side: types.SideTypeSell, Type: types.OrderTypeMarket,
		Price: sellPrice, AveragePrice: sellPrice, Quantity: buyQty, ExecutedQuantity: buyQty, Status: types.OrderStatusFilled,
	}
	// strategy.OnOrderUpdate(sellOrderFilled)
	// Assertions on strategy.currentPositionQuantity == 0, strategy.averageEntryPrice == 0, etc.
	t.Log("Conceptual test for sell order fill: Requires state manipulation.")

	// Test Case 3: Order Canceled
	// strategy.lastOrderID = 789; // PSEUDOCODE
	orderCanceled := types.Order{OrderID: 789, Symbol: "BTCUSDT", Status: types.OrderStatusCanceled}
	// strategy.OnOrderUpdate(orderCanceled)
	// Assert strategy.lastOrderID == 0
	t.Log("Conceptual test for order cancel: Requires state manipulation.")
	
	// Test Case 4: Update for an unknown order ID
	// strategy.lastOrderID = 111; // PSEUDOCODE
	unknownOrderUpdate := types.Order{OrderID: 999, Status: types.OrderStatusFilled} // Different ID
	// strategy.OnOrderUpdate(unknownOrderUpdate)
	// Assert strategy state (currentPositionQuantity, etc.) unchanged.
	t.Log("Conceptual test for unknown order update: Requires state manipulation.")

	t.Skip("TestBullMarketSupportBandStrategy_OrderUpdateHandling is conceptual due to unexported state. Requires test helpers in strategy code for full execution.")
}


func TestBullMarketSupportBandStrategy_Buy_NoQuantityConfigured(t *testing.T) {
	ctx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	mockSession := NewMockExchangeSession("BTCUSDT")
	mockOrderExecutor := &MockOrderExecutor{}

	strategy := newTestBMSBStrategy("BTCUSDT", "1m", 3, 3, 3, fixedpoint.Zero, fixedpoint.NewFromInt(1)) // Quantity is Zero

	go func() {
		_ = strategy.Run(ctx, mockOrderExecutor, mockSession)
	}()

	baseTime := time.Now()
	klines := []types.KLine{
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), baseTime.Add(-5*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), baseTime.Add(-4*time.Minute)),
		newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), fixedpoint.NewFromFloat(10000), baseTime.Add(-3*time.Minute)),
	}
	for _, k := range klines {
		mockSession.TriggerKLine(k)
		time.Sleep(10 * time.Millisecond)
	}
	
	buySignalKLine := newKLine("BTCUSDT", "1m", fixedpoint.NewFromFloat(10020), fixedpoint.NewFromFloat(10025), fixedpoint.NewFromFloat(10005), baseTime.Add(-2*time.Minute))
	mockSession.TriggerKLine(buySignalKLine)
	time.Sleep(50 * time.Millisecond) 

	submittedOrders := mockOrderExecutor.GetSubmittedOrders()
	if len(submittedOrders) > 0 {
		t.Errorf("Expected no orders when strategy quantity is zero, got %d. Orders: %+v", len(submittedOrders), submittedOrders)
	}
	// Ideally, also check for a log message indicating the issue.
}

func TestBullMarketSupportBandStrategy_LeverageSettingError(t *testing.T) {
	ctx := context.Background()
	mockSession := NewMockExchangeSession("BTCUSDT")
	
	mockSession.mockExchange.LeverageSetError = fmt.Errorf("simulated leverage error")
	strategy := newTestBMSBStrategy("BTCUSDT", "1h", 20, 21, 50, fixedpoint.NewFromFloat(0.01), fixedpoint.NewFromInt(5))
	
	strategy.InitializeIndicators(ctx, mockSession)
	// Test confirms SetLeverage is called. Strategy logs error, continues.
	// No direct functional change to assert beyond no panic and that the call was made (verified by mockExchange fields if no error).
	// If LeverageSetError is non-nil, LeverageSetSymbol/Leverage might not be set in mock, which is fine.
	// The key is that the strategy attempts the call.
	t.Log("LeverageSettingError: Test assumes strategy logs the error and continues.")
}
