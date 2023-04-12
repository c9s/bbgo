package max

import (
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

//go:generate GetRequest -url "/api/v2/markets" -type GetMarketsRequest -responseType []Market
type GetMarketsRequest struct {
	client requestgen.APIClient
}

func (c *RestClient) NewGetMarketsRequest() *GetMarketsRequest {
	return &GetMarketsRequest{client: c}
}
