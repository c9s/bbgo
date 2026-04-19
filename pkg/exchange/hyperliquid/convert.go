package hyperliquid

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/hyperliquid/hyperapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalSpotMarket(s hyperapi.UniverseMeta, tokens []hyperapi.TokenMeta) types.Market {
	base, quote := tokens[s.Tokens[0]], tokens[s.Tokens[1]]
	tickSize := fixedpoint.NewFromFloat(math.Pow10(-quote.SzDecimals))
	stepSize := fixedpoint.NewFromFloat(math.Pow10(-base.SzDecimals))

	return types.Market{
		Exchange:        types.ExchangeHyperliquid,
		Symbol:          base.Name + quote.Name,
		LocalSymbol:     base.Name + "@" + strconv.Itoa(s.Index),
		BaseCurrency:    base.Name,
		QuoteCurrency:   quote.Name,
		TickSize:        tickSize,
		StepSize:        stepSize,
		MinPrice:        fixedpoint.Zero, // not used
		MaxPrice:        fixedpoint.Zero, // not used
		MinNotional:     stepSize.Mul(tickSize),
		MinAmount:       stepSize,
		MinQuantity:     stepSize,
		MaxQuantity:     fixedpoint.NewFromFloat(1e9),
		PricePrecision:  quote.SzDecimals,
		VolumePrecision: base.SzDecimals,
	}
}

func toLocalSpotSymbol(symbol string) (string, int) {
	if s, ok := spotSymbolSyncMap.Load(symbol); ok {
		if localSymbol, ok := s.(string); ok {
			at := strings.LastIndexByte(localSymbol, '@')
			if at < 0 || at+1 >= len(localSymbol) {
				log.Errorf("invalid local symbol format %q for %s", localSymbol, symbol)
				return symbol, -1
			}

			if asset, err := strconv.Atoi(localSymbol[at+1:]); err == nil {
				return localSymbol[:at], asset + 1000
			}
		}

		log.Errorf("failed to convert symbol %s to local symbol and asset, but found in spotSymbolSyncMap", symbol)
	}

	log.Errorf("failed to look up local symbol and asset from %s", symbol)
	return symbol, -1
}

func toLocalFuturesSymbol(symbol string) (string, int) {
	if s, ok := futuresSymbolSyncMap.Load(symbol); ok {
		if localSymbol, ok := s.(string); ok {
			at := strings.LastIndexByte(localSymbol, '@')
			if at < 0 || at+1 >= len(localSymbol) {
				log.Errorf("invalid local symbol format %q for %s", localSymbol, symbol)
				return symbol, -1
			}

			if asset, err := strconv.Atoi(localSymbol[at+1:]); err == nil {
				return localSymbol[:at], asset
			}
		}

		log.Errorf("failed to convert symbol %s to local symbol and asset, but found in futuresSymbolSyncMaps", symbol)
	}

	log.Errorf("failed to look up local symbol and asset from %s", symbol)
	return symbol, -1
}

func toGlobalSymbol(localSymbol string, isFutures bool) string {
	symbolMap := &spotSymbolSyncMap
	if isFutures {
		symbolMap = &futuresSymbolSyncMap
	}

	if symbol, ok := lookupGlobalSymbol(symbolMap, localSymbol); ok {
		return symbol
	}

	log.Errorf("failed to look up global symbol symbol %s", localSymbol)
	return localSymbol
}

func lookupGlobalSymbol(symbolMap *sync.Map, localSymbol string) (string, bool) {
	var symbol string
	var found bool

	inputCoin := localSymbol
	inputHasAt := strings.LastIndexByte(inputCoin, '@') > 0
	if at := strings.LastIndexByte(localSymbol, '@'); at > 0 {
		inputCoin = localSymbol[:at]
	}

	symbolMap.Range(func(key, value any) bool {
		mapLocalSymbol, ok := value.(string)
		if !ok {
			return true
		}

		if inputHasAt {
			if mapLocalSymbol != localSymbol {
				return true
			}
		} else {
			if at := strings.LastIndexByte(mapLocalSymbol, '@'); at > 0 {
				mapLocalSymbol = mapLocalSymbol[:at]
			}
			if mapLocalSymbol != inputCoin {
				return true
			}
		}

		globalSymbol, ok := key.(string)
		if !ok {
			return true
		}

		symbol = globalSymbol
		found = true
		return false
	})

	return symbol, found
}

