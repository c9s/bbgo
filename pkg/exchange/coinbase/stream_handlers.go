package coinbase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type channelType struct {
	Name       string   `json:"name"`
	ProductIDs []string `json:"product_ids,omitempty"`
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
	Type       string   `json:"type"`
	Channels   []string `json:"channels"`
	ProductIDs []string `json:"product_ids,omitempty"`
	AccountIDs []string `json:"account_ids,omitempty"` // for balance channel

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
	// subscribe to channels
	if len(s.Subscriptions) == 0 {
		return
	}

	subProductsMap := make(map[string][]string)
	for _, sub := range s.Subscriptions {
		strChannel := string(sub.Channel)
		// rfqMatchChannel allow empty symbol
		if sub.Channel != rfqMatchChannel && len(sub.Symbol) == 0 {
			continue
		}
		subProductsMap[strChannel] = append(subProductsMap[strChannel], sub.Symbol)
	}
	var subCmds []any
	signature, ts := s.generateSignature()
	for channel, productIDs := range subProductsMap {
		var subType string
		switch channel {
		case "rfq_matches":
			subType = "subscriptions"
		default:
			subType = "subscribe"
		}
		var subCmd any
		switch channel {
		case "ticker", "ticker_batch", "level2", "level2_batch":
			subCmd = subscribeMsgType2{
				Type:       subType,
				Channels:   []string{channel},
				ProductIDs: productIDs,
				authMsg: authMsg{
					Signature:  signature,
					Key:        s.apiKey,
					Passphrase: s.passphrase,
					Timestamp:  ts,
				},
			}
		case "balance":
			subCmd = subscribeMsgType2{
				Type:       subType,
				Channels:   []string{channel},
				AccountIDs: productIDs,

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
						ProductIDs: productIDs,
					},
				},

				authMsg: authMsg{
					Signature:  signature,
					Key:        s.apiKey,
					Passphrase: s.passphrase,
					Timestamp:  ts,
				},
			}
		}
		subCmds = append(subCmds, subCmd)
	}
	for _, subCmd := range subCmds {
		err := s.Conn.WriteJSON(subCmd)
		if err != nil {
			log.WithError(err).Errorf("subscription error: %s", subCmd)
		} else {
			log.Infof("subscribed to %s", subCmd)
		}
	}

	// emit balance snapshot on connection
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		// query account balances
		balances, err := s.exchange.QueryAccountBalances(ctx)
		if err != nil {
			log.WithError(err).Warn("failed to query account balances, the balance snapshot is initialized with empty balances")
			balances = make(types.BalanceMap)
		}
		s.EmitBalanceSnapshot(balances)
	}()
}

func (s *Stream) handleDisconnect() {
	// clear sequence numbers
	s.lock.Lock()
	defer s.lock.Unlock()

	s.lastSequenceMsgMap = make(map[MessageType]SequenceNumberType)
}

// order book, trade
func (s *Stream) handleTickerMessage(msg *TickerMessage) {
	// ignore outdated messages
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.Sequence) {
		return
	}
	trade := msg.Trade()
	s.EmitTradeUpdate(trade)
}

func (s *Stream) handleMatchMessage(msg *MatchMessage) {
	// ignore outdated messages
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.Sequence) {
		return
	}
	trade := msg.Trade()
	s.EmitTradeUpdate(trade)
}

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
		Time:   time.Now(),
	}
	s.EmitBookSnapshot(book)
}

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
			log.Warnf("unknown order book update side: %s", side)
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

// order update
func (s *Stream) handleReceivedMessage(msg *ReceivedMessage) {
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.Sequence) {
		return
	}

	orderUpdate := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   toGlobalSymbol(msg.ProductID),
			Side:     toGlobalSide(msg.Side),
			Price:    msg.Price,
			Quantity: msg.Size,
			Type:     types.OrderType(strings.ToUpper(msg.OrderType)),
		},
		Exchange:   types.ExchangeCoinBase,
		UpdateTime: types.Time(msg.Time),
		IsWorking:  true,
	}
	s.EmitOrderUpdate(orderUpdate)
}

func (s *Stream) handleOpenMessage(msg *OpenMessage) {
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.Sequence) {
		return
	}
	orderUpdate := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   toGlobalSymbol(msg.ProductID),
			Side:     toGlobalSide(msg.Side),
			Price:    msg.Price,
			Quantity: msg.RemainingSize,
		},
		UUID:       msg.OrderID,
		Exchange:   types.ExchangeCoinBase,
		IsWorking:  true,
		UpdateTime: types.Time(msg.Time),
	}
	s.EmitOrderUpdate(orderUpdate)
}

func (s *Stream) handleDoneMessage(msg *DoneMessage) {
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.Sequence) {
		return
	}
	isWorking := true
	if msg.Reason == "canceled" {
		isWorking = false
	}
	orderUpdate := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   toGlobalSymbol(msg.ProductID),
			Side:     toGlobalSide(msg.Side),
			Price:    msg.Price,
			Quantity: msg.RemainingSize,
		},
		UUID:       msg.OrderID,
		Exchange:   types.ExchangeCoinBase,
		IsWorking:  isWorking,
		UpdateTime: types.Time(msg.Time),
	}
	s.EmitOrderUpdate(orderUpdate)
}

func (s *Stream) handleChangeMessage(msg *ChangeMessage) {
	if !s.checkAndUpdateSequenceNumber(msg.Type, msg.Sequence) {
		return
	}
	price := msg.Price
	if msg.Reason == "modify_order" {
		price = msg.NewPrice
	}
	orderUpdate := types.Order{
		SubmitOrder: types.SubmitOrder{
			Symbol:   toGlobalSymbol(msg.ProductID),
			Side:     toGlobalSide(msg.Side),
			Price:    price,
			Quantity: msg.NewSize,
		},
		UUID:       msg.OrderID,
		Exchange:   types.ExchangeCoinBase,
		IsWorking:  true,
		UpdateTime: types.Time(msg.Time),
	}
	s.EmitOrderUpdate(orderUpdate)
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
func (s *Stream) checkAndUpdateSequenceNumber(msgType MessageType, currentSeqNum SequenceNumberType) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	latestSeq := s.lastSequenceMsgMap[msgType]
	if latestSeq >= currentSeqNum {
		return false
	}
	s.lastSequenceMsgMap[msgType] = currentSeqNum
	return true
}
