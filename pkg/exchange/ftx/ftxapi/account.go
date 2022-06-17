package ftxapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result
//go:generate -command DeleteRequest requestgen -method DELETE -responseType .APIResponse -responseDataField Result

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type Position struct {
	Cost                         fixedpoint.Value `json:"cost"`
	EntryPrice                   fixedpoint.Value `json:"entryPrice"`
	Future                       string           `json:"future"`
	InitialMarginRequirement     fixedpoint.Value `json:"initialMarginRequirement"`
	LongOrderSize                fixedpoint.Value `json:"longOrderSize"`
	MaintenanceMarginRequirement fixedpoint.Value `json:"maintenanceMarginRequirement"`
	NetSize                      fixedpoint.Value `json:"netSize"`
	OpenSize                     fixedpoint.Value `json:"openSize"`
	ShortOrderSize               fixedpoint.Value `json:"shortOrderSize"`
	Side                         string           `json:"side"`
	Size                         fixedpoint.Value `json:"size"`
	RealizedPnl                  fixedpoint.Value `json:"realizedPnl"`
	UnrealizedPnl                fixedpoint.Value `json:"unrealizedPnl"`
}

type Account struct {
	BackstopProvider             bool             `json:"backstopProvider"`
	Collateral                   fixedpoint.Value `json:"collateral"`
	FreeCollateral               fixedpoint.Value `json:"freeCollateral"`
	Leverage                     fixedpoint.Value `json:"leverage"`
	InitialMarginRequirement     fixedpoint.Value `json:"initialMarginRequirement"`
	MaintenanceMarginRequirement fixedpoint.Value `json:"maintenanceMarginRequirement"`
	Liquidating                  bool             `json:"liquidating"`
	MakerFee                     fixedpoint.Value `json:"makerFee"`
	MarginFraction               fixedpoint.Value `json:"marginFraction"`
	OpenMarginFraction           fixedpoint.Value `json:"openMarginFraction"`
	TakerFee                     fixedpoint.Value `json:"takerFee"`
	TotalAccountValue            fixedpoint.Value `json:"totalAccountValue"`
	TotalPositionSize            fixedpoint.Value `json:"totalPositionSize"`
	Username                     string           `json:"username"`
	Positions                    []Position       `json:"positions"`
}

//go:generate GetRequest -url "/api/account" -type GetAccountRequest -responseDataType .Account
type GetAccountRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetAccountRequest() *GetAccountRequest {
	return &GetAccountRequest{
		client: c,
	}
}

//go:generate GetRequest -url "/api/positions" -type GetPositionsRequest -responseDataType []Position
type GetPositionsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetPositionsRequest() *GetPositionsRequest {
	return &GetPositionsRequest{
		client: c,
	}
}

type Balance struct {
	Coin                   string           `json:"coin"`
	Free                   fixedpoint.Value `json:"free"`
	SpotBorrow             fixedpoint.Value `json:"spotBorrow"`
	Total                  fixedpoint.Value `json:"total"`
	UsdValue               fixedpoint.Value `json:"usdValue"`
	AvailableWithoutBorrow fixedpoint.Value `json:"availableWithoutBorrow"`
}

//go:generate GetRequest -url "/api/wallet/balances" -type GetBalancesRequest -responseDataType []Balance
type GetBalancesRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetBalancesRequest() *GetBalancesRequest {
	return &GetBalancesRequest{
		client: c,
	}
}
