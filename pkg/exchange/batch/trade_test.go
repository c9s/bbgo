package batch

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func matchTradeQueryOptions(expected *types.TradeQueryOptions) *TradeQueryOptionsMatcher {
	return &TradeQueryOptionsMatcher{
		expected: expected,
	}
}

type TradeQueryOptionsMatcher struct {
	expected *types.TradeQueryOptions
}

func (m TradeQueryOptionsMatcher) Matches(arg interface{}) bool {
	given, ok := arg.(*types.TradeQueryOptions)
	if !ok {
		return false
	}

	if given.StartTime != nil && m.expected.StartTime != nil {
		if !given.StartTime.Equal(*m.expected.StartTime) {
			return false
		}
	}

	if given.EndTime != nil && m.expected.EndTime != nil {
		if !given.EndTime.Equal(*m.expected.EndTime) {
			return false
		}
	}

	if given.Limit != m.expected.Limit {
		return false
	}

	if given.LastTradeID != m.expected.LastTradeID {
		return false
	}

	return true
}

func (m TradeQueryOptionsMatcher) String() string {
	return fmt.Sprintf("%+v", m.expected)
}

func Test_TradeBatchQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		ctx          = context.Background()
		timeNow      = time.Now()
		startTime    = timeNow.Add(-24 * time.Hour)
		endTime      = timeNow
		expSymbol    = "BTCUSDT"
		queryTrades1 = []types.Trade{
			{
				ID:   1,
				Time: types.Time(startTime.Add(time.Hour)),
			},
		}
		queryTrades2 = []types.Trade{
			{
				ID:   2,
				Time: types.Time(startTime.Add(time.Hour)),
			},
			{
				ID:   3,
				Time: types.Time(startTime.Add(2 * time.Hour)),
			},
		}
		emptyTrades = []types.Trade{}
		allRes      = append(queryTrades1, queryTrades2...)
	)

	t.Run("succeeds", func(t *testing.T) {
		var (
			expOptions = &types.TradeQueryOptions{
				StartTime:   &startTime,
				EndTime:     &endTime,
				LastTradeID: 0,
				Limit:       50,
			}
			mockExchange = mocks.NewMockExchangeTradeHistoryService(ctrl)
		)

		mockExchange.EXPECT().QueryTrades(ctx, expSymbol, matchTradeQueryOptions(expOptions)).DoAndReturn(
			func(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
				assert.Equal(t, startTime, *options.StartTime)
				assert.Equal(t, endTime, *options.EndTime)
				assert.Equal(t, uint64(0), options.LastTradeID)
				assert.Equal(t, expOptions.Limit, options.Limit)
				return queryTrades1, nil
			}).Times(1)

		mockExchange.EXPECT().QueryTrades(ctx, expSymbol, matchTradeQueryOptions(&types.TradeQueryOptions{
			StartTime:   timePtr(queryTrades1[0].Time.Time()),
			EndTime:     expOptions.EndTime,
			LastTradeID: 1,
			Limit:       50,
		})).DoAndReturn(
			func(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
				assert.Equal(t, queryTrades1[0].Time.Time(), *options.StartTime)
				assert.Equal(t, endTime, *options.EndTime)
				assert.Equal(t, queryTrades1[0].ID, options.LastTradeID)
				assert.Equal(t, expOptions.Limit, options.Limit)
				return queryTrades2, nil
			}).Times(1)

		mockExchange.EXPECT().QueryTrades(ctx, expSymbol, matchTradeQueryOptions(&types.TradeQueryOptions{
			StartTime:   timePtr(queryTrades2[1].Time.Time()),
			EndTime:     expOptions.EndTime,
			LastTradeID: queryTrades2[1].ID,
			Limit:       50,
		})).DoAndReturn(
			func(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
				assert.Equal(t, queryTrades2[1].Time.Time(), *options.StartTime)
				assert.Equal(t, endTime, *options.EndTime)
				assert.Equal(t, queryTrades2[1].ID, options.LastTradeID)
				assert.Equal(t, expOptions.Limit, options.Limit)
				return emptyTrades, nil
			}).Times(1)

		tradeBatchQuery := &TradeBatchQuery{ExchangeTradeHistoryService: mockExchange}

		resCh, errC := tradeBatchQuery.Query(ctx, expSymbol, expOptions)
		wg := sync.WaitGroup{}
		wg.Add(1)
		rcvCount := 0
		go func() {
			defer wg.Done()
			for ch := range resCh {
				assert.Equal(t, allRes[rcvCount], ch)
				rcvCount++
			}
			assert.NoError(t, <-errC)
		}()
		wg.Wait()
		assert.Equal(t, rcvCount, len(allRes))
	})

	t.Run("failed to call query trades", func(t *testing.T) {
		var (
			expOptions = &types.TradeQueryOptions{
				StartTime:   &startTime,
				EndTime:     &endTime,
				LastTradeID: 0,
				Limit:       50,
			}
			mockExchange = mocks.NewMockExchangeTradeHistoryService(ctrl)
			unknownErr   = errors.New("unknown err")
		)

		mockExchange.EXPECT().QueryTrades(ctx, expSymbol, expOptions).DoAndReturn(
			func(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
				assert.Equal(t, startTime, *options.StartTime)
				assert.Equal(t, endTime, *options.EndTime)
				assert.Equal(t, uint64(0), options.LastTradeID)
				assert.Equal(t, expOptions.Limit, options.Limit)
				return nil, unknownErr
			}).Times(1)

		tradeBatchQuery := &TradeBatchQuery{ExchangeTradeHistoryService: mockExchange}

		resCh, errC := tradeBatchQuery.Query(ctx, expSymbol, expOptions)
		wg := sync.WaitGroup{}
		wg.Add(1)
		rcvCount := 0
		go func() {
			defer wg.Done()
			for ch := range resCh {
				assert.Equal(t, allRes[rcvCount], ch)
				rcvCount++
			}
			assert.Equal(t, 0, rcvCount)
			assert.Equal(t, unknownErr, <-errC)
		}()
		wg.Wait()
		assert.Equal(t, rcvCount, 0)
	})
}

func timePtr(t time.Time) *time.Time {
	return &t
}
