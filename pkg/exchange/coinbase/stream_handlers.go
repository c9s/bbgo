package coinbase

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/core/klinedriver"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const (
	rfqMatchChannel    types.Channel = "rfq_matches"
	statusChannel      types.Channel = "status"
	auctionChannel     types.Channel = "auctionfeed"
	matchesChannel     types.Channel = "matches"
	tickerChannel      types.Channel = "ticker"
	tickerBatchChannel types.Channel = "ticker_batch"
	level2Channel      types.Channel = "level2"
	level2BatchChannel types.Channel = "level2_batch"
	fullChannel        types.Channel = "full"
	userChannel        types.Channel = "user"
	balanceChannel     types.Channel = "balance"
)

type channelType struct {
	Name       types.Channel `json:"name"`
	ProductIDs []string      `json:"product_ids,omitempty"`
}

func (c channelType) String() string {
	return fmt.Sprintf("(channelType Name:%s, ProductIDs: %s)", c.Name, c.ProductIDs)
}

type authMsg struct {
	Signature  string `json:"signature,omitempty"`
	Key        string `json:"key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
}

func (msg authMsg) String() string {
	return fmt.Sprintf("(authMsg Signature:%s, Timestamp:%s)", msg.Signature, msg.Timestamp)
}

type subscribeMsgType1 struct {
	Type     string        `json:"type"`
	Channels []channelType `json:"channels"`

	authMsg
}

func (msg subscribeMsgType1) String() string {
	return fmt.Sprintf("(subscribeMsg Type:%s, Channels: %s, %s)", msg.Type, msg.Channels, msg.authMsg.String())
}

type subscribeMsgType2 struct {
	Type       string          `json:"type"`
	Channels   []types.Channel `json:"channels"`
	ProductIDs []string        `json:"product_ids,omitempty"`
	AccountIDs []string        `json:"account_ids,omitempty"` // for balance channel

	authMsg
}

func (msg subscribeMsgType2) String() string {
	return fmt.Sprintf(
		"(Type:%s, Channels:%s, ProductIDs:%s, AccountIDs:%s, %s",
		msg.Type,
		msg.Channels,
		msg.ProductIDs,
		msg.AccountIDs,
		msg.authMsg.String(),
	)
}

func (s *Stream) handleConnect() {
	// context for kline workers
	// klineCtx not set -> first time connecting, create a new context
	if s.klineCancel == nil {
		s.klineCtx, s.klineCancel = context.WithCancel(context.Background())
	}
	// channel2LocalSymbolsMap is a map from channel to local symbols
	channel2LocalSymbolsMap := make(map[types.Channel][]string)

	// user data stream, subscribe to user channel for the user order/trade updates
	if !s.PublicOnly {
		s.buildMapForUserDataStream(channel2LocalSymbolsMap)
	} else {
		s.buildMapForMarketStream(channel2LocalSymbolsMap)
	}
	s.writeSubscribeJson(channel2LocalSymbolsMap)

	s.clearSequenceNumber()
	s.clearWorkingOrders()
	go func() {
		if s.PublicOnly {
			// start kline workers
			for _, driver := range s.klineDrivers {
				driver.Run(s.klineCtx)
			}
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		// emit auth to notify the connection is established
		s.EmitAuth()

		// emit balance snapshot on connection
		// query account balances for user stream
		balances, err := s.exchange.QueryAccountBalances(ctx)
		if err != nil {
			logStream.WithError(err).Warn("failed to query account balances, the balance snapshot is initialized with empty balances")
			balances = make(types.BalanceMap)
		}
		s.EmitBalanceSnapshot(balances)

		// query working orders and cache them
		// the cache is required since we do note receive the original order
		// size in the order update messages. Hence we need the cache to find
		// the original order size to calculate the executed quantity.
		// empty symbol -> all symbols
		// empty status array -> all orders that are open or un-settled
		workingRawOrders, err := s.exchange.queryOrdersByPagination(ctx, "", nil, nil, []string{})
		if err != nil {
			logStream.WithError(err).Warn("failed to query open orders, the orders snapshot is initialized with empty orders")
		} else {
			openOrders := make([]types.Order, 0, len(workingRawOrders))
			for _, rawOrder := range workingRawOrders {
				openOrders = append(openOrders, toGlobalOrder(&rawOrder))
			}
			s.updateWorkingOrders(openOrders...)
		}

	}()
}

func (s *Stream) buildMapForUserDataStream(channel2LocalSymbolsMap map[types.Channel][]string) {
	if !s.authEnabled {
		panic("user channel requires authentication")
	}
	privateChannelLocalSymbols := s.privateChannelLocalSymbols()
	if len(privateChannelLocalSymbols) == 0 {
		panic("user channel requires at least one private symbol")
	}
	// subscribe private symbols to user channel
	// Once subscribe to the user channel, it will receive events for the following types:
	// - order life cycle events: receive, open, done, change, activate(for stop orders)
	// - order match
	channel2LocalSymbolsMap[userChannel] = privateChannelLocalSymbols
}

func (s *Stream) buildMapForMarketStream(channel2LocalSymbolsMap map[types.Channel][]string) {
	// market data stream: subscribe to channels
	if len(s.Subscriptions) == 0 {
		return
	}
	// bridge bbgo channels to coinbase channels
	// auth required: level2, full, user
	dedupLocalSymbols := make(map[types.Channel]map[string]struct{})
	// klineOptionsMap: map from **global symbol** to kline options
	// this map will be used to create kline workers which build klines by market trades
	klineOptionsMap := make(map[string][]types.SubscribeOptions)

	for _, sub := range s.Subscriptions {
		if _, ok := dedupLocalSymbols[sub.Channel]; !ok {
			dedupLocalSymbols[sub.Channel] = make(map[string]struct{})
		}
		localSymbol := toLocalSymbol(sub.Symbol)
		dedupLocalSymbols[sub.Channel][localSymbol] = struct{}{}
		if sub.Channel == types.KLineChannel {
			klineOptionsMap[sub.Symbol] = append(klineOptionsMap[sub.Symbol], sub.Options)
		}
	}
	// temp helper function to extract keys from map
	keys := func(mm map[string]struct{}) (localSymbols []string) {
		localSymbols = make([]string, 0, len(mm))
		for product := range mm {
			localSymbols = append(localSymbols, product)
		}
		return
	}
	// populate channel2LocalSymbolsMap
	for channel, dedupLocalSymbols := range dedupLocalSymbols {
		switch channel {
		case types.BookChannel:
			// bridge to level2 channel, which provides order book snapshot and book updates
			logStream.Infof("bridge %s to level2_batch channel", channel)
			channel2LocalSymbolsMap[level2BatchChannel] = keys(dedupLocalSymbols)
		case types.MarketTradeChannel:
			// matches: all trades
			channel2LocalSymbolsMap[matchesChannel] = keys(dedupLocalSymbols)
			logStream.Infof("bridge %s to %s", channel, matchesChannel)
		case types.BookTickerChannel:
			// ticker channel provides feeds on best bid/ask prices
			channel2LocalSymbolsMap[tickerChannel] = keys(dedupLocalSymbols)
			logStream.Infof("bridge %s to %s", channel, tickerChannel)
		case types.KLineChannel:
			// kline stream is available on Advanced Trade API only: https://docs.cdp.coinbase.com/advanced-trade/docs/ws-channels#candles-channel
			// We implement the subscription to kline channel by market trade feeds for now
			// symbols for kline channel will be added to match later with deduplication so we do nothing here
		case types.AggTradeChannel, types.ForceOrderChannel, types.MarkPriceChannel, types.LiquidationOrderChannel, types.ContractInfoChannel:
			logStream.Warnf("coinbase stream does not support subscription to %s, skipped", channel)
		default:
			// rfqMatchChannel allow empty symbol
			for _, localSymbol := range keys(dedupLocalSymbols) {
				if channel != rfqMatchChannel && channel != statusChannel && len(localSymbol) == 0 {
					logStream.Warnf("do not support subscription to %s without symbol, skipped", channel)
					continue
				}
				channel2LocalSymbolsMap[channel] = append(channel2LocalSymbolsMap[channel], localSymbol)
			}
		}
	}

	if len(klineOptionsMap) > 0 {
		// adding symbols to matches channel for market trades
		seenMatchSymbols, ok := dedupLocalSymbols[matchesChannel]
		if !ok {
			seenMatchSymbols = make(map[string]struct{})
			dedupLocalSymbols[matchesChannel] = seenMatchSymbols
		}
		for symbol := range klineOptionsMap {
			if _, ok := seenMatchSymbols[symbol]; !ok {
				channel2LocalSymbolsMap[matchesChannel] = append(channel2LocalSymbolsMap[matchesChannel], toLocalSymbol(symbol))
			}
		}

		// binding serial market store
		for symbol, options := range klineOptionsMap {
			klineDriver := klinedriver.NewTickKLineDriver(
				symbol, time.Second*5,
			)
			for _, option := range options {
				logStream.Debugf("subscribe to kline %s(%s)", symbol, option.Interval)
				err := klineDriver.AddInterval(option.Interval)
				if err != nil {
					logStream.WithError(err).Warnf("failed to subscribe to kline %s(%s)", symbol, option.Interval)
					continue
				}
			}
			s.OnMarketTrade(func(trade types.Trade) {
				klineDriver.AddTrade(trade)
			})
			klineDriver.SetKLineEmitter(s)
			s.klineDrivers = append(s.klineDrivers, klineDriver)
		}
	}
}

func (s *Stream) writeSubscribeJson(channel2LocalSymbolsMap map[types.Channel][]string) {
	var subCmds []any
	signature, ts := s.generateSignature()
	for channel, localSymbols := range channel2LocalSymbolsMap {
		var subType string
		if channel == rfqMatchChannel {
			subType = "subscriptions"
		} else {
			subType = "subscribe"
		}

		var subCmd any
		switch channel {
		case statusChannel:
			subCmd = subscribeMsgType1{
				Type: subType,
				Channels: []channelType{
					{
						Name: channel,
					},
				},
			}
		case auctionChannel, rfqMatchChannel:
			subCmd = subscribeMsgType1{
				Type: subType,
				Channels: []channelType{
					{
						Name:       channel,
						ProductIDs: localSymbols,
					},
				},
			}
		case matchesChannel:
			subCmd = subscribeMsgType1{
				Type: subType,
				Channels: []channelType{
					{
						Name:       channel,
						ProductIDs: localSymbols,
					},
				},
			}
			if v, _ := subCmd.(subscribeMsgType1); s.authEnabled {
				v.authMsg = authMsg{
					Signature:  signature,
					Key:        s.apiKey,
					Passphrase: s.passphrase,
					Timestamp:  ts,
				}
				subCmd = v
			}
		case tickerChannel, tickerBatchChannel:
			subCmd = subscribeMsgType2{
				Type:       subType,
				Channels:   []types.Channel{channel},
				ProductIDs: localSymbols,
			}
		case fullChannel, userChannel:
			if !s.authEnabled {
				panic("full/user channel requires authentication")
			}
			if channel == fullChannel && !s.PublicOnly {
				panic("cannot subscribe to full channel on a private stream")
			}
			subCmd = subscribeMsgType2{
				Type:       subType,
				Channels:   []types.Channel{channel},
				ProductIDs: localSymbols,
				authMsg: authMsg{
					Signature:  signature,
					Key:        s.apiKey,
					Passphrase: s.passphrase,
					Timestamp:  ts,
				},
			}
		case level2Channel:
			if !s.authEnabled {
				panic("level2 channel requires authentication")
			}
			subCmd = subscribeMsgType2{
				Type:       subType,
				Channels:   []types.Channel{channel},
				ProductIDs: localSymbols,

				authMsg: authMsg{
					Signature:  signature,
					Key:        s.apiKey,
					Passphrase: s.passphrase,
					Timestamp:  ts,
				},
			}
		case level2BatchChannel:
			subCmd = subscribeMsgType2{
				Type:       subType,
				Channels:   []types.Channel{channel},
				ProductIDs: localSymbols,
			}
		case balanceChannel:
			if !s.authEnabled {
				panic("balance channel requires authentication")
			}
			subCmd = subscribeMsgType2{
				Type:       subType,
				Channels:   []types.Channel{channel},
				AccountIDs: localSymbols,

				authMsg: authMsg{
					Signature:  signature,
					Key:        s.apiKey,
					Passphrase: s.passphrase,
					Timestamp:  ts,
				},
			}
		default:
			subCmd = subscribeMsgType1{
				Type: subType,
				Channels: []channelType{
					{
						Name:       channel,
						ProductIDs: localSymbols,
					},
				},
			}
			if v, _ := subCmd.(subscribeMsgType1); s.authEnabled {
				v.authMsg = authMsg{
					Signature:  signature,
					Key:        s.apiKey,
					Passphrase: s.passphrase,
					Timestamp:  ts,
				}
				subCmd = v
			}
		}
		subCmds = append(subCmds, subCmd)
	}
	for _, subCmd := range subCmds {
		err := s.Conn.WriteJSON(subCmd)
		if err != nil {
			panic(fmt.Errorf("subscription error for %s: %w", subCmd, err))
		} else {
			logStream.Infof("subscribed to %s", subCmd)
		}
	}
}

func (s *Stream) handleDisconnect() {
	// clear sequence numbers
	s.clearSequenceNumber()
	s.clearWorkingOrders()
	if s.klineCancel != nil {
		s.klineCancel()
		s.klineCtx = nil
		s.klineCancel = nil
	}
}

// Local Handlers: handlers that deal with the messages from the Coinbase WebSocket

// ticker update (real-time price update when there is a match)
// To receive ticker messages, you need to subscribe bbgo BookTickerChannel
func (s *Stream) handleTickerMessage(msg *TickerMessage) {
	// ignore outdated messages
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.ProductID, msg.Sequence) {
		return
	}
	bookTicker := types.BookTicker{
		Symbol:   toGlobalSymbol(msg.ProductID),
		Buy:      msg.BestBid,
		BuySize:  msg.BestBidSize,
		Sell:     msg.BestAsk,
		SellSize: msg.BestAskSize,
	}
	s.EmitBookTickerUpdate(bookTicker)
}

// matches channel (or match message from full/user channel)
// To receive match messages, you need to subscribe bbgo MarketTradeChannel on a public stream
func (s *Stream) handleMatchMessage(msg *MatchMessage) {
	if msg.Type == "last_match" {
		// TODO: fetch missing trades from the REST API and emit them
		return
	}
	// ignore outdated messages
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.ProductID, msg.Sequence) {
		return
	}
	trade := msg.Trade()
	if s.PublicOnly {
		// the stream is a public stream, providing feeds on public market trades, emit market trade
		s.EmitMarketTrade(trade)
	} else {
		// the stream is a user stream, providing feeds on user trades, emit user trade update
		s.EmitTradeUpdate(trade)
	}
}

// level2 handlers: order book snapshot and order book updates
// To receive order book updates, you need to subscribe bbgo BookChannel
// level2 order book snapshot handler
func (s *Stream) handleOrderBookSnapshotMessage(msg *OrderBookSnapshotMessage) {
	symbol := toGlobalSymbol(msg.ProductID)
	var bids types.PriceVolumeSlice
	for _, b := range msg.Bids {
		if len(b) != 2 {
			continue
		}
		bids = append(bids, types.PriceVolume{
			Price:  b[0],
			Volume: b[1],
		})
	}
	var asks types.PriceVolumeSlice
	for _, a := range msg.Asks {
		if len(a) != 2 {
			continue
		}
		asks = append(asks, types.PriceVolume{
			Price:  a[0],
			Volume: a[1],
		})
	}
	book := types.SliceOrderBook{
		Symbol: symbol,
		Bids:   bids,
		Asks:   asks,
		// NOTE: Coinbase does not provide the timestamp for the order book snapshot, should leave it empty
		// See `types.SliceOrderBook.Time` for details
	}
	s.EmitBookSnapshot(book)
}

// level2 order book update handler
func (s *Stream) handleOrderbookUpdateMessage(msg *OrderBookUpdateMessage) {
	var bids types.PriceVolumeSlice
	var asks types.PriceVolumeSlice
	for _, c := range msg.Changes {
		if len(c) != 3 {
			continue
		}
		side := c[0]
		price := fixedpoint.MustNewFromString(c[1])
		volume := fixedpoint.MustNewFromString(c[2])
		if volume.IsZero() {
			// 0 volume means the price can be removed.
			continue
		}
		switch side {
		case "buy":
			bids = append(bids, types.PriceVolume{
				Price:  price,
				Volume: volume,
			})
		case "sell":
			asks = append(asks, types.PriceVolume{
				Price:  price,
				Volume: volume,
			})
		default:
			logStream.Warnf("unknown order book update side: %s", side)
		}
	}
	if len(bids) > 0 || len(asks) > 0 {
		book := types.SliceOrderBook{
			Symbol: toGlobalSymbol(msg.ProductID),
			Bids:   bids,
			Asks:   asks,
			Time:   msg.Time,
		}
		s.EmitBookUpdate(book)
	}
}

// full or user channel: all order updates
// a private stream will automatically subscribe to the user channel
func (s *Stream) handleReceivedMessage(msg *ReceivedMessage) {
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.ProductID, msg.Sequence) {
		return
	}
	if msg.OrderType == "stop" {
		// stop order is not supported
		logStream.Warnf("should not receive stop order via received channel: %s", msg.OrderID)
		return
	}
	// A valid order has been received and is now active.
	orderUpdate := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol: toGlobalSymbol(msg.ProductID),
			Side:   toGlobalSide(msg.Side),
		},
		Status:     types.OrderStatusNew,
		UUID:       msg.OrderID,
		Exchange:   types.ExchangeCoinBase,
		UpdateTime: types.Time(msg.Time),
		IsWorking:  true,
	}
	switch msg.OrderType {
	case "limit":
		orderUpdate.SubmitOrder.Type = types.OrderTypeLimit
		orderUpdate.SubmitOrder.Price = msg.Price
		orderUpdate.SubmitOrder.Quantity = msg.Size
	case "market":
		// NOTE: the Exchange.SubmitOrder method guarantees that the market order does not support funds.
		// So we simply check the size for market order here.
		if msg.Size.IsZero() {
			logStream.Warnf("received empty order size, dropped: %s", msg.OrderID)
			return
		}
		orderUpdate.SubmitOrder.Type = types.OrderTypeMarket
		orderUpdate.SubmitOrder.Quantity = msg.Size
	default:
		logStream.Warnf("unknown order type, dropped: %s", msg.OrderType)
		return
	}
	s.updateWorkingOrders(orderUpdate)
	s.EmitOrderUpdate(orderUpdate)
}

func (s *Stream) handleOpenMessage(msg *OpenMessage) {
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.ProductID, msg.Sequence) {
		return
	}
	// The order is now open on the order book.
	// Getting an open message means that it's not fully filled immediately at receipt of the order or not canceled.
	// We need to consider the case of partially filled orders here.
	lastOrder, found := s.getOrderById(msg.OrderID)
	if !found {
		// the order is not in the working orders map, retrieve it via the API
		order, err := s.retrieveOrderById(msg.OrderID)
		if err != nil {
			logStream.Warnf(
				"the order is not found in the cache and cannot be retrieved via API: %s. Skipped",
				msg.OrderID,
			)
			return
		}
		lastOrder = *order
	}
	amountExecuted := lastOrder.SubmitOrder.Quantity.Sub(msg.RemainingSize)
	orderUpdate := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   toGlobalSymbol(msg.ProductID),
			Side:     toGlobalSide(msg.Side),
			Price:    msg.Price,
			Quantity: lastOrder.Quantity,
		},
		ExecutedQuantity: amountExecuted,
		Status:           types.OrderStatusNew,
		UUID:             msg.OrderID,
		Exchange:         types.ExchangeCoinBase,
		IsWorking:        true,
		UpdateTime:       types.Time(msg.Time),
	}
	s.EmitOrderUpdate(orderUpdate)
}

func (s *Stream) handleDoneMessage(msg *DoneMessage) {
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.ProductID, msg.Sequence) {
		return
	}
	// The lastOrder is no longer on the lastOrder book.
	lastOrder, found := s.getOrderById(msg.OrderID)
	if !found {
		// the order is not in the working orders map, retrieve it via the API
		order, err := s.retrieveOrderById(msg.OrderID)
		if err != nil {
			logStream.Warnf(
				"the order is not found in the cache and cannot be retrieved via API: %s. Skipped",
				msg.OrderID,
			)
			return
		}
		lastOrder = *order
	}
	quantityExecuted := lastOrder.SubmitOrder.Quantity.Sub(msg.RemainingSize)
	status := types.OrderStatusFilled
	if msg.Reason == "canceled" {
		status = types.OrderStatusCanceled
	} else {
		status = types.OrderStatusFilled
	}

	orderUpdate := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   toGlobalSymbol(msg.ProductID),
			Side:     toGlobalSide(msg.Side),
			Price:    msg.Price,
			Quantity: lastOrder.Quantity,
		},
		Status:           status,
		UUID:             msg.OrderID,
		Exchange:         types.ExchangeCoinBase,
		ExecutedQuantity: quantityExecuted,
		IsWorking:        false,
		UpdateTime:       types.Time(msg.Time),
	}
	s.updateWorkingOrders(orderUpdate)
	s.EmitOrderUpdate(orderUpdate)
}

func (s *Stream) handleChangeMessage(msg *ChangeMessage) {
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.ProductID, msg.Sequence) {
		return
	}
	// An order has changed.
	_, found := s.getOrderById(msg.OrderID)
	if !found {
		// We assume the change should only apply to the orders that are cached in the working orders map.
		// The rationale is that
		// 1. If the order is submitted after the connection is established, there should be a received/open message
		//    before the change message. So we should have the order in the working orders map.
		// 2. If the order is submitted before the connection is established, it should be cached in the working orders map
		//    since the map is created on connection.
		// Skip the message if not found since it should be orders that are not belong to the user.
		logStream.Warnf("the order is not found in the cache: %s. Skipped the change", msg.OrderID)
		return
	}
	orderUpdate := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   toGlobalSymbol(msg.ProductID),
			Side:     toGlobalSide(msg.Side),
			Price:    msg.NewPrice,
			Quantity: msg.NewSize,
		},
		UUID:       msg.OrderID,
		Exchange:   types.ExchangeCoinBase,
		IsWorking:  true,
		UpdateTime: types.Time(msg.Time),
	}
	s.updateWorkingOrders(orderUpdate)
	s.EmitOrderUpdate(orderUpdate)
}

func (s *Stream) handleActivateMessage(msg *ActivateMessage) {
	// An activate message is sent when a stop order is placed.
	// the stop order now becomes a limit order.
	lastOrder, found := s.getOrderById(msg.OrderID)
	if !found {
		// the order is not in the working orders map, retrieve it via the API
		order, err := s.retrieveOrderById(msg.OrderID)
		if err != nil {
			logStream.Warnf(
				"the order is not found in the cache and cannot be retrieved via API: %s. Skipped",
				msg.OrderID,
			)
			return
		}
		lastOrder = *order
	}
	var updateTime types.Time
	timestamp, err := strconv.ParseFloat(msg.Timestamp, 64)
	if err != nil {
		updateTime = types.Time(time.Now())
	} else {
		updateTime = types.Time(time.UnixMilli(int64(timestamp * 1000)))
	}
	// do no consider limit order with funds
	order := types.Order{
		SubmitOrder: types.SubmitOrder{
			Type:      types.OrderTypeStopLimit,
			Symbol:    toGlobalSymbol(msg.ProductID),
			Side:      toGlobalSide(msg.Side),
			Price:     lastOrder.Price,
			StopPrice: msg.StopPrice,
			Quantity:  msg.Size,
		},
		Status:     types.OrderStatusNew,
		UUID:       msg.OrderID,
		Exchange:   types.ExchangeCoinBase,
		IsWorking:  true,
		UpdateTime: updateTime,
	}
	s.updateWorkingOrders(order)
	s.EmitOrderUpdate(order)
}

// balance update
func (s *Stream) handleBalanceMessage(msg *BalanceMessage) {
	balanceUpdte := make(types.BalanceMap)
	balanceUpdte[msg.Currency] = types.Balance{
		Currency:  msg.Currency,
		Available: msg.Available,
		Locked:    msg.Holds,
		NetAsset:  msg.Available.Add(msg.Holds),
	}
	s.EmitBalanceUpdate(balanceUpdte)
}

// helpers
func (s *Stream) checkAndUpdateSequenceNumber(msgType MessageType, productId string, currentSeqNum SequenceNumberType) bool {
	s.lockSeqNumMap.Lock()
	defer s.lockSeqNumMap.Unlock()

	key := fmt.Sprintf("%s-%s", msgType, productId)
	latestSeq := s.lastSequenceMsgMap[key]
	if latestSeq >= currentSeqNum {
		return false
	}
	s.lastSequenceMsgMap[key] = currentSeqNum
	return true
}

func (s *Stream) clearSequenceNumber() {
	s.lockSeqNumMap.Lock()
	defer s.lockSeqNumMap.Unlock()

	s.lastSequenceMsgMap = make(map[string]SequenceNumberType)
}

func (s *Stream) updateWorkingOrders(orders ...types.Order) {
	s.lockWorkingOrderMap.Lock()
	defer s.lockWorkingOrderMap.Unlock()

	for _, order := range orders {
		existing, ok := s.workingOrdersMap[order.UUID]
		if ok {
			if !order.IsWorking {
				// order is already in the map and not working, remove it
				delete(s.workingOrdersMap, order.UUID)
				continue
			} else {
				// order is already in the map and working, update it
				existing.Update(order)
				s.workingOrdersMap[order.UUID] = existing
			}
		} else {
			// order is not in the map, add it
			s.workingOrdersMap[order.UUID] = order
		}
	}
}

func (s *Stream) getOrderById(orderId string) (types.Order, bool) {
	s.lockWorkingOrderMap.Lock()
	defer s.lockWorkingOrderMap.Unlock()

	order, found := s.workingOrdersMap[orderId]
	return order, found
}

func (s *Stream) clearWorkingOrders() {
	s.lockWorkingOrderMap.Lock()
	defer s.lockWorkingOrderMap.Unlock()

	s.workingOrdersMap = make(map[string]types.Order)
}

func (s *Stream) retrieveOrderById(uuid string) (*types.Order, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if s.PublicOnly {
		msg := fmt.Sprintf("retrieve order by id disabled on a public stream: %s", uuid)
		logStream.Warn(msg)
		return nil, errors.New(msg)
	}
	order, err := s.exchange.QueryOrder(ctx, types.OrderQuery{OrderUUID: uuid})
	if err != nil {
		return nil, err
	}
	s.updateWorkingOrders(*order)
	return order, nil
}
