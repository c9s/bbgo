package grid2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
)

func mustNewTime(v string) time.Time {
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		panic(err)
	}

	return t
}

var testClosedOrderID = uint64(0)

func newClosedLimitOrder(symbol string, side types.SideType, price, quantity fixedpoint.Value, ta ...time.Time) types.Order {
	testClosedOrderID++
	creationTime := time.Now()
	updateTime := creationTime

	if len(ta) > 0 {
		creationTime = ta[0]
		if len(ta) > 1 {
			updateTime = ta[1]
		}
	}

	return types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   symbol,
			Side:     side,
			Type:     types.OrderTypeLimit,
			Quantity: quantity,
			Price:    price,
		},
		Exchange:         types.ExchangeBinance,
		OrderID:          testClosedOrderID,
		Status:           types.OrderStatusFilled,
		ExecutedQuantity: quantity,
		CreationTime:     types.Time(creationTime),
		UpdateTime:       types.Time(updateTime),
	}
}

func TestProfitFixer(t *testing.T) {
	testClosedOrderID = 0

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	ctx := context.Background()
	mockHistoryService := mocks.NewMockExchangeTradeHistoryService(mockCtrl)

	// mock QueryTrades: return trades with fee data for the filled orders
	mockHistoryService.EXPECT().QueryTrades(gomock.Any(), "ETHUSDT", gomock.Any()).
		Return([]types.Trade{
			{ID: 1, OrderID: 3, Symbol: "ETHUSDT", Side: types.SideTypeSell, Fee: number(0.018), FeeCurrency: "USDT", Price: number(1800), Quantity: number(0.1), Time: types.Time(mustNewTime("2022-01-01T00:01:00Z"))},
			{ID: 2, OrderID: 4, Symbol: "ETHUSDT", Side: types.SideTypeSell, Fee: number(0.019), FeeCurrency: "USDT", Price: number(1900), Quantity: number(0.1), Time: types.Time(mustNewTime("2022-01-01T00:03:00Z"))},
			{ID: 3, OrderID: 9, Symbol: "ETHUSDT", Side: types.SideTypeSell, Fee: number(0.018), FeeCurrency: "USDT", Price: number(1800), Quantity: number(0.1), Time: types.Time(mustNewTime("2022-01-01T00:04:00Z"))},
			{ID: 4, OrderID: 10, Symbol: "ETHUSDT", Side: types.SideTypeSell, Fee: number(0.019), FeeCurrency: "USDT", Price: number(1900), Quantity: number(0.1), Time: types.Time(mustNewTime("2022-01-01T00:08:00Z"))},
		}, nil).AnyTimes()

	mockHistoryService.EXPECT().QueryClosedOrders(gomock.Any(), "ETHUSDT", mustNewTime("2022-01-01T00:00:00Z"), mustNewTime("2022-01-07T00:00:00Z"), uint64(0)).
		Return([]types.Order{
			newClosedLimitOrder("ETHUSDT", types.SideTypeBuy, number(1800.0), number(0.1), mustNewTime("2022-01-01T00:01:00Z")),
			newClosedLimitOrder("ETHUSDT", types.SideTypeBuy, number(1700.0), number(0.1), mustNewTime("2022-01-01T00:01:00Z")),
			newClosedLimitOrder("ETHUSDT", types.SideTypeSell, number(1800.0), number(0.1), mustNewTime("2022-01-01T00:01:00Z")),
			newClosedLimitOrder("ETHUSDT", types.SideTypeSell, number(1900.0), number(0.1), mustNewTime("2022-01-01T00:03:00Z")),
			newClosedLimitOrder("ETHUSDT", types.SideTypeSell, number(1905.0), number(0.1), mustNewTime("2022-01-01T00:03:00Z")),
		}, nil)

	mockHistoryService.EXPECT().QueryClosedOrders(gomock.Any(), "ETHUSDT", mustNewTime("2022-01-01T00:03:00Z"), mustNewTime("2022-01-07T00:00:00Z"), uint64(5)).
		Return([]types.Order{
			newClosedLimitOrder("ETHUSDT", types.SideTypeBuy, number(1900.0), number(0.1), mustNewTime("2022-01-01T00:04:00Z")),
			newClosedLimitOrder("ETHUSDT", types.SideTypeBuy, number(1800.0), number(0.1), mustNewTime("2022-01-01T00:04:00Z")),
			newClosedLimitOrder("ETHUSDT", types.SideTypeBuy, number(1700.0), number(0.1), mustNewTime("2022-01-01T00:04:00Z")),
			newClosedLimitOrder("ETHUSDT", types.SideTypeSell, number(1800.0), number(0.1), mustNewTime("2022-01-01T00:04:00Z")),
			newClosedLimitOrder("ETHUSDT", types.SideTypeSell, number(1900.0), number(0.1), mustNewTime("2022-01-01T00:08:00Z")),
		}, nil)

	mockHistoryService.EXPECT().QueryClosedOrders(gomock.Any(), "ETHUSDT", mustNewTime("2022-01-01T00:08:00Z"), mustNewTime("2022-01-07T00:00:00Z"), uint64(10)).
		Return([]types.Order{}, nil)

	grid := NewGrid(number(1000.0), number(2000.0), number(11), number(0.01))
	grid.CalculateArithmeticPins()

	since, err := time.Parse(time.RFC3339, "2022-01-01T00:00:00Z")
	assert.NoError(t, err)

	until, err := time.Parse(time.RFC3339, "2022-01-07T00:00:00Z")
	assert.NoError(t, err)

	stats := &GridProfitStats{}
	fixer := newProfitFixer(grid, "ETHUSDT", mockHistoryService)
	err = fixer.Fix(ctx, since, until, 0, stats)
	assert.NoError(t, err)

	assert.Equal(t, "40", stats.TotalQuoteProfit.String())
	assert.Equal(t, 4, stats.ArbitrageCount)

	// verify daily fee was aggregated
	dateKey := getDateKey(mustNewTime("2022-01-01T00:00:00Z"))
	assert.NotNil(t, stats.DailyFee[dateKey])
	// trades for orders 3,4,9,10 all have USDT fees: 0.018 + 0.019 + 0.018 + 0.019
	expectedFee := number(0.018).Add(number(0.019)).Add(number(0.018)).Add(number(0.019))
	assert.Equal(t, expectedFee, stats.DailyFee[dateKey]["USDT"])
}
