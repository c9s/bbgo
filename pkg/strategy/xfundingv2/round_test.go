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

	"github.com/c9s/bbgo/pkg/bbgo"
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
	transfers     []transferCall
}

type transferCall struct {
	Asset     string
	Amount    fixedpoint.Value
	Direction types.TransferDirection
}

func (m *mockFuturesService) Transfers() []transferCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]transferCall, len(m.transfers))
	copy(out, m.transfers)
	return out
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
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.transferErr != nil {
		return m.transferErr
	}
	m.transfers = append(m.transfers, transferCall{Asset: asset, Amount: amount, Direction: direction})
	return nil
}

func (m *mockFuturesService) QueryPremiumIndex(ctx context.Context, symbol string) (*types.PremiumIndex, error) {
	return nil, nil
}

func (m *mockFuturesService) QueryPositionRisk(ctx context.Context, symbol ...string) ([]types.PositionRisk, error) {
	return nil, nil
}

func (m *mockFuturesService) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	return nil
}

func newTestArbitrageRound(t *testing.T, ctrl *gomock.Controller, fundingIntervalHours, minHoldingIntervals int, nextFundingTime time.Time) (*ArbitrageRound, *mockFuturesService) {
	config := TWAPWorkerConfig{
		Duration:  types.Duration(10 * time.Minute),
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
		minHoldingIntervals, fundingIntervalHours, Number(3), spotWorker, futuresWorker, mockService,
		types.PositionShort, time.Minute)
	round.SetLogger(logrus.WithField("test", "round_test"))
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
		round.syncState.StartAt = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// currentTime is before FundingIntervalStart (00:00 UTC)
		result := round.NumHoldingIntervals(time.Date(2023, 12, 31, 23, 0, 0, 0, time.UTC))
		assert.Equal(t, 0, result)
	})

	t.Run("returns_zero_within_first_interval", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.StartAt = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// 4 hours after FundingIntervalStart → still in first 8h interval
		result := round.NumHoldingIntervals(time.Date(2024, 1, 1, 4, 0, 0, 0, time.UTC))
		assert.Equal(t, 0, result)
	})

	t.Run("returns_one_after_first_interval", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.StartAt = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// exactly 8 hours after FundingIntervalStart
		result := round.NumHoldingIntervals(time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC))
		assert.Equal(t, 1, result)
	})

	t.Run("returns_three_after_24_hours_with_8h_interval", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.StartAt = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// 24 hours after FundingIntervalStart → 3 intervals
		result := round.NumHoldingIntervals(time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
		assert.Equal(t, 3, result)
	})

	t.Run("rounds_down_partial_intervals", func(t *testing.T) {
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingTime)
		round.syncState.StartAt = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// 20 hours after FundingIntervalStart → 2.5 intervals → rounds down to 2
		result := round.NumHoldingIntervals(time.Date(2024, 1, 1, 20, 0, 0, 0, time.UTC))
		assert.Equal(t, 2, result)
	})

	t.Run("works_with_4h_funding_interval", func(t *testing.T) {
		// Funding at 04:00 UTC, 4h interval → FundingIntervalStart = 00:00 UTC
		nextFunding4h := time.Date(2024, 1, 1, 4, 0, 0, 0, time.UTC)
		round, _ := newTestArbitrageRound(t, ctrl, 4, 3, nextFunding4h)
		round.syncState.StartAt = time.Date(2024, 1, 1, 0, 30, 0, 0, time.UTC)
		// 12 hours after FundingIntervalStart → 3 intervals of 4h
		result := round.NumHoldingIntervals(time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
		assert.Equal(t, 3, result)
	})

	t.Run("works_with_non_epoch_aligned_funding_start", func(t *testing.T) {
		// Funding at 10:00 UTC, 8h interval → FundingIntervalStart = 02:00 UTC
		nextFundingOffset := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		round, _ := newTestArbitrageRound(t, ctrl, 8, 3, nextFundingOffset)
		round.syncState.StartAt = time.Date(2024, 1, 1, 2, 30, 0, 0, time.UTC)
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

	t.Run("returns error when startTime is zero", func(t *testing.T) {
		ctx := context.Background()
		currentTime := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
		err := round.SyncFundingFeeRecords(ctx, currentTime)
		assert.Error(t, err)
		result := round.TotalFundingIncome()
		assert.Equal(t, fixedpoint.Zero, result)
	})

	t.Run("sums funding fee records", func(t *testing.T) {
		round.syncState.StartAt = time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)

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
		// Advance past the sync-throttle window (FundingIntervalHours/2 = 4h after
		// the previous subtest's sync at Jan 2 08:00) so the query actually runs
		// and its error can propagate.
		currentTime := time.Date(2024, 1, 2, 13, 0, 0, 0, time.UTC)
		err := round.SyncFundingFeeRecords(ctx, currentTime)
		assert.Error(t, err)
	})
}

// arbSpotTrade builds a spot trade for arb-round tests, populating QuoteQuantity
// from price*quantity so that LONG-direction transfers (which use QuoteQuantity)
// behave realistically.
func arbSpotTrade(id, orderID uint64, side types.SideType, price, quantity fixedpoint.Value) types.Trade {
	t := makeTrade(id, orderID, side, price, quantity)
	t.QuoteQuantity = price.Mul(quantity)
	return t
}

// arbFuturesTrade is the futures counterpart of arbSpotTrade. The IsFutures
// flag is required for syncSpotPosition (called on the closing leg) to update
// the spot target instead of bailing out early.
func arbFuturesTrade(id, orderID uint64, side types.SideType, price, quantity fixedpoint.Value) types.Trade {
	t := arbSpotTrade(id, orderID, side, price, quantity)
	t.IsFutures = true
	return t
}

type directionScenario struct {
	name         string
	direction    types.PositionType
	collateral   string
	spotTarget   fixedpoint.Value
	spotOpenSide types.SideType // side spot uses to open the position
	// futuresCloseSide is the side futures uses to close (opposite of its open).
	futuresCloseSide types.SideType
	// expectedFuturesTargetAfterFirstSpotFill is the target the round syncs onto
	// the futures leg after the first spot fill of half the position.
	expectedFuturesTargetAfterFirstSpotFill fixedpoint.Value
	// expectedSpotTargetAfterFirstFuturesClose is the target the round syncs onto
	// the spot leg after the first futures-close fill.
	expectedSpotTargetAfterFirstFuturesClose fixedpoint.Value
	// expectedTransferAmount returns the asset amount the round should transfer
	// when given the trade's price and quantity. For SHORT it's the base
	// quantity; for LONG it's the quote notional.
	expectedTransferAmount func(price, quantity fixedpoint.Value) fixedpoint.Value
}

// TestArbitrageRound_DirectionalLeaderFollower exercises both SHORT and LONG
// futures directions and verifies the leader/follower contract:
//
//	Opening: the spot leg is the leader. Each spot fill triggers a TransferIn
//	         of the collateral asset to futures, and only afterwards is the
//	         futures target advanced so it can place its offsetting slice.
//	Closing: the futures leg is the leader. Each futures-close fill triggers
//	         a TransferOut of the freed collateral back to spot, and only
//	         afterwards is the spot target advanced so it can place its
//	         offsetting slice.
func TestArbitrageRound_DirectionalLeaderFollower(t *testing.T) {
	scenarios := []directionScenario{
		{
			name:                                     "ShortFutures",
			direction:                                types.PositionShort,
			collateral:                               "BTC",
			spotTarget:                               Number(10.0),
			spotOpenSide:                             types.SideTypeBuy,
			futuresCloseSide:                         types.SideTypeBuy,
			expectedFuturesTargetAfterFirstSpotFill:  Number(-5.0),
			expectedSpotTargetAfterFirstFuturesClose: Number(5.0),
			expectedTransferAmount: func(_, quantity fixedpoint.Value) fixedpoint.Value {
				return quantity
			},
		},
		{
			name:                                     "LongFutures",
			direction:                                types.PositionLong,
			collateral:                               "USDT",
			spotTarget:                               Number(-10.0),
			spotOpenSide:                             types.SideTypeSell,
			futuresCloseSide:                         types.SideTypeSell,
			expectedFuturesTargetAfterFirstSpotFill:  Number(5.0),
			expectedSpotTargetAfterFirstFuturesClose: Number(-5.0),
			expectedTransferAmount: func(price, quantity fixedpoint.Value) fixedpoint.Value {
				// Fees are paid in BNB in these tests, so the transfer amount
				// is exactly the quote notional (no fee deduction).
				return price.Mul(quantity)
			},
		},
	}

	for _, sc := range scenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			runLeaderFollowerScenario(t, sc)
		})
	}
}

