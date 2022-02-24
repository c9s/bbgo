package ftxapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result
//go:generate -command DeleteRequest requestgen -method DELETE -responseType .APIResponse -responseDataField Result

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Coin struct {
	Bep2Asset        *string          `json:"bep2Asset"`
	CanConvert       bool             `json:"canConvert"`
	CanDeposit       bool             `json:"canDeposit"`
	CanWithdraw      bool             `json:"canWithdraw"`
	Collateral       bool             `json:"collateral"`
	CollateralWeight fixedpoint.Value `json:"collateralWeight"`
	CreditTo         *string          `json:"creditTo"`
	Erc20Contract    string           `json:"erc20Contract"`
	Fiat             bool             `json:"fiat"`
	HasTag           bool             `json:"hasTag"`
	Id               string           `json:"id"`
	IsToken          bool             `json:"isToken"`
	Methods          []string         `json:"methods"`
	Name             string           `json:"name"`
	SplMint          string           `json:"splMint"`
	Trc20Contract    string           `json:"trc20Contract"`
	UsdFungible      bool             `json:"usdFungible"`
}

//go:generate GetRequest -url "api/coins" -type GetCoinsRequest -responseDataType []Coin
type GetCoinsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetCoinsRequest() *GetCoinsRequest {
	return &GetCoinsRequest{
		client: c,
	}
}
