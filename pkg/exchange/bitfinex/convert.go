package bitfinex

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate go run generate_symbol_map.go
var stableCoinRE = regexp.MustCompile(`(EUR|GBP|TUSD|UST|UDC|USD[TC]*)`)

var primaryLocalCurrencyMap = map[string]string{
	"USDC": "UDC",
	"USDT": "UST",
	"TUSD": "TSD",
	"MANA": "MNA",
	"WBTC": "WBT",
}

func toGlobalSymbol(symbol string) string {
	symbol = strings.TrimLeft(symbol, "tf")
	s, ok := localSymbolMap[symbol]
	if ok {
		return s
	}

	return symbol
}

func splitLocalSymbol(symbol string) (string, string) {
	symbol = strings.TrimLeft(symbol, "tf")

	// if the symbol contains ":", we can split it directly
	if strings.Contains(symbol, ":") {
		parts := strings.SplitN(symbol, ":", 2)
		if len(parts) != 2 {
			log.Errorf("unable to handle symbol: %s", symbol)
		} else {
			return parts[0], parts[1]
		}
	}

	indexes := stableCoinRE.FindStringSubmatchIndex(symbol)
	if len(indexes) < 1 {
		// if the symbol does not match the expected format, return it as is
		return symbol, ""
	}

	if indexes[0] == 0 {
		return toLocalCurrency(symbol[indexes[0]:indexes[1]]), toLocalCurrency(symbol[indexes[1]:])
	}

	return toLocalCurrency(symbol[:indexes[0]]), toLocalCurrency(symbol[indexes[0]:])
}

func toLocalSymbol(symbol string) string {
	s, ok := globalSymbolMap[symbol]
	if ok {
		return "t" + s
	}

	indexes := stableCoinRE.FindStringSubmatchIndex(symbol)
	if len(indexes) < 1 {
		// if the symbol does not match the expected format, return it as is
		return "t" + symbol
	}

	if indexes[0] == 0 {
		return "t" + toLocalCurrency(symbol[indexes[0]:indexes[1]]) + toLocalCurrency(symbol[indexes[1]:])
	}

	return "t" + toLocalCurrency(symbol[:indexes[0]]) + toLocalCurrency(symbol[indexes[0]:])
}

func toGlobalCurrency(c string) string {
	c = strings.TrimLeft(c, "tf")

	s, ok := localCurrencyMap[c]
	if ok {
		return s
	}

	return c
}

func toLocalCurrency(c string) string {
	s, ok := primaryLocalCurrencyMap[c]
	if ok {
		return s
	}

	s, ok = globalCurrencyMap[c]
	if ok {
		return s
	}

	return c
}

// toGlobalOrder converts bfxapi.Order to types.Order
func toGlobalOrder(o bfxapi.Order) *types.Order {
	// map bfxapi.Order to types.Order using struct literal
	order := &types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:       toGlobalSymbol(o.Symbol),
			Price:        o.Price,
			Quantity:     o.AmountOrig,
			Type:         toGlobalOrderType(o.OrderType),
			AveragePrice: o.PriceAvg,
		},
		OrderID:          uint64(o.OrderID),
		ExecutedQuantity: o.AmountOrig.Sub(o.Amount),
		Status:           toGlobalOrderStatus(o.Status),
		CreationTime:     types.Time(o.CreatedAt),
		UpdateTime:       types.Time(o.UpdatedAt),
		OriginalStatus:   string(o.Status), // keep original status for reference
		UUID:             "",               // Bitfinex does not provide UUID field
		Exchange:         ID,
	}

	// map ClientOrderID if present
	if o.ClientOrderID != nil {
		order.ClientOrderID = strconv.FormatInt(*o.ClientOrderID, 10)
	}

	// set IsWorking based on status
	order.IsWorking = order.Status == types.OrderStatusNew || order.Status == types.OrderStatusPartiallyFilled
	return order
}