func toGlobalBalance(account *hyperapi.Account) types.BalanceMap {
	balances := make(types.BalanceMap)
	for _, b := range account.Balances {
		available := b.Total.Sub(b.Hold)
		balances[b.Coin] = types.Balance{
			Currency:          b.Coin,
			Available:         available,
			Locked:            b.Hold,
			NetAsset:          b.Total,
			MaxWithdrawAmount: available,
		}
	}
	return balances
}

func toGlobalFuturesAccountInfo(rawAccount *hyperapi.FuturesAccount) *types.FuturesAccount {
	account := &types.FuturesAccount{
		Assets:    make(types.FuturesAssetMap),
		Positions: make(types.FuturesPositionMap),
	}
	account.TotalMarginBalance = rawAccount.MarginSummary.AccountValue
	account.TotalWalletBalance = rawAccount.MarginSummary.TotalRawUsd
	account.TotalInitialMargin = rawAccount.MarginSummary.TotalMarginUsed
	account.TotalMaintMargin = rawAccount.CrossMaintenanceMarginUsed
	account.AvailableBalance = rawAccount.Withdrawable

	for _, asset := range rawAccount.AssetPositions {
		p := asset.Position
		symbol := p.Coin + QuoteCurrency
		positionSide := types.PositionLong
		if p.Szi.Sign() < 0 {
			positionSide = types.PositionShort
		}

		posKey := types.NewPositionKey(symbol, positionSide)
		account.Positions[posKey] = types.FuturesPosition{
			Symbol:        symbol,
			PositionSide:  positionSide,
			BaseCurrency:  p.Coin,
			QuoteCurrency: QuoteCurrency,
			Base:          p.Szi.Abs(),
			Quote:         p.PositionValue,
			Isolated:      p.Leverage.Type == "isolated",
			PositionRisk:  toGlobalPositionRisk(asset.Position),
			UpdateTime:    rawAccount.Time,
		}
	}

	return account
}

func toGlobalPositionRisk(p hyperapi.FuturesPosition) *types.PositionRisk {
	markPrice := p.EntryPx.Add(p.UnrealizedPnl.Div(p.Szi))
	side := types.PositionLong
	if p.Szi.Sign() < 0 {
		side = types.PositionShort
	}
	return &types.PositionRisk{
		Leverage:              p.Leverage.Value,
		Symbol:                p.Coin + QuoteCurrency,
		EntryPrice:            p.EntryPx,
		LiquidationPrice:      p.LiquidationPx,
		PositionAmount:        p.Szi,
		UnrealizedPnL:         p.UnrealizedPnl,
		InitialMargin:         p.MarginUsed,
		MarkPrice:             markPrice,
		PositionInitialMargin: p.MarginUsed,
		Notional:              p.PositionValue,
		PositionSide:          side,
	}
}

func toLocalInterval(interval types.Interval) (string, error) {
	if _, ok := SupportedIntervals[interval]; !ok {
		return "", fmt.Errorf("interval %s is not supported", interval)
	}

	in, ok := localInterval[interval]
	if !ok {
		return "", fmt.Errorf("interval %s is not supported, got local interval %s", interval, in)
	}

	return in, nil
}

