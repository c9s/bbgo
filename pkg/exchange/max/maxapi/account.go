package maxapi

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

// Account is for max rest api v2, Balance and Type will be conflict with types.PrivateBalanceUpdate
type Account struct {
	Type     string           `json:"type"`
	Currency string           `json:"currency"`
	Balance  fixedpoint.Value `json:"balance"`
	Locked   fixedpoint.Value `json:"locked"`

	// v3 fields for M wallet
	Principal fixedpoint.Value `json:"principal"`
	Interest  fixedpoint.Value `json:"interest"`

	// v2 fields
	FiatCurrency string           `json:"fiat_currency"`
	FiatBalance  fixedpoint.Value `json:"fiat_balance"`
}

type UserBank struct {
	Branch  string `json:"branch"`
	Name    string `json:"name"`
	Account string `json:"account"`
	State   string `json:"state"`
}

type UserInfo struct {
	Email          string           `json:"email"`
	Level          int              `json:"level"`
	MWalletEnabled bool             `json:"m_wallet_enabled"`
	Current        VipLevelSettings `json:"current_vip_level"`
	Next           VipLevelSettings `json:"next_vip_level"`
}

type VipLevelSettings struct {
	Level                int     `json:"level"`
	MinimumTradingVolume float64 `json:"minimum_trading_volume"`
	MinimumStakingVolume float64 `json:"minimum_staking_volume"`
	MakerFee             float64 `json:"maker_fee"`
	TakerFee             float64 `json:"taker_fee"`
}

type VipLevel struct {
	Current VipLevelSettings `json:"current_vip_level"`
	Next    VipLevelSettings `json:"next_vip_level"`
}
