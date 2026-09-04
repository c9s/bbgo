package backpackapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET

type Balance struct {
	Available fixedpoint.Value `json:"available"`
	Locked    fixedpoint.Value `json:"locked"`
	Staked    fixedpoint.Value `json:"staked"`
}

func (b Balance) Total() fixedpoint.Value {
	return b.Available.Add(b.Locked).Add(b.Staked)
}

// BalanceMap maps an asset symbol to its balance.
// Note that GET /api/v1/capital returns a JSON object rather than an array.
type BalanceMap map[string]Balance

//go:generate GetRequest -url "/api/v1/capital" -type GetBalancesRequest -responseType .BalanceMap
type GetBalancesRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetBalancesRequest() *GetBalancesRequest {
	return &GetBalancesRequest{client: c.withInstruction(InstructionBalanceQuery)}
}
