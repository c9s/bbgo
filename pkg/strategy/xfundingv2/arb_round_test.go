package xfundingv2

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

// mockFuturesService implements FuturesService for testing
type mockFuturesService struct {
	mu sync.Mutex

	incomeHistory []binanceapi.FuturesIncome
	incomeErr     error
	transferErr   error
}

func (m *mockFuturesService) ResetIncomeError() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.incomeErr = nil
}

func (m *mockFuturesService) SetIncomeError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.incomeErr = err
}

func (m *mockFuturesService) QueryFuturesIncomeHistory(
	ctx context.Context, symbol string, incomeType binanceapi.FuturesIncomeType,
	startTime, endTime *time.Time,
) ([]binanceapi.FuturesIncome, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.incomeHistory, m.incomeErr
}

func (m *mockFuturesService) TransferFuturesAccountAsset(
	ctx context.Context, asset string, amount fixedpoint.Value, direction types.TransferDirection,
) error {
	return m.transferErr
}

func (m *mockFuturesService) QueryPremiumIndex(ctx context.Context, symbol string) (*types.PremiumIndex, error) {
	return nil, nil
}

func (m *mockFuturesService) QueryPositionRisk(ctx context.Context, symbol ...string) ([]types.PositionRisk, error) {
	return nil, nil
}

func newTestArbitrageRound(t *testing.T, ctrl *gomock.Controller, fundingIntervalHours, minHoldingIntervals int, nextFundingTime time.Time) (*ArbitrageRound, *mockFuturesService) {
	config := TWAPWorkerConfig{
		Duration:  10 * time.Minute,
		NumSlices: 5,
	}

	spotWorker, _, _, _ := newTestTWAPWorker(t, ctrl, config)
	spotWorker.SetTargetPosition(Number(1.0)) // long spot

	futuresWorker, _, _, _ := newTestTWAPWorker(t, ctrl, config)
	futuresWorker.SetTargetPosition(Number(-1.0)) // short futures

	mockService := &mockFuturesService{}

	fundingRate := &types.PremiumIndex{
		LastFundingRate: Number(0.001),
		NextFundingTime: nextFundingTime,
	}

	round := NewArbitrageRound(
		fundingRate,
		types.ExchangeBinance, types.ExchangeBinance,
		minHoldingIntervals, fundingIntervalHours, spotWorker, futuresWorker, mockService)
	return round, mockService
}

func TestArbitrageRound_NumHoldingIntervals(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Funding at 08:00 UTC, 8h interval → FundingIntervalStart = 00:00 UTC
	nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)

	t.Run("returns_zero_when_start_time_is_zero", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		// StartTime is zero by default (round not started)
		result := round.NumHoldingIntervals(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
		assert.Equal(t, 0, result)
	})

	t.Run("returns_zero_when_current_time_before_funding_interval_start", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.StartTime = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// currentTime is before FundingIntervalStart (00:00 UTC)
		result := round.NumHoldingIntervals(time.Date(2023, 12, 31, 23, 0, 0, 0, time.UTC))
		assert.Equal(t, 0, result)
	})

	t.Run("returns_zero_within_first_interval", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.StartTime = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// 4 hours after FundingIntervalStart → still in first 8h interval
		result := round.NumHoldingIntervals(time.Date(2024, 1, 1, 4, 0, 0, 0, time.UTC))
		assert.Equal(t, 0, result)
	})

	t.Run("returns_one_after_first_interval", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.StartTime = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// exactly 8 hours after FundingIntervalStart
		result := round.NumHoldingIntervals(time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC))
		assert.Equal(t, 1, result)
	})

	t.Run("returns_three_after_24_hours_with_8h_interval", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.StartTime = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// 24 hours after FundingIntervalStart → 3 intervals
		result := round.NumHoldingIntervals(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
		assert.Equal(t, 3, result)
	})

	t.Run("rounds_down_partial_intervals", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.StartTime = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// 20 hours after FundingIntervalStart → 2.5 intervals → rounds down to 2
		result := round.NumHoldingIntervals(time.Date(2024, 1, 1, 20, 0, 0, 0, time.UTC))
		assert.Equal(t, 2, result)
	})

	t.Run("works_with_4h_funding_interval", func(t *testing.T) {
		// Funding at 04:00 UTC, 4h interval → FundingIntervalStart = 00:00 UTC
		nextFunding4h := time.Date(2024, 1, 1, 4, 0, 0, 0, time.UTC)
		round, _ := newTestArbitrageRound(t, ctrl, 4, 3, nextFunding4h)
		round.syncState.StartTime = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// 12 hours after FundingIntervalStart → 3 intervals of 4h
		result := round.NumHoldingIntervals(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
		assert.Equal(t, 3, result)
	})

	t.Run("works_with_non_epoch_aligned_funding_start", func(t *testing.T) {
		// Funding at 10:00 UTC, 8h interval → FundingIntervalStart = 02:00 UTC
		nextFundingOffset := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingOffset)
		round.syncState.StartTime = time.Date(2024, 1, 1, 2, 30, 0, 0, time.UTC)
		// FundingIntervalStart is 02:00; currentTime 18:00 → 16h elapsed → 2 intervals
		result := round.NumHoldingIntervals(time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC))
		assert.Equal(t, 2, result)
	})
}

