package backpackapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET

type Collateral struct {
	Symbol string `json:"symbol"`

	AssetMarkPrice fixedpoint.Value `json:"assetMarkPrice"`
	TotalQuantity  fixedpoint.Value `json:"totalQuantity"`

	BalanceNotional  fixedpoint.Value `json:"balanceNotional"`
	CollateralWeight fixedpoint.Value `json:"collateralWeight"`
	CollateralValue  fixedpoint.Value `json:"collateralValue"`

	OpenOrderQuantity fixedpoint.Value `json:"openOrderQuantity"`
	LendQuantity      fixedpoint.Value `json:"lendQuantity"`
	AvailableQuantity fixedpoint.Value `json:"availableQuantity"`
}

// MarginAccountSummary is the margin/collateral summary of the account.
type MarginAccountSummary struct {
	AssetsValue      fixedpoint.Value `json:"assetsValue"`
	LiabilitiesValue fixedpoint.Value `json:"liabilitiesValue"`
	BorrowLiability  fixedpoint.Value `json:"borrowLiability"`

	Collateral []Collateral `json:"collateral"`

	Imf fixedpoint.Value `json:"imf"`
	Mmf fixedpoint.Value `json:"mmf"`

	MarginFraction fixedpoint.Value `json:"marginFraction,omitempty"`

	NetEquity          fixedpoint.Value `json:"netEquity"`
	NetEquityAvailable fixedpoint.Value `json:"netEquityAvailable"`
	NetEquityLocked    fixedpoint.Value `json:"netEquityLocked"`
	NetExposureFutures fixedpoint.Value `json:"netExposureFutures"`

	UnsettledEquity fixedpoint.Value `json:"unsettledEquity"`
	PnlUnrealized   fixedpoint.Value `json:"pnlUnrealized"`
}

//go:generate GetRequest -url "/api/v1/capital/collateral" -type GetCollateralRequest -responseType .MarginAccountSummary
type GetCollateralRequest struct {
	client requestgen.AuthenticatedAPIClient

	subaccountId *uint16 `param:"subaccountId"`
}

func (c *RestClient) NewGetCollateralRequest() *GetCollateralRequest {
	return &GetCollateralRequest{client: c.withInstruction(InstructionCollateralQuery)}
}
