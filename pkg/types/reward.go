package types

import (
	"time"

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
	Exchange ExchangeName     `json:"exchange" db:"exchange"`
	Type     RewardType       `json:"reward_type" db:"reward_type"`
	Currency string           `json:"currency" db:"currency"`
	Quantity fixedpoint.Value `json:"quantity" db:"quantity"`
	State    string           `json:"state" db:"state"`
	Note     string           `json:"note" db:"note"`
	Used     bool             `json:"spent" db:"spent"`

	// Unix timestamp in seconds
	CreatedAt datatype.Time `json:"created_at" db:"created_at"`
}

type RewardSlice []Reward

func (s RewardSlice) Len() int      { return len(s) }
func (s RewardSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

type RewardSliceByCreationTime struct {
	RewardSlice
}

// Less reports whether x[i] should be ordered before x[j]
func (s RewardSliceByCreationTime) Less(i, j int) bool {
	return time.Time(s.RewardSlice[i].CreatedAt).After(
		time.Time(s.RewardSlice[j].CreatedAt),
	)
}