func kLineToGlobal(k hyperapi.KLine, interval types.Interval, isFutures bool) types.KLine {
	symbol := toGlobalSymbol(k.Symbol, isFutures)
	return types.KLine{
		Exchange:                 types.ExchangeHyperliquid,
		Symbol:                   symbol,
		StartTime:                types.Time(k.StartTime),
		EndTime:                  types.Time(k.EndTime),
		Interval:                 interval,
		Open:                     k.OpenPrice,
		Close:                    k.ClosePrice,
		High:                     k.HighestPrice,
		Low:                      k.LowestPrice,
		Volume:                   k.Volume,
		NumberOfTrades:           k.Trades,
		QuoteVolume:              fixedpoint.Zero, // not supported
		TakerBuyBaseAssetVolume:  fixedpoint.Zero, // not supported
		TakerBuyQuoteAssetVolume: fixedpoint.Zero, // not supported
		LastTradeID:              0,               // not supported
		Closed:                   true,
	}
}

func toGlobalOrder(order hyperapi.OpenOrder, isFutures bool) types.Order {
	timeInForce := toGlobalTimeInForce(order.Tif)
	orderType := toGlobalOrderType(order, timeInForce)
	symbol := toGlobalSymbol(order.Coin, isFutures)

	return types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:      symbol,
			Price:       order.LimitPx,
			Quantity:    order.Sz,
			Side:        toGlobalSide(order.Side),
			Type:        orderType,
			TimeInForce: timeInForce,
		},
		Exchange:     types.ExchangeHyperliquid,
		OrderID:      uint64(order.Oid),
		CreationTime: types.Time(order.Timestamp),
		UpdateTime:   types.Time(order.Timestamp),
		IsFutures:    isFutures,
	}
}

func toGlobalTimeInForce(tif string) types.TimeInForce {
	switch strings.ToLower(strings.TrimSpace(tif)) {
	case "gtc":
		return types.TimeInForceGTC
	case "ioc":
		return types.TimeInForceIOC
	case "alo":
		return types.TimeInForceALO
	default:
		return types.TimeInForceGTC
	}
}

func toGlobalOrderType(order hyperapi.OpenOrder, tif types.TimeInForce) types.OrderType {
	orderType := strings.ToLower(strings.TrimSpace(order.OrderType))
	triggerCondition := strings.ToLower(strings.TrimSpace(order.TriggerCondition))

	if !order.IsTrigger {
		switch orderType {
		case "market":
			return types.OrderTypeMarket
		case "limit":
			if tif == types.TimeInForceALO {
				return types.OrderTypeLimitMaker
			}
			return types.OrderTypeLimit
		default:
			return types.OrderTypeLimit
		}
	}

	isTakeProfit := strings.Contains(triggerCondition, "tp") ||
		strings.Contains(triggerCondition, "profit")
	isMarket := strings.Contains(orderType, "market")
	if isTakeProfit {
		if isMarket {
			return types.OrderTypeTakeProfitMarket
		}
		return types.OrderTypeTakeProfit
	}

	if isMarket {
		return types.OrderTypeStopMarket
	}
	return types.OrderTypeStopLimit
}

func toGlobalSide(side string) types.SideType {
	switch side {
	case "B":
		return types.SideTypeBuy
	case "A":
		return types.SideTypeSell
	}
	return types.SideType(side)
}

// wsLevelsToPriceVolumeSlice converts WS order book levels to PriceVolumeSlice.
func wsLevelsToPriceVolumeSlice(levels []WsLevel) types.PriceVolumeSlice {
	out := make(types.PriceVolumeSlice, 0, len(levels))
	for _, l := range levels {
		out = append(out, types.PriceVolume{
			Price:  l.Px,
			Volume: l.Sz,
		})
	}
	return out
}

// toGlobalOrderBook converts WS l2Book to bbgo SliceOrderBook.
// levels[0] = bids, levels[1] = asks per Hyperliquid doc.
func toGlobalOrderBook(book WsBook, isFutures bool) *types.SliceOrderBook {
	bids := wsLevelsToPriceVolumeSlice(book.Levels[0])
	asks := wsLevelsToPriceVolumeSlice(book.Levels[1])
	var t time.Time
	if !book.Time.Time().IsZero() {
		t = book.Time.Time()
	}

	symbol := toGlobalSymbol(book.Coin, isFutures)
	return &types.SliceOrderBook{
		Symbol: symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   t,
	}
}

