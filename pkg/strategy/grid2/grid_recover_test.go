package grid2

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

type TestData struct {
	Market       types.Market  `json:"market" yaml:"market"`
	Strategy     Strategy      `json:"strategy" yaml:"strategy"`
	OpenOrders   []types.Order `json:"openOrders" yaml:"openOrders"`
	ClosedOrders []types.Order `json:"closedOrders" yaml:"closedOrders"`
	Trades       []types.Trade `json:"trades" yaml:"trades"`
}

type TestDataService struct {
	Orders map[string]types.Order
	Trades []types.Trade
}

func (t *TestDataService) QueryTrades(ctx context.Context, symbol string, options *types.TradeQueryOptions) ([]types.Trade, error) {
	var i int = 0
	if options.LastTradeID != 0 {
		for idx, trade := range t.Trades {
			if trade.ID < options.LastTradeID {
				continue
			}

			i = idx
			break
		}
	}

	var trades []types.Trade
	l := len(t.Trades)
	for ; i < l && len(trades) < int(options.Limit); i++ {
		trades = append(trades, t.Trades[i])
	}

	return trades, nil
}

func (t *TestDataService) QueryOrder(ctx context.Context, q types.OrderQuery) (*types.Order, error) {
	if len(q.OrderID) == 0 {
		return nil, fmt.Errorf("order id should not be empty")
	}

	order, exist := t.Orders[q.OrderID]
	if !exist {
		return nil, fmt.Errorf("order not found")
	}

	return &order, nil
}

// dummy method for interface
func (t *TestDataService) QueryClosedOrders(ctx context.Context, symbol string, since, until time.Time, lastOrderID uint64) (orders []types.Order, err error) {
	return nil, nil
}

// dummy method for interface
func (t *TestDataService) QueryOrderTrades(ctx context.Context, q types.OrderQuery) ([]types.Trade, error) {
	return nil, nil
}

func NewStrategy(t *TestData) *Strategy {
	s := t.Strategy
	s.Debug = true
	s.Initialize()
	s.Market = t.Market
	s.Position = types.NewPositionFromMarket(t.Market)
	s.orderExecutor = bbgo.NewGeneralOrderExecutor(&bbgo.ExchangeSession{}, t.Market.Symbol, ID, s.InstanceID(), s.Position)
	return &s
}

func NewTestDataService(t *TestData) *TestDataService {
	var orders map[string]types.Order = make(map[string]types.Order)
	for _, order := range t.OpenOrders {
		orders[strconv.FormatUint(order.OrderID, 10)] = order
	}

	for _, order := range t.ClosedOrders {
		orders[strconv.FormatUint(order.OrderID, 10)] = order
	}

	trades := t.Trades
	sort.Slice(t.Trades, func(i, j int) bool {
		return trades[i].ID < trades[j].ID
	})

	return &TestDataService{
		Orders: orders,
		Trades: trades,
	}
}

func readSpec(fileName string) (*TestData, error) {
	content, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	market := types.Market{}
	if err := json.Unmarshal(content, &market); err != nil {
		return nil, err
	}

	strategy := Strategy{}
	if err := json.Unmarshal(content, &strategy); err != nil {
		return nil, err
	}

	data := TestData{
		Market:   market,
		Strategy: strategy,
	}
	return &data, nil
}

func readOrdersFromCSV(fileName string) ([]types.Order, error) {
	csvFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer csvFile.Close()
	csvReader := csv.NewReader(csvFile)

	keys, err := csvReader.Read()
	if err != nil {
		return nil, err
	}

	var orders []types.Order
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if len(row) != len(keys) {
			return nil, fmt.Errorf("length of row should be equal to length of keys")
		}

		var m map[string]interface{} = make(map[string]interface{})
		for i, key := range keys {
			if key == "orderID" {
				x, err := strconv.ParseUint(row[i], 10, 64)
				if err != nil {
					return nil, err
				}
				m[key] = x
			} else {
				m[key] = row[i]
			}
		}

		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}

		order := types.Order{}
		if err = json.Unmarshal(b, &order); err != nil {
			return nil, err
		}

		orders = append(orders, order)
	}

	return orders, nil
}

func readTradesFromCSV(fileName string) ([]types.Trade, error) {
	csvFile, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer csvFile.Close()
	csvReader := csv.NewReader(csvFile)

	keys, err := csvReader.Read()
	if err != nil {
		return nil, err
	}

	var trades []types.Trade
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return nil, err
		}

		if len(row) != len(keys) {
			return nil, fmt.Errorf("length of row should be equal to length of keys")
		}

		var m map[string]interface{} = make(map[string]interface{})
		for i, key := range keys {
			switch key {
			case "id", "orderID":
				x, err := strconv.ParseUint(row[i], 10, 64)
				if err != nil {
					return nil, err
				}
				m[key] = x
			default:
				m[key] = row[i]
			}
		}

		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}

		trade := types.Trade{}
		if err = json.Unmarshal(b, &trade); err != nil {
			return nil, err
		}

		trades = append(trades, trade)
	}

	return trades, nil
}

func readTestDataFrom(fileDir string) (*TestData, error) {
	data, err := readSpec(fmt.Sprintf("%s/spec", fileDir))
	if err != nil {
		return nil, err
	}

	openOrders, err := readOrdersFromCSV(fmt.Sprintf("%s/open_orders.csv", fileDir))
	if err != nil {
		return nil, err
	}

	closedOrders, err := readOrdersFromCSV(fmt.Sprintf("%s/closed_orders.csv", fileDir))
	if err != nil {
		return nil, err
	}

	trades, err := readTradesFromCSV(fmt.Sprintf("%s/trades.csv", fileDir))
	if err != nil {
		return nil, err
	}

	data.OpenOrders = openOrders
	data.ClosedOrders = closedOrders
	data.Trades = trades
	return data, nil
}

func TestRecoverByScanningTrades(t *testing.T) {
	assert := assert.New(t)

	t.Run("test case 1", func(t *testing.T) {
		fileDir := "recovery_testcase/testcase1/"

		data, err := readTestDataFrom(fileDir)
		if !assert.NoError(err) {
			return
		}

		testService := NewTestDataService(data)
		strategy := NewStrategy(data)
		filledOrders, err := strategy.getFilledOrdersByScanningTrades(context.Background(), testService, testService, data.OpenOrders)
		if !assert.NoError(err) {
			return
		}

		assert.Len(filledOrders, 0)
	})
}
