package coinbase

import "github.com/c9s/bbgo/pkg/types"

func (s *Stream) Subscribe(channel types.Channel, symbol string, options types.SubscribeOptions) {
	localSymbol := toLocalSymbol(symbol)
	switch channel {
	case types.BookChannel:
		logStream.Infof("bridge %s to level2 channel (%s)", channel, symbol)
		s.StandardStream.Subscribe(types.Channel("level2"), localSymbol, options)
	case types.BookTickerChannel:
		logStream.Infof("bridge %s to ticker channel (%s)", channel, symbol)
		s.StandardStream.Subscribe(types.Channel("ticker"), localSymbol, options)
	case types.MarketTradeChannel:
		if s.PublicOnly {
			logStream.Infof("bridge %s to full channel (%s)", channel, symbol)
			s.StandardStream.Subscribe(types.Channel("full"), localSymbol, options)
		} else {
			logStream.Infof("bridge %s to user channel (%s)", channel, symbol)
			s.StandardStream.Subscribe(types.Channel("user"), localSymbol, options)
		}
	case types.KLineChannel, types.AggTradeChannel, types.ForceOrderChannel, types.MarkPriceChannel, types.LiquidationOrderChannel, types.ContractInfoChannel:
		logStream.Warnf("coinbase stream does not support subscription to %s", channel)
	default:
		logStream.Warnf("do not support subscription to non-standard channel %s (use `.StandardStream.Subscribe` to bypass it)", channel)
	}
}
