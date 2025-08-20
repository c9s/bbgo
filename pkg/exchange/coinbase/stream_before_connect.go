package coinbase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/c9s/bbgo/pkg/core/klinedriver"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *Stream) beforeConnect(ctx context.Context) error {
	// user data stream, subscribe to user channel for the user order/trade updates
	var err error
	if !s.PublicOnly {
		err = s.buildMapForUserDataStream(ctx)
	} else {
		err = s.buildMapForMarketStream()
	}
	if err != nil {
		return err
	}
	// check channel authorization
	for channel := range s.channel2LocalSymbolsMap {
		switch channel {
		case userChannel, level2Channel, balanceChannel:
			if !s.authEnabled {
				return fmt.Errorf("channel %s requires authentication", channel)
			}
		case fullChannel:
			if !s.authEnabled {
				return errors.New("full channel requires authentication")
			}
			if !s.PublicOnly {
				return errors.New("cannot subscribe to full channel on a private stream")
			}
		}
	}
	return nil
}

func (s *Stream) buildMapForUserDataStream(ctx context.Context) error {
	if !s.authEnabled {
		return errors.New("user data stream requires authentication")
	}
	privateChannelLocalSymbols := s.privateChannelLocalSymbols()
	if len(privateChannelLocalSymbols) == 0 {
		return errors.New("user data stream requires at least one private symbol")
	}
	// subscribe private symbols to user channel
	// Once subscribe to the user channel, it will receive events for the following types:
	// - order life cycle events: receive, open, done, change, activate(for stop orders)
	// - order match
	s.channel2LocalSymbolsMap[userChannel] = privateChannelLocalSymbols

	// query account id for the symbols
	balanceAccountIDs, err := s.exchange.queryAccountIDsBySymbols(ctx, s.privateChannelSymbols)
	if err != nil {
		return err
	}
	s.channel2LocalSymbolsMap[balanceChannel] = balanceAccountIDs
	return nil
}

func (s *Stream) buildMapForMarketStream() error {
	// market data stream: subscribe to channels
	if len(s.Subscriptions) == 0 {
		return errors.New("market stream requires at least one subscription")
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
			s.logger.Infof("bridge %s to level2_batch channel", channel)
			s.channel2LocalSymbolsMap[level2BatchChannel] = keys(dedupLocalSymbols)
		case types.MarketTradeChannel:
			// matches: all trades
			s.channel2LocalSymbolsMap[matchesChannel] = keys(dedupLocalSymbols)
			s.logger.Infof("bridge %s to %s", channel, matchesChannel)
		case types.BookTickerChannel:
			// ticker channel provides feeds on best bid/ask prices
			s.channel2LocalSymbolsMap[tickerChannel] = keys(dedupLocalSymbols)
			s.logger.Infof("bridge %s to %s", channel, tickerChannel)
		case types.KLineChannel:
			// kline stream is available on Advanced Trade API only: https://docs.cdp.coinbase.com/advanced-trade/docs/ws-channels#candles-channel
			// We implement the subscription to kline channel by market trade feeds for now
			// symbols for kline channel will be added to match later with deduplication so we do nothing here
		case types.AggTradeChannel, types.ForceOrderChannel, types.MarkPriceChannel, types.LiquidationOrderChannel, types.ContractInfoChannel:
			s.logger.Warnf("coinbase stream does not support subscription to %s, skipped", channel)
		default:
			// rfqMatchChannel allow empty symbol
			for _, localSymbol := range keys(dedupLocalSymbols) {
				if channel != rfqMatchChannel && channel != statusChannel && len(localSymbol) == 0 {
					s.logger.Warnf("do not support subscription to %s without symbol, skipped", channel)
					continue
				}
				s.channel2LocalSymbolsMap[channel] = append(s.channel2LocalSymbolsMap[channel], localSymbol)
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
				s.channel2LocalSymbolsMap[matchesChannel] = append(s.channel2LocalSymbolsMap[matchesChannel], toLocalSymbol(symbol))
			}
		}

		// binding serial market store
		for symbol, options := range klineOptionsMap {
			klineDriver := klinedriver.NewTickKLineDriver(
				symbol, time.Second*5,
			)
			for _, option := range options {
				s.logger.Debugf("subscribe to kline %s(%s)", symbol, option.Interval)
				err := klineDriver.AddInterval(option.Interval)
				if err != nil {
					s.logger.WithError(err).Warnf("failed to subscribe to kline %s(%s)", symbol, option.Interval)
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
	return nil
}