// toGlobalMarketTrade converts WS trade to bbgo Trade (market trade).
func toGlobalMarketTrade(w WsTrade, isFutures bool) types.Trade {
	symbol := toGlobalSymbol(w.Coin, isFutures)

	price := w.Px
	sz := w.Sz
	side := toGlobalSide(w.Side)
	return types.Trade{
		ID:            uint64(w.Tid),
		OrderID:       0,
		Exchange:      types.ExchangeHyperliquid,
		Price:         price,
		Quantity:      sz,
		QuoteQuantity: price.Mul(sz),
		Symbol:        symbol,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       false, // not provided by WS
		Time:          types.Time(w.Time.Time()),
		Fee:           fixedpoint.Zero,
		FeeCurrency:   QuoteCurrency,
		IsFutures:     isFutures,
	}
}

// toGlobalKLine converts WS candle to bbgo KLine.
func toGlobalKLine(c WsCandle, isFutures bool) types.KLine {
	symbol := toGlobalSymbol(c.Symbol, isFutures)
	interval := intervalFromCandleInterval(c.Interval)

	return types.KLine{
		Exchange:                 types.ExchangeHyperliquid,
		Symbol:                   symbol,
		StartTime:                types.Time(c.OpenTime.Time()),
		EndTime:                  types.Time(c.CloseTime.Time()),
		Interval:                 interval,
		Open:                     c.O,
		Close:                    c.C,
		High:                     c.H,
		Low:                      c.L,
		Volume:                   c.V,
		NumberOfTrades:           uint64(c.N),
		QuoteVolume:              fixedpoint.Zero,
		TakerBuyBaseAssetVolume:  fixedpoint.Zero,
		TakerBuyQuoteAssetVolume: fixedpoint.Zero,
		Closed:                   false,
	}
}

// toGlobalPrivateTrade converts WS user fill to bbgo Trade (private trade update).
func toGlobalPrivateTrade(f WsFill, isFutures bool) types.Trade {
	symbol := toGlobalSymbol(f.Coin, isFutures)
	side := toGlobalSide(f.Side)
	return types.Trade{
		ID:            uint64(f.Tid),
		OrderID:       uint64(f.Oid),
		Exchange:      types.ExchangeHyperliquid,
		Price:         f.Px,
		Quantity:      f.Sz,
		QuoteQuantity: f.Px.Mul(f.Sz),
		Symbol:        symbol,
		Side:          side,
		IsBuyer:       side == types.SideTypeBuy,
		IsMaker:       !f.Crossed,
		Time:          types.Time(f.Time.Time()),
		Fee:           fixedpoint.MustNewFromString(f.Fee),
		FeeCurrency:   f.FeeToken,
		IsFutures:     isFutures,
	}
}

func toGlobalOrderStatus(status string) types.OrderStatus {
	switch status {
	case "open", "openOrder":
		return types.OrderStatusNew
	case "filled", "filledOrder":
		return types.OrderStatusFilled
	case "canceled", "cancelled", "canceledOrder", "cancelledOrder":
		return types.OrderStatusCanceled
	case "rejected":
		return types.OrderStatusRejected
	case "partiallyFilled":
		return types.OrderStatusPartiallyFilled
	case "expired":
		return types.OrderStatusExpired
	default:
		return types.OrderStatus(status)
	}
}

