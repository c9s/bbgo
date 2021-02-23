package types

import (
	"github.com/c9s/bbgo/pkg/datatype"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type RewardType string

const (
	RewardAirdrop    = RewardType("airdrop")
	RewardCommission = RewardType("commission")
	RewardHolding    = RewardType("holding")
	RewardMining     = RewardType("mining")
	RewardTrading    = RewardType("trading")
	RewardVipRebate  = RewardType("vip_rebate")
)

type Reward struct {
	UUID     string           `json:"uuid" db:"uuid"`
	Type     RewardType       `json:"reward_type" db:"reward_type"`
	Currency string           `json:"currency" db:"currency"`
	Amount   fixedpoint.Value `json:"quantity" db:"quantity"`
	State    string           `json:"state" db:"state"`
	Note     string           `json:"note" db:"note"`
	Used     bool             `json:"used" db:"used"`

	// Unix timestamp in seconds
	CreatedAt datatype.Time `json:"created_at"`
}