// toGlobalOrderStatus maps bfxapi.OrderStatus to types.OrderStatus.
// It normalizes Bitfinex order status string to bbgo's types.OrderStatus.
func toGlobalOrderStatus(status bfxapi.OrderStatus) types.OrderStatus {
	switch status {
	case bfxapi.OrderStatusActive:
		return types.OrderStatusNew
	case bfxapi.OrderStatusExecuted:
		return types.OrderStatusFilled
	case bfxapi.OrderStatusPartiallyFilled:
		return types.OrderStatusPartiallyFilled
	case bfxapi.OrderStatusCanceled, bfxapi.OrderStatusPartiallyCanceled:
		return types.OrderStatusCanceled
	case bfxapi.OrderStatusRejected, bfxapi.OrderStatusInsufficientBal:
		return types.OrderStatusRejected
	case bfxapi.OrderStatusExpired:
		return types.OrderStatusExpired
	case bfxapi.OrderStatusPending:
		return types.OrderStatusNew
	default:
		return types.OrderStatusNew // fallback to new
	}
}

// convertTrade converts bfxapi.OrderTradeDetail to types.Trade
func convertTrade(trade bfxapi.OrderTradeDetail) *types.Trade {
	// map bfxapi.OrderTradeDetail to types.Trade using struct literal
	return &types.Trade{
		ID:            uint64(trade.TradeID),
		OrderID:       uint64(trade.OrderID),
		Exchange:      ID,
		Price:         trade.ExecPrice,
		Quantity:      trade.ExecAmount,
		QuoteQuantity: trade.ExecPrice.Mul(trade.ExecAmount),
		Symbol:        toGlobalSymbol(trade.Symbol),
		Side: func() types.SideType {
			if trade.ExecAmount.Sign() > 0 {
				return types.SideTypeBuy
			}
			return types.SideTypeSell
		}(),
		IsBuyer:     trade.ExecAmount.Sign() > 0,
		IsMaker:     trade.Maker == 1,
		Time:        types.Time(trade.Time),
		Fee:         trade.Fee,
		FeeCurrency: trade.FeeCurrency,
	}
}

// convertTicker converts bfxapi.Ticker to types.Ticker.
// It maps Bitfinex ticker fields to the standard types.Ticker fields.
func convertTicker(t bfxapi.Ticker) *types.Ticker {
	return &types.Ticker{
		Volume: t.Volume,
		Last:   t.LastPrice,
		High:   t.High,
		Low:    t.Low,
		Buy:    t.Bid,
		Sell:   t.Ask,
	}
}

// toGlobalOrderType maps bfxapi.OrderType to types.OrderType.
// It normalizes Bitfinex order type string to bbgo's types.OrderType.
func toGlobalOrderType(t bfxapi.OrderType) types.OrderType {
	switch t {
	case bfxapi.OrderTypeLimit, bfxapi.OrderTypeExchangeLimit:
		return types.OrderTypeLimit
	case bfxapi.OrderTypeMarket, bfxapi.OrderTypeExchangeMarket:
		return types.OrderTypeMarket
	case bfxapi.OrderTypeStopLimit, bfxapi.OrderTypeExchangeStopLimit:
		return types.OrderTypeStopLimit
	case bfxapi.OrderTypeStop, bfxapi.OrderTypeExchangeStop:
		return types.OrderTypeStopMarket
	case bfxapi.OrderTypeTrailingStop, bfxapi.OrderTypeExchangeTrailingStop:
		return types.OrderTypeStopMarket // fallback to stop market
	case bfxapi.OrderTypeFOK, bfxapi.OrderTypeExchangeFOK:
		return types.OrderTypeLimit // fallback to limit
	case bfxapi.OrderTypeIOC, bfxapi.OrderTypeExchangeIOC:
		return types.OrderTypeLimit // fallback to limit
	default:
		return types.OrderTypeLimit // fallback to limit
	}
}

// convertBookEntries converts a slice of bfxapi.BookEntry to types.SliceOrderBook.
// It maps Bitfinex book entries to the standard SliceOrderBook fields.
func convertBookEntries(entries []bfxapi.BookEntry) types.SliceOrderBook {
	var ob types.SliceOrderBook
	for _, entry := range entries {
		if entry.Amount.Sign() > 0 {
			ob.Bids = append(ob.Bids, types.PriceVolume{
				Price:  entry.Price,
				Volume: entry.Amount,
			})
		} else if entry.Amount.Sign() < 0 {
			ob.Asks = append(ob.Asks, types.PriceVolume{
				Price:  entry.Price,
				Volume: entry.Amount.Neg(),
			})
		}
	}
	return ob
}

// convertDepth converts bfxapi.BookResponse to types.SliceOrderBook.
// It delegates to convertBookEntries for BookEntries.
func convertDepth(resp *bfxapi.BookResponse) types.SliceOrderBook {
	return convertBookEntries(resp.BookEntries)
}
