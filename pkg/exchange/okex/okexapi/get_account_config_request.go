package okexapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

type AccountConfig struct {
	AccountLevel types.StrInt64 `json:"acctLv"`

	AccountSelfTradePreventionMode string `json:"acctStpMode"`

	AutoLoan              bool           `json:"autoLoan"`
	ContractIsolationMode string         `json:"ctIsoMode"`
	EnableSpotBorrow      bool           `json:"enableSpotBorrow"`
	GreeksType            string         `json:"greeksType"`
	Ip                    string         `json:"ip"`
	Type                  string         `json:"type"`
	KycLv                 types.StrInt64 `json:"kycLv"`
	Label                 string         `json:"label"`
	Level                 string         `json:"level"`
	LevelTmp              string         `json:"levelTmp"`

	LiquidationGear types.StrInt64 `json:"liquidationGear"`

	MainUid             string         `json:"mainUid"`
	MarginIsoMode       string         `json:"mgnIsoMode"`
	OpAuth              string         `json:"opAuth"`
	Perm                string         `json:"perm"`
	PosMode             string         `json:"posMode"`
	RoleType            types.StrInt64 `json:"roleType"`
	SpotBorrowAutoRepay bool           `json:"spotBorrowAutoRepay"`
	SpotOffsetType      string         `json:"spotOffsetType"`
	SpotRoleType        string         `json:"spotRoleType"`
	SpotTraderInsts     []interface{}  `json:"spotTraderInsts"`
	TraderInsts         []interface{}  `json:"traderInsts"`
	Uid                 string         `json:"uid"`
}

//go:generate GetRequest -url "/api/v5/account/config" -type GetAccountConfigRequest -responseDataType []AccountConfig
type GetAccountConfigRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetAccountConfigRequest() *GetAccountConfigRequest {
	return &GetAccountConfigRequest{
		client: c,
	}
}
