package dca2

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/types/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func Test_NewCollector(t *testing.T) {
	symbol := "ETHUSDT"
	logger := log.WithField("strategy", ID)

	t.Run("return nil if the exchange doesn't support ExchangeTradeHistoryService", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockEx := mocks.NewMockExchange(mockCtrl)
		mockEx.EXPECT().Name().Return(types.ExchangeMax)

		collector := NewCollector(logger, symbol, 0, false, mockEx)

		assert.Nil(t, collector)
	})

	t.Run("return nil if the exchange doesn't support ExchangeOrderQueryService", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockEx := mocks.NewMockExchange(mockCtrl)
		mockEx.EXPECT().Name().Return(types.ExchangeMax)

		mockTradeHistoryService := mocks.NewMockExchangeTradeHistoryService(mockCtrl)

		type TestEx struct {
			types.Exchange
			types.ExchangeTradeHistoryService
		}

		ex := TestEx{
			Exchange:                    mockEx,
			ExchangeTradeHistoryService: mockTradeHistoryService,
		}

		collector := NewCollector(logger, symbol, 0, false, ex)

		assert.Nil(t, collector)
	})

	t.Run("return nil if the exchange doesn't support descendingClosedOrderQueryService", func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		mockEx := mocks.NewMockExchange(mockCtrl)
		mockEx.EXPECT().Name().Return(types.ExchangeMax)

		mockTradeHistoryService := mocks.NewMockExchangeTradeHistoryService(mockCtrl)
		mockOrderQueryService := mocks.NewMockExchangeOrderQueryService(mockCtrl)

		type TestEx struct {
			types.Exchange
			types.ExchangeTradeHistoryService
			types.ExchangeOrderQueryService
		}

		ex := TestEx{
			Exchange:                    mockEx,
			ExchangeTradeHistoryService: mockTradeHistoryService,
			ExchangeOrderQueryService:   mockOrderQueryService,
		}

		collector := NewCollector(logger, symbol, 0, false, ex)

		assert.Nil(t, collector)
	})
}
