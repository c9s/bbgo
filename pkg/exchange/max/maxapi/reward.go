package max

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
	CreatedAt Timestamp `json:"created_at"`
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
	client *RestClient
}

func (s *RewardService) NewRewardsRequest() *RewardsRequest {
	return &RewardsRequest{client: s.client}
}

func (s *RewardService) NewRewardsByTypeRequest(pathType RewardType) *RewardsRequest {
	return &RewardsRequest{client: s.client, pathType: &pathType}
}

type RewardsRequest struct {
	client *RestClient

	pathType *RewardType

	currency *string

	// From Unix-timestamp
	from *int64

	// To Unix-timestamp
	to *int64

	limit *int
}

func (r *RewardsRequest) Currency(currency string) *RewardsRequest {
	r.currency = &currency
	return r
}

func (r *RewardsRequest) From(from int64) *RewardsRequest {
	r.from = &from
	return r
}

func (r *RewardsRequest) Limit(limit int) *RewardsRequest {
	r.limit = &limit
	return r
}

func (r *RewardsRequest) To(to int64) *RewardsRequest {
	r.to = &to
	return r
}

func (r *RewardsRequest) Do(ctx context.Context) (rewards []Reward, err error) {
	payload := map[string]interface{}{}

	if r.currency != nil {
		payload["currency"] = r.currency
	}

	if r.to != nil {
		payload["to"] = r.to
	}

	if r.from != nil {
		payload["from"] = r.from
	}

	if r.limit != nil {
		payload["limit"] = r.limit
	}

	refURL := "v2/rewards"

	if r.pathType != nil {
		refURL += "/" + string(*r.pathType)
	}

	req, err := r.client.newAuthenticatedRequest("GET", refURL, payload)
	if err != nil {
		return rewards, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return rewards, err
	}

	if err := response.DecodeJSON(&rewards); err != nil {
		return rewards, err
	}

	return rewards, err
}
