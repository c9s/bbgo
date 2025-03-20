package coinbase

import "github.com/c9s/bbgo/pkg/types"

func (s *Stream) Subscribe(channel types.Channel, symbol string, options types.SubscribeOptions) {
	localSymbol := toLocalSymbol(symbol)
	switch channel {
	case types.BookChannel:
		s.StandardStream.Subscribe(types.Channel("level2"), localSymbol, options)
	case types.BookTickerChannel:
		s.StandardStream.Subscribe(types.Channel("ticker"), localSymbol, options)
	case types.MarketTradeChannel:
		if s.PublicOnly {
			s.StandardStream.Subscribe(types.Channel("full"), localSymbol, options)
		} else {
			s.StandardStream.Subscribe(types.Channel("user"), localSymbol, options)
		}
	case types.KLineChannel, types.AggTradeChannel, types.ForceOrderChannel, types.MarkPriceChannel, types.LiquidationOrderChannel, types.ContractInfoChannel:
		logStream.Warnf("coinbase stream does not support subscription to %s", channel)
	default:
		s.StandardStream.Subscribe(channel, localSymbol, options)
	}
}
