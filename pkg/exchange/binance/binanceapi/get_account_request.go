package binanceapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type Balance struct {
	Asset string `json:"asset"`

	Free   fixedpoint.Value `json:"free"`
	Locked fixedpoint.Value `json:"locked"`
}

type SpotAccount struct {
	MakerCommission  int `json:"makerCommission"`
	TakerCommission  int `json:"takerCommission"`
	BuyerCommission  int `json:"buyerCommission"`
	SellerCommission int `json:"sellerCommission"`
	CommissionRates  struct {
		Maker  fixedpoint.Value `json:"maker"`
		Taker  fixedpoint.Value `json:"taker"`
		Buyer  fixedpoint.Value `json:"buyer"`
		Seller fixedpoint.Value `json:"seller"`
	} `json:"commissionRates"`
	CanTrade    bool `json:"canTrade"`
	CanWithdraw bool `json:"canWithdraw"`
	CanDeposit  bool `json:"canDeposit"`

	Brokered                   bool `json:"brokered"`
	RequireSelfTradePrevention bool `json:"requireSelfTradePrevention"`
	PreventSor                 bool `json:"preventSor"`

	UpdateTime types.MillisecondTimestamp `json:"updateTime"`

	AccountType string    `json:"accountType"`
	Balances    []Balance `json:"balances"`

	Permissions []string `json:"permissions"`

	Uid int `json:"uid"`
}

//go:generate requestgen -method GET -url "/api/v3/account" -type GetAccountRequest -responseType .SpotAccount
type GetAccountRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetAccountRequest() *GetAccountRequest {
	return &GetAccountRequest{client: c}
}