// toGlobalOrderUpdate converts WS order update to bbgo Order.
func toGlobalOrderUpdate(o WsOrderUpdate, isFutures bool) types.Order {
	symbol := toGlobalSymbol(o.Order.Coin, isFutures)

	ob := o.Order
	status := toGlobalOrderStatus(o.Status)
	qty := ob.Sz
	price := ob.LimitPx
	origSz := ob.OrigSz
	return types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:      symbol,
			Price:       price,
			Quantity:    origSz,
			Side:        toGlobalSide(ob.Side),
			Type:        types.OrderTypeLimit,
			TimeInForce: types.TimeInForceGTC,
		},
		Exchange:         types.ExchangeHyperliquid,
		OrderID:          uint64(ob.Oid),
		Status:           status,
		ExecutedQuantity: origSz.Sub(qty), // origSz - remaining = executed
		IsWorking:        status == types.OrderStatusNew || status == types.OrderStatusPartiallyFilled,
		CreationTime:     types.Time(ob.Timestamp.Time()),
		UpdateTime:       types.Time(o.StatusTimestamp.Time()),
		IsFutures:        isFutures,
		OriginalStatus:   o.Status,
	}
}

// toGlobalFuturesPositions converts WS clearinghouse state to bbgo FuturesPositionMap.
func toGlobalFuturesPositions(state WsClearinghouseState) types.FuturesPositionMap {
	positions := make(types.FuturesPositionMap)
	for _, asset := range state.AssetPositions {
		p := asset.Position

		// Skip zero positions
		szi := p.Szi
		if szi.IsZero() {
			continue
		}

		positionSide := types.PositionLong
		if szi.Sign() < 0 {
			positionSide = types.PositionShort
		}

		entryPx := p.EntryPx
		positionValue := p.PositionValue
		marginUsed := p.MarginUsed
		unrealizedPnl := p.UnrealizedPnl

		// Calculate mark price from entry price and unrealized PnL
		markPrice := entryPx
		if !szi.IsZero() {
			markPrice = entryPx.Add(unrealizedPnl.Div(szi))
		}

		// Convert liquidation price
		var liquidationPrice fixedpoint.Value
		if p.LiquidationPx != nil {
			liquidationPrice = *p.LiquidationPx
		}

		symbol := toGlobalSymbol(p.Coin, true)
		posKey := types.NewPositionKey(symbol, positionSide)
		positions[posKey] = types.FuturesPosition{
			Symbol:        symbol,
			PositionSide:  positionSide,
			BaseCurrency:  p.Coin,
			QuoteCurrency: QuoteCurrency,
			Base:          szi.Abs(),
			Quote:         positionValue,
			Isolated:      p.Leverage.Type == "isolated",
			PositionRisk: &types.PositionRisk{
				Leverage:              p.Leverage.Value,
				Symbol:                symbol,
				EntryPrice:            entryPx,
				LiquidationPrice:      liquidationPrice,
				PositionAmount:        szi,
				UnrealizedPnL:         unrealizedPnl,
				InitialMargin:         marginUsed,
				MarkPrice:             markPrice,
				PositionInitialMargin: marginUsed,
				Notional:              positionValue,
				PositionSide:          positionSide,
			},
		}
	}
	return positions
}

func toGlobalTicker(markPx, midPx, prevDayPx, dayNtlVlm string) types.Ticker {
	mark := fixedpoint.MustNewFromString(markPx)
	mid := fixedpoint.MustNewFromString(midPx)
	prev := fixedpoint.MustNewFromString(prevDayPx)
	dayNotional := fixedpoint.MustNewFromString(dayNtlVlm)

	last := mark
	if last.IsZero() {
		last = mid
	}

	bid := mid
	ask := mid
	if mid.IsZero() {
		bid = mark
		ask = mark
	}

	return types.Ticker{
		Time:   time.Now(),
		Volume: dayNotional, // Hyperliquid provides day notional volume, not base volume.
		Last:   last,
		Open:   prev,
		High:   fixedpoint.Zero, // Not provided by Hyperliquid asset context.
		Low:    fixedpoint.Zero, // Not provided by Hyperliquid asset context.
		Buy:    bid,             // Best bid is not provided; use mid/mark as approximation.
		Sell:   ask,             // Best ask is not provided; use mid/mark as approximation.
	}
}