func runLeaderFollowerScenario(t *testing.T, sc directionScenario) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := TWAPWorkerConfig{
		Duration:      types.Duration(10 * time.Minute),
		NumSlices:     2,
		OrderType:     TWAPOrderTypeMaker,
		CheckInterval: types.Duration(1 * time.Second),
		NumOfTicks:    1,
	}

	nextFundingTime := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	startTime := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)

	spotWorker, spotMockExchange, spotMockOrderQuery, spotGeneralExecutor := newTestTWAPWorker(t, ctrl, config)
	futuresWorker, futuresMockExchange, futuresMockOrderQuery, futuresGeneralExecutor := newTestTWAPWorker(t, ctrl, config)

	spotWorker.SetTargetPosition(sc.spotTarget)

	mockService := &mockFuturesService{}
	fundingRate := &types.PremiumIndex{
		LastFundingRate: Number(0.001),
		NextFundingTime: nextFundingTime,
	}

	round := NewArbitrageRound(
		fundingRate,
		types.ExchangeBinance, types.ExchangeBinance,
		3, 8, Number(3), spotWorker, futuresWorker, mockService,
		sc.direction, time.Minute)
	round.SetLogger(logrus.WithField("test", sc.name))

	assert.Equal(t, sc.collateral, round.CollateralAsset())
	assert.Equal(t, sc.direction, round.syncState.DirectionPolicy.Direction)

	spotOrderID := uint64(1000)
	setupDeltaNeutralMockExchange(spotMockExchange, spotMockOrderQuery, &spotOrderID)
	futuresOrderID := uint64(2000)
	setupDeltaNeutralMockExchange(futuresMockExchange, futuresMockOrderQuery, &futuresOrderID)

	// rebalance() (invoked from round.Tick while closing) queries both accounts.
	// Return empty accounts so the net quote balance is zero and no rebalancing
	// transfer is attempted.
	spotMockExchange.EXPECT().QueryAccount(gomock.Any()).Return(types.NewAccount(), nil).AnyTimes()
	futuresMockExchange.EXPECT().QueryAccount(gomock.Any()).Return(types.NewAccount(), nil).AnyTimes()

	spotSession := &bbgo.ExchangeSession{Exchange: spotMockExchange}
	futuresSession := &bbgo.ExchangeSession{Exchange: futuresMockExchange}

	assertDeltaNeutral := func(t *testing.T, msg string) {
		t.Helper()
		net := spotWorker.FilledPosition().Add(futuresWorker.FilledPosition())
		assert.True(t, net.IsZero(),
			"%s: delta-neutral violated, spot=%s, futures=%s, net=%s",
			msg, spotWorker.FilledPosition(), futuresWorker.FilledPosition(), net)
	}

	orderBook := newOrderBook(76990.0, 100.0, 77010.0, 100.0)
	openPrice := Number(77010.0)
	closePrice := Number(76990.0)
	sliceQty := Number(5.0)
	futuresOpenSide := oppositeSide(sc.spotOpenSide)
	futuresCloseSide := sc.futuresCloseSide
	spotCloseSide := oppositeSide(sc.spotOpenSide)

	// collateralBalance holds far more of the collateral asset than any single
	// transfer needs, so the balance clamp in HandleSpotTrade/HandleFuturesTrade
	// never reduces the transfer amount below the expected value.
	//
	// The magnitude must stay below the int64 fixedpoint ceiling (~9.2e10 in the
	// non-dnum build); a value like 1e12 overflows int64 and, because
	// out-of-range float->int64 conversion is implementation-defined in Go, it
	// becomes +inf on some CPUs (arm64) and -inf on others (amd64). The latter
	// makes fixedpoint.Min clamp the transfer to a negative amount. 1e9 is
	// comfortably larger than any transfer here yet safely representable.
	collateralAccount := types.NewAccount()
	collateralAccount.UpdateBalances(types.BalanceMap{
		sc.collateral: Balance(sc.collateral, Number(1e9)),
	})

	ctx := context.Background()
	assert.NoError(t, round.Start(ctx, spotSession, futuresSession, startTime))
	assert.Equal(t, RoundOpening, round.State())
	assertDeltaNeutral(t, "after start")

	// =========================================================================
	// Opening: spot is the leader.
	//   1. Spot worker places & fills a slice.
	//   2. HandleSpotTrade transfers collateral spot -> futures (TransferIn).
	//   3. Futures target is synced *after* the transfer succeeds; only then
	//      can the futures worker tick its own slice.
	// =========================================================================

	assert.True(t, futuresWorker.TargetPosition().IsZero(),
		"futures target should be zero before any spot fill")

	// --- opening slice 1 ---
	assert.NoError(t, spotWorker.Tick(startTime, orderBook))
	assert.NotNil(t, spotWorker.ActiveOrder())

	transfersBefore := len(mockService.Transfers())
	spotTrade1 := arbSpotTrade(1, spotWorker.ActiveOrder().OrderID, sc.spotOpenSide, openPrice, sliceQty)
	processTrade(spotWorker, spotGeneralExecutor, spotTrade1)
	round.HandleSpotTrade(spotTrade1, collateralAccount, startTime)

	transfers := mockService.Transfers()
	if assert.Len(t, transfers, transfersBefore+1, "spot fill must trigger exactly one transfer") {
		call := transfers[len(transfers)-1]
		assert.Equal(t, sc.collateral, call.Asset)
		assert.Equal(t, types.TransferIn, call.Direction)
		assert.Equal(t, sc.expectedTransferAmount(openPrice, sliceQty), call.Amount)
	}
	assert.Equal(t, sc.expectedFuturesTargetAfterFirstSpotFill, futuresWorker.TargetPosition(),
		"futures target must be advanced after the spot->futures transfer")

	// Futures (the follower) can now place and fill its slice.
	assert.NoError(t, futuresWorker.Tick(startTime, orderBook))
	assert.NotNil(t, futuresWorker.ActiveOrder())

	futuresTrade1 := arbFuturesTrade(101, futuresWorker.ActiveOrder().OrderID, futuresOpenSide, openPrice, sliceQty)
	processTrade(futuresWorker, futuresGeneralExecutor, futuresTrade1)
	round.HandleFuturesTrade(futuresTrade1, collateralAccount, startTime)

	assert.Len(t, mockService.Transfers(), transfersBefore+1,
		"futures fill during opening must not trigger a transfer (spot is the leader)")
	assertDeltaNeutral(t, "after opening slice 1")

	// --- opening slice 2 ---
	tick2 := startTime.Add(5 * time.Minute)
	assert.NoError(t, spotWorker.Tick(tick2, orderBook))

	spotTrade2 := arbSpotTrade(2, spotWorker.ActiveOrder().OrderID, sc.spotOpenSide, openPrice, sliceQty)
	processTrade(spotWorker, spotGeneralExecutor, spotTrade2)
	round.HandleSpotTrade(spotTrade2, collateralAccount, tick2)

	assert.Len(t, mockService.Transfers(), transfersBefore+2)
	assert.Equal(t, sc.spotTarget.Neg(), futuresWorker.TargetPosition())

	assert.NoError(t, futuresWorker.Tick(tick2, orderBook))
	futuresTrade2 := arbFuturesTrade(102, futuresWorker.ActiveOrder().OrderID, futuresOpenSide, openPrice, sliceQty)
	processTrade(futuresWorker, futuresGeneralExecutor, futuresTrade2)
	round.HandleFuturesTrade(futuresTrade2, collateralAccount, tick2)
	assertDeltaNeutral(t, "after opening slice 2 (fully opened)")

	// Transition to RoundReady.
	round.Tick(ctx, startTime.Add(6*time.Minute), orderBook, orderBook)
	assert.Equal(t, RoundReady, round.State())
	assertDeltaNeutral(t, "at RoundReady")
	assert.Equal(t, sc.spotTarget, spotWorker.FilledPosition(),
		"spot leg should be fully opened to its target")
	assert.Equal(t, sc.spotTarget.Neg(), futuresWorker.FilledPosition(),
		"futures leg should fully mirror the spot fill")

	// =========================================================================
	// Closing: futures is the leader.
	//   1. Futures worker places & fills a buy-back/sell-back slice.
	//   2. HandleFuturesTrade transfers freed collateral futures -> spot
	//      (TransferOut).
	//   3. Spot target is synced *after* the transfer succeeds; only then can
	//      the spot worker tick its own closing slice.
	// =========================================================================
	closeTime := startTime.Add(20 * time.Minute)
	round.SetClosing(closeTime, types.Duration(10*time.Minute))
	assert.Equal(t, RoundClosing, round.State())
	assert.Equal(t, fixedpoint.Zero, futuresWorker.TargetPosition(),
		"SetClosing drives the futures target to zero so it leads the close")
	assert.Equal(t, sc.spotTarget, spotWorker.TargetPosition(),
		"spot target stays put until syncSpotPosition advances it per fill")
	assertDeltaNeutral(t, "after SetClosing (before closing trades)")

	transfersBeforeClose := len(mockService.Transfers())

	// --- closing slice 1 ---
	assert.NoError(t, futuresWorker.Tick(closeTime, orderBook))
	assert.NotNil(t, futuresWorker.ActiveOrder())

	futuresCloseTrade1 := arbFuturesTrade(103, futuresWorker.ActiveOrder().OrderID, futuresCloseSide, closePrice, sliceQty)
	processTrade(futuresWorker, futuresGeneralExecutor, futuresCloseTrade1)
	round.HandleFuturesTrade(futuresCloseTrade1, collateralAccount, closeTime)

	transfers = mockService.Transfers()
	if assert.Len(t, transfers, transfersBeforeClose+1, "futures close fill must trigger exactly one transfer") {
		call := transfers[len(transfers)-1]
		assert.Equal(t, sc.collateral, call.Asset)
		assert.Equal(t, types.TransferOut, call.Direction)
		assert.Equal(t, sc.expectedTransferAmount(closePrice, sliceQty), call.Amount)
	}
	assert.Equal(t, sc.expectedSpotTargetAfterFirstFuturesClose, spotWorker.TargetPosition(),
		"spot target must be advanced after the futures->spot transfer")
	// Spot (the follower) can now place and fill its closing slice.
	assert.NoError(t, spotWorker.Tick(closeTime, orderBook))
	spotCloseTrade1 := arbSpotTrade(3, spotWorker.ActiveOrder().OrderID, spotCloseSide, closePrice, sliceQty)
	processTrade(spotWorker, spotGeneralExecutor, spotCloseTrade1)
	round.HandleSpotTrade(spotCloseTrade1, collateralAccount, closeTime)

	assert.Len(t, mockService.Transfers(), transfersBeforeClose+1,
		"spot fill during closing must not trigger a transfer (futures is the leader)")
	assertDeltaNeutral(t, "after closing slice 1")

	// --- closing slice 2 ---
	tick5 := closeTime.Add(5 * time.Minute)
	assert.NoError(t, futuresWorker.Tick(tick5, orderBook))
	futuresCloseTrade2 := arbFuturesTrade(104, futuresWorker.ActiveOrder().OrderID, futuresCloseSide, closePrice, sliceQty)
	processTrade(futuresWorker, futuresGeneralExecutor, futuresCloseTrade2)
	round.HandleFuturesTrade(futuresCloseTrade2, collateralAccount, tick5)

	assert.True(t, futuresWorker.FilledPosition().IsZero())
	assert.True(t, spotWorker.TargetPosition().IsZero())

	assert.NoError(t, spotWorker.Tick(tick5, orderBook))
	spotCloseTrade2 := arbSpotTrade(4, spotWorker.ActiveOrder().OrderID, spotCloseSide, closePrice, sliceQty)
	processTrade(spotWorker, spotGeneralExecutor, spotCloseTrade2)
	round.HandleSpotTrade(spotCloseTrade2, collateralAccount, tick5)

	assertDeltaNeutral(t, "after closing slice 2 (fully closed)")
	assert.True(t, spotWorker.FilledPosition().IsZero())
	assert.True(t, futuresWorker.FilledPosition().IsZero())

	round.Tick(ctx, closeTime.Add(6*time.Minute), orderBook, orderBook)
	assert.Equal(t, RoundClosed, round.State())
	assertDeltaNeutral(t, "at RoundClosed")
}

func oppositeSide(s types.SideType) types.SideType {
	if s == types.SideTypeBuy {
		return types.SideTypeSell
	}
	return types.SideTypeBuy
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
