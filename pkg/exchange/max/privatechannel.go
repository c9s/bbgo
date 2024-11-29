package max

//go:generate mapgen -type PrivateChannel
type PrivateChannel string

const (
	PrivateChannelOrder           PrivateChannel = "order"
	PrivateChannelOrderUpdate     PrivateChannel = "order_update"
	PrivateChannelTrade           PrivateChannel = "trade"
	PrivateChannelTradeUpdate     PrivateChannel = "trade_update"
	PrivateChannelFastTradeUpdate PrivateChannel = "fast_trade_update"
	PrivateChannelAccount         PrivateChannel = "account"
	PrivateChannelAccountUpdate   PrivateChannel = "account_update"

	// @group Misc
	PrivateChannelAveragePrice   PrivateChannel = "average_price"
	PrivateChannelFavoriteMarket PrivateChannel = "favorite_market"

	// @group Margin
	PrivateChannelMWalletOrder           PrivateChannel = "mwallet_order"
	PrivateChannelMWalletTrade           PrivateChannel = "mwallet_trade"
	PrivateChannelMWalletFastTradeUpdate PrivateChannel = "mwallet_fast_trade_update"
	PrivateChannelMWalletAccount         PrivateChannel = "mwallet_account"
	PrivateChannelMWalletAveragePrice    PrivateChannel = "mwallet_average_price"
	PrivateChannelBorrowing              PrivateChannel = "borrowing"
	PrivateChannelAdRatio                PrivateChannel = "ad_ratio"
	PrivateChannelPoolQuota              PrivateChannel = "borrowing_pool_quota"
)

var defaultMarginPrivateChannels = []PrivateChannel{
	PrivateChannelMWalletOrder,
	PrivateChannelMWalletTrade,
	PrivateChannelMWalletAccount,
	PrivateChannelBorrowing,
	PrivateChannelAdRatio,
	PrivateChannelPoolQuota,
}

var defaultSpotPrivateChannels = []PrivateChannel{
	PrivateChannelOrder,
	PrivateChannelTrade,
	PrivateChannelAccount,
}
