package bybitapi

import (
	"github.com/c9s/requestgen"
)

type PositionLevelResponse struct {
}

//go:generate requestgen -method POST -url "/v5/position/set-leverage" -type SetPositionLevelRequest -responseType .PositionLevelResponse
type SetPositionLevelRequest struct {
	client requestgen.AuthenticatedAPIClient

	category     Category `param:"category" validValues:"inverse,linear"`
	symbol       string   `param:"symbol,required"`
	buyLeverage  string   `param:"buyLeverage,required"`
	sellLeverage string   `param:"sellLeverage,required"`
}

func (c *RestClient) NewSetPositionLevelRequest() *SetPositionLevelRequest {
	return &SetPositionLevelRequest{
		client:   c,
		category: CategoryLinear,
	}
}
