package max

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST

import (
	"encoding/json"
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

func ParseRewardType(s string) (RewardType, error) {
	switch s {
	case "airdrop_reward":
		return RewardAirdrop, nil
	case "commission":
		return RewardCommission, nil
	case "holding_reward":
		return RewardHolding, nil
	case "mining_reward":
		return RewardMining, nil
	case "trading_reward":
		return RewardTrading, nil
	case "vip_rebate":
		return RewardVipRebate, nil
	case "redemption_reward":
		return RewardRedemption, nil

	}

	return RewardType(""), fmt.Errorf("unknown reward type: %s", s)
}

func (t *RewardType) UnmarshalJSON(o []byte) error {
	var s string
	var err = json.Unmarshal(o, &s)
	if err != nil {
		return err
	}

	rt, err := ParseRewardType(s)
	if err != nil {
		return err
	}

	*t = rt
	return nil
}

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

func (s *RewardService) NewGetRewardsRequest() *GetRewardsRequest {
	return &GetRewardsRequest{client: s.client}
}

func (s *RewardService) NewGetRewardsOfTypeRequest(pathType RewardType) *GetRewardsOfTypeRequest {
	return &GetRewardsOfTypeRequest{client: s.client, pathType: &pathType}
}

//go:generate GetRequest -url "v2/rewards/:path_type" -type GetRewardsOfTypeRequest -responseType []Reward
type GetRewardsOfTypeRequest struct {
	client requestgen.AuthenticatedAPIClient

	pathType *RewardType `param:"path_type,slug"`

	// From Unix-timestamp
	from *int64 `param:"from"`

	// To Unix-timestamp
	to *int64 `param:"to"`

	page   *int64 `param:"page"`
	limit  *int64 `param:"limit"`
	offset *int64 `param:"offset"`
}

//go:generate GetRequest -url "v2/rewards" -type GetRewardsRequest -responseType []Reward
type GetRewardsRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency *string `param:"currency"`

	// From Unix-timestamp
	from *int64 `param:"from"`

	// To Unix-timestamp
	to *int64 `param:"to"`

	page   *int64 `param:"page"`
	limit  *int64 `param:"limit"`
	offset *int64 `param:"offset"`
}