func TestArbitrageRound_TotalFundingIncome(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	round, mockService := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)

	t.Run("returns zero when startTime is zero", func(t *testing.T) {
		ctx := context.Background()
		currentTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
		err := round.SyncFundingFeeRecords(ctx, currentTime)
		assert.NoError(t, err)
		result := round.TotalFundingIncome()
		assert.Equal(t, fixedpoint.Zero, result)
	})

	t.Run("sums funding fee records", func(t *testing.T) {
		round.syncState.StartTime = time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)

		// Simulate funding fee income returned by the service
		mockService.incomeHistory = []binanceapi.FuturesIncome{
			{
				Symbol:     "BTCUSDT",
				IncomeType: binanceapi.FuturesIncomeFundingFee,
				Income:     Number(0.005),
				Asset:      "USDT",
				TranId:     1001,
				Time:       types.NewMillisecondTimestampFromInt(time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC).UnixMilli()),
			},
			{
				Symbol:     "BTCUSDT",
				IncomeType: binanceapi.FuturesIncomeFundingFee,
				Income:     Number(0.003),
				Asset:      "USDT",
				TranId:     1002,
				Time:       types.NewMillisecondTimestampFromInt(time.Date(2024, 1, 1, 16, 0, 0, 0, time.UTC).UnixMilli()),
			},
		}

		ctx := context.Background()
		currentTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
		err := round.SyncFundingFeeRecords(ctx, currentTime)
		assert.NoError(t, err)
		result := round.TotalFundingIncome()

		expected := Number(0.005).Add(Number(0.003))
		assert.Equal(t, expected, result)
	})

	t.Run("deduplicates by transaction ID", func(t *testing.T) {
		// Call again with same income history - should not double-count
		ctx := context.Background()
		currentTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
		err := round.SyncFundingFeeRecords(ctx, currentTime)
		assert.NoError(t, err)
		result := round.TotalFundingIncome()

		expected := Number(0.005).Add(Number(0.003))
		assert.Equal(t, expected, result)
	})

	t.Run("accumulates new records", func(t *testing.T) {
		// Add a third funding fee
		mockService.incomeHistory = append(mockService.incomeHistory, binanceapi.FuturesIncome{
			Symbol:     "BTCUSDT",
			IncomeType: binanceapi.FuturesIncomeFundingFee,
			Income:     Number(0.002),
			Asset:      "USDT",
			TranId:     1003,
			Time:       types.NewMillisecondTimestampFromInt(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli()),
		})

		ctx := context.Background()
		currentTime := time.Date(2024, 1, 2, 8, 0, 0, 0, time.UTC)
		err := round.SyncFundingFeeRecords(ctx, currentTime)
		assert.NoError(t, err)
		result := round.TotalFundingIncome()

		expected := Number(0.005).Add(Number(0.003)).Add(Number(0.002))
		assert.Equal(t, expected, result)
	})

	t.Run("returns error when income query fails", func(t *testing.T) {
		queryErr := errors.New("income history query failed")
		mockService.SetIncomeError(queryErr)
		defer mockService.ResetIncomeError()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		currentTime := time.Date(2024, 1, 2, 8, 0, 0, 0, time.UTC)
		err := round.SyncFundingFeeRecords(ctx, currentTime)
		assert.Error(t, err)
	})
}

