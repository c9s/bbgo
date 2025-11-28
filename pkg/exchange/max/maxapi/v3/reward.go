package v3

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type RewardType string

const (
	RewardAirdrop    = RewardType("airdrop_reward")
	RewardCommission = RewardType("commission")
	RewardHolding    = RewardType("holding_reward")
	RewardMining     = RewardType("mining_reward")
	RewardTrading    = RewardType("trading_reward")
	RewardStaking    = RewardType("staking_reward")
	RewardRedemption = RewardType("redemption_reward")
	RewardVipRebate  = RewardType("vip_rebate")
	RewardYield      = RewardType("yield")
)

type Reward struct {
	// UUID here is more like SN, not the real UUID
	UUID string `json:"uuid"`

	Type RewardType `json:"type"`

	Currency string `json:"currency"`

	Amount fixedpoint.Value `json:"amount"`

	Note string `json:"note"`

	// Unix timestamp in seconds
	CreatedAt types.MillisecondTimestamp `json:"created_at"`
}
