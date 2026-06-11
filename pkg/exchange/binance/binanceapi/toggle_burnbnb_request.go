package binanceapi

import "github.com/c9s/requestgen"

type BurnBnbResponse struct {
	SpotBNBBurn     bool `json:"spotBNBBurn"`
	InterestBNBBurn bool `json:"interestBNBBurn"`
}

//go:generate requestgen -method POST -url "/sapi/v1/bnbBurn" -type ToggleBurnBnbRequest -responseType BurnBnbResponse
type ToggleBurnBnbRequest struct {
	client requestgen.AuthenticatedAPIClient

	spotBnbBurn     *bool `param:"spotBNBBurn"`
	interestBnbBurn *bool `param:"interestBNBBurn"`
}

func (c *RestClient) NewToggleBurnBnbRequest() *ToggleBurnBnbRequest {
	return &ToggleBurnBnbRequest{client: c}
}