func TestArbitrageRound_DeltaNeutral(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{
		Duration:      10 * time.Minute,
		NumSlices:     2,
		OrderType:     TWAPOrderTypeMaker,
		CheckInterval: 1 * time.Second,
		NumOfTicks:    1,
	}

	nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	startTime := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)

	spotWorker, spotMockExchange, spotMockOrderQuery, spotGeneralExecutor := newTestTWAPWorker(t, ctrl, config)
	futuresWorker, futuresMockExchange, futuresMockOrderQuery, futuresGeneralExecutor := newTestTWAPWorker(t, ctrl, config)

	targetPosition := Number(10.0)
	spotWorker.SetTargetPosition(targetPosition)

	mockService := &mockFuturesService{}

	fundingRate := &types.PremiumIndex{
		LastFundingRate: Number(0.001),
		NextFundingTime: nextFundingTime,
	}

	round := NewArbitrageRound(
		fundingRate,
		types.ExchangeBinance, types.ExchangeBinance,
		3, 8, spotWorker, futuresWorker, mockService)
	round.SetLogger(logrus.WithField("test", "delta_neutral"))

	spotOrderID := uint64(1000)
	setupDeltaNeutralMockExchange(spotMockExchange, spotMockOrderQuery, &spotOrderID)

	futuresOrderID := uint64(2000)
	setupDeltaNeutralMockExchange(futuresMockExchange, futuresMockOrderQuery, &futuresOrderID)

	assertDeltaNeutral := func(t *testing.T, msg string) {
		t.Helper()
		spotFilled := spotWorker.FilledPosition()
		futuresFilled := futuresWorker.FilledPosition()
		netDelta := spotFilled.Add(futuresFilled)
		assert.True(t, netDelta.IsZero(),
			"%s: delta-neutral violated, spot=%s, futures=%s, net=%s",
			msg, spotFilled, futuresFilled, netDelta)
	}

	assertFuturesTargetMatchesSpot := func(t *testing.T, msg string) {
		t.Helper()
		spotFilled := spotWorker.FilledPosition()
		futuresTarget := futuresWorker.TargetPosition()
		expected := spotFilled.Neg()
		assert.Equal(t, expected, futuresTarget,
			"%s: futures target should be negation of spot filled, spot=%s, futuresTarget=%s",
			msg, spotFilled, futuresTarget)
	}

	orderBook := newOrderBook(76990.0, 100.0, 77010.0, 100.0)

	// ==========================================
	// Phase 1: Start the round (Opening)
	// ==========================================
	ctx := context.Background()
	err := round.Start(ctx, startTime)
	assert.NoError(t, err)
	assert.Equal(t, RoundOpening, round.State())
	assertDeltaNeutral(t, "after start")

	// ==========================================
	// Phase 2: Opening - buy spot in slices, verify futures stays delta-neutral
	// ==========================================

	// Tick spot worker -> places first slice order (10/2 = 5 BTC)
	err = spotWorker.Tick(startTime, orderBook)
	assert.NoError(t, err)
	assert.NotNil(t, spotWorker.ActiveOrder())

	// Spot trade 1: buy 5 BTC
	spotTrade1 := makeTrade(1, spotWorker.ActiveOrder().OrderID, types.SideTypeBuy, Number(77010.0), Number(5.0))
	processTrade(spotWorker, spotGeneralExecutor, spotTrade1)
	round.HandleSpotTrade(spotTrade1, startTime)

	assertFuturesTargetMatchesSpot(t, "after spot trade 1")
	assert.Equal(t, Number(-5.0), futuresWorker.TargetPosition())

	// Tick futures worker -> places order matching synced target
	err = futuresWorker.Tick(startTime, orderBook)
	assert.NoError(t, err)
	assert.NotNil(t, futuresWorker.ActiveOrder())

	// Futures trade 1: sell 5 BTC
	futuresTrade1 := makeTrade(101, futuresWorker.ActiveOrder().OrderID, types.SideTypeSell, Number(77010.0), Number(5.0))
	processTrade(futuresWorker, futuresGeneralExecutor, futuresTrade1)
	round.HandleFuturesTrade(futuresTrade1, startTime)

	assertDeltaNeutral(t, "after opening trade 1")
	assert.Equal(t, Number(5.0), spotWorker.FilledPosition())
	assert.Equal(t, Number(-5.0), futuresWorker.FilledPosition())

	// Tick spot worker -> places second slice order (remaining 5/1 = 5 BTC)
	tick2 := startTime.Add(5 * time.Minute)
	err = spotWorker.Tick(tick2, orderBook)
	assert.NoError(t, err)

	// Spot trade 2: buy 5 BTC more
	spotTrade2 := makeTrade(2, spotWorker.ActiveOrder().OrderID, types.SideTypeBuy, Number(77010.0), Number(5.0))
	processTrade(spotWorker, spotGeneralExecutor, spotTrade2)
	round.HandleSpotTrade(spotTrade2, tick2)

	assertFuturesTargetMatchesSpot(t, "after spot trade 2")
	assert.Equal(t, Number(-10.0), futuresWorker.TargetPosition())

	// Tick futures worker -> fills remaining
	err = futuresWorker.Tick(tick2, orderBook)
	assert.NoError(t, err)

	futuresTrade2 := makeTrade(102, futuresWorker.ActiveOrder().OrderID, types.SideTypeSell, Number(77010.0), Number(5.0))
	processTrade(futuresWorker, futuresGeneralExecutor, futuresTrade2)
	round.HandleFuturesTrade(futuresTrade2, tick2)

	assertDeltaNeutral(t, "after opening trade 2 (fully opened)")
	assert.Equal(t, Number(10.0), spotWorker.FilledPosition())
	assert.Equal(t, Number(-10.0), futuresWorker.FilledPosition())

	// ==========================================
	// Phase 3: Tick round to transition to RoundReady
	// ==========================================
	tickReady := startTime.Add(6 * time.Minute)
	round.Tick(tickReady, orderBook, orderBook)
	assert.Equal(t, RoundReady, round.State())
	assertDeltaNeutral(t, "at RoundReady")

	// ==========================================
	// Phase 4: Set closing
	// ==========================================
	closeTime := startTime.Add(20 * time.Minute)
	closeDuration := 10 * time.Minute
	round.SetClosing(closeTime, closeDuration)
	assert.Equal(t, RoundClosing, round.State())
	assert.Equal(t, fixedpoint.Zero, spotWorker.TargetPosition())
	assertDeltaNeutral(t, "after SetClosing (before closing trades)")

	// ==========================================
	// Phase 5: Closing - sell spot, buy back futures
	// ==========================================

	// Tick spot worker -> places sell order (remaining = -10, slice = 10/2 = 5)
	err = spotWorker.Tick(closeTime, orderBook)
	assert.NoError(t, err)

	// Spot close trade 1: sell 5 BTC
	spotCloseTrade1 := makeTrade(3, spotWorker.ActiveOrder().OrderID, types.SideTypeSell, Number(76990.0), Number(5.0))
	processTrade(spotWorker, spotGeneralExecutor, spotCloseTrade1)
	round.HandleSpotTrade(spotCloseTrade1, closeTime)

	assertFuturesTargetMatchesSpot(t, "after spot close trade 1")
	assert.Equal(t, Number(5.0), spotWorker.FilledPosition())
	assert.Equal(t, Number(-5.0), futuresWorker.TargetPosition())

	// Tick futures worker -> places buy-back order
	err = futuresWorker.Tick(closeTime, orderBook)
	assert.NoError(t, err)

	futuresCloseTrade1 := makeTrade(103, futuresWorker.ActiveOrder().OrderID, types.SideTypeBuy, Number(76990.0), Number(5.0))
	processTrade(futuresWorker, futuresGeneralExecutor, futuresCloseTrade1)
	round.HandleFuturesTrade(futuresCloseTrade1, closeTime)

	assertDeltaNeutral(t, "after closing trade 1")

	// Tick spot worker -> places second sell order
	tick5 := closeTime.Add(5 * time.Minute)
	err = spotWorker.Tick(tick5, orderBook)
	assert.NoError(t, err)

	// Spot close trade 2: sell remaining 5 BTC
	spotCloseTrade2 := makeTrade(4, spotWorker.ActiveOrder().OrderID, types.SideTypeSell, Number(76990.0), Number(5.0))
	processTrade(spotWorker, spotGeneralExecutor, spotCloseTrade2)
	round.HandleSpotTrade(spotCloseTrade2, tick5)

	assertFuturesTargetMatchesSpot(t, "after spot close trade 2")
	assert.True(t, spotWorker.FilledPosition().IsZero())
	assert.True(t, futuresWorker.TargetPosition().IsZero())

	// Tick futures worker -> places final buy-back order
	err = futuresWorker.Tick(tick5, orderBook)
	assert.NoError(t, err)

	futuresCloseTrade2 := makeTrade(104, futuresWorker.ActiveOrder().OrderID, types.SideTypeBuy, Number(76990.0), Number(5.0))
	processTrade(futuresWorker, futuresGeneralExecutor, futuresCloseTrade2)
	round.HandleFuturesTrade(futuresCloseTrade2, tick5)

	assertDeltaNeutral(t, "after closing trade 2 (fully closed)")
	assert.True(t, spotWorker.FilledPosition().IsZero())
	assert.True(t, futuresWorker.FilledPosition().IsZero())

	// ==========================================
	// Phase 6: Tick round to transition to RoundClosed
	// ==========================================
	tickClosed := closeTime.Add(6 * time.Minute)
	round.Tick(tickClosed, orderBook, orderBook)
	assert.Equal(t, RoundClosed, round.State())
	assertDeltaNeutral(t, "at RoundClosed")
}

func setupDeltaNeutralMockExchange(mockExchange *mocks.MockExchange, mockOrderQuery *mocks.MockExchangeOrderQueryService, orderID *uint64) {
	mockExchange.EXPECT().
		SubmitOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
			currentOrderID := *orderID
			*orderID++
			return &types.Order{
				OrderID: currentOrderID,
				SubmitOrder: types.SubmitOrder{
					Symbol:   order.Symbol,
					Side:     order.Side,
					Type:     order.Type,
					Quantity: order.Quantity,
					Price:    order.Price,
				},
				Status: types.OrderStatusNew,
			}, nil
		}).AnyTimes()

	mockExchange.EXPECT().
		CancelOrders(gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	mockOrderQuery.EXPECT().
		QueryOrder(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
			return &types.Order{OrderID: 1, Status: types.OrderStatusCanceled}, nil
		}).AnyTimes()

	mockOrderQuery.EXPECT().
		QueryOrderTrades(gomock.Any(), gomock.Any()).
		Return([]types.Trade{}, nil).AnyTimes()
}
