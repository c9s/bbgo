package v3

import "github.com/c9s/requestgen"

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate GetRequest -url "v3/rewards" -type GetRewardsRequest -responseType []Reward
type GetRewardsRequest struct {
	client requestgen.AuthenticatedAPIClient

	rewardType *RewardType `param:"reward_type"`

	currency *string `param:"currency"`

	// timestamp: timestamp in millisecond.
	// responses records whose created time less than or equal to specified timestamp if order in desc, responses records whose created time is greater than or equal to timestamp if order in asc.
	// latest time as default.
	timestamp *int64 `param:"timestamp"`

	// order default: "desc"
	// Enum: "asc" "desc"
	// order in created time.
	order *string `param:"order"`

	limit *int64 `param:"limit"`
}

func (c *Client) NewGetRewardsRequest() *GetRewardsRequest {
	return &GetRewardsRequest{client: c}
}
