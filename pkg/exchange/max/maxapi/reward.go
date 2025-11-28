package maxapi

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST

import (
	"fmt"
	"strings"

	"github.com/c9s/requestgen"

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
	RewardRedemption = RewardType("redemption_reward")
	RewardVipRebate  = RewardType("vip_rebate")
)

func (t RewardType) RewardType() (types.RewardType, error) {
	switch t {

	case RewardAirdrop:
		return types.RewardAirdrop, nil

	case RewardCommission:
		return types.RewardCommission, nil

	case RewardHolding:
		return types.RewardHolding, nil

	case RewardMining:
		return types.RewardMining, nil

	case RewardTrading:
		return types.RewardTrading, nil

	case RewardVipRebate:
		return types.RewardVipRebate, nil

	}

	return types.RewardType(""), fmt.Errorf("unknown reward type: %s", t)
}

type Reward struct {
	// UUID here is more like SN, not the real UUID
	UUID     string           `json:"uuid"`
	Type     RewardType       `json:"type"`
	Currency string           `json:"currency"`
	Amount   fixedpoint.Value `json:"amount"`
	State    string           `json:"state"`
	Note     string           `json:"note"`

	// Unix timestamp in seconds
	CreatedAt types.Timestamp `json:"created_at"`
}

func (reward Reward) Reward() (*types.Reward, error) {
	rt, err := reward.Type.RewardType()
	if err != nil {
		return nil, err
	}

	return &types.Reward{
		UUID:      reward.UUID,
		Exchange:  types.ExchangeMax,
		Type:      rt,
		Currency:  strings.ToUpper(reward.Currency),
		Quantity:  reward.Amount,
		State:     reward.State,
		Note:      reward.Note,
		Spent:     false,
		CreatedAt: types.Time(reward.CreatedAt),
	}, nil
}

type RewardService struct {
	client requestgen.AuthenticatedAPIClient
}
