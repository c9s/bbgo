package bybitapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type WalletBalancesResponse struct {
	List []WalletBalances `json:"list"`
}

type WalletBalances struct {
	AccountType            AccountType      `json:"accountType"`
	AccountIMRate          fixedpoint.Value `json:"accountIMRate"`
	AccountMMRate          fixedpoint.Value `json:"accountMMRate"`
	TotalEquity            fixedpoint.Value `json:"totalEquity"`
	TotalWalletBalance     fixedpoint.Value `json:"totalWalletBalance"`
	TotalMarginBalance     fixedpoint.Value `json:"totalMarginBalance"`
	TotalAvailableBalance  fixedpoint.Value `json:"totalAvailableBalance"`
	TotalPerpUPL           fixedpoint.Value `json:"totalPerpUPL"`
	TotalInitialMargin     fixedpoint.Value `json:"totalInitialMargin"`
	TotalMaintenanceMargin fixedpoint.Value `json:"totalMaintenanceMargin"`
	// Account LTV: account total borrowed size / (account total equity + account total borrowed size).
	// In non-unified mode & unified (inverse) & unified (isolated_margin), the field will be returned as an empty string.
	AccountLTV fixedpoint.Value `json:"accountLTV"`
	Coins      []struct {
		Coin string `json:"coin"`
		// Equity of current coin
		Equity fixedpoint.Value `json:"equity"`
		// UsdValue of current coin. If this coin cannot be collateral, then it is 0
		UsdValue fixedpoint.Value `json:"usdValue"`
		// WalletBalance of current coin
		WalletBalance fixedpoint.Value `json:"walletBalance"`
		// Free available balance for Spot wallet. This is a unique field for Normal SPOT
		Free fixedpoint.Value
		// Locked balance for Spot wallet. This is a unique field for Normal SPOT
		Locked fixedpoint.Value
		// Available amount to withdraw of current coin
		AvailableToWithdraw fixedpoint.Value `json:"availableToWithdraw"`
		// Available amount to borrow of current coin
		AvailableToBorrow fixedpoint.Value `json:"availableToBorrow"`
		// Borrow amount of current coin
		BorrowAmount fixedpoint.Value `json:"borrowAmount"`
		// Accrued interest
		AccruedInterest fixedpoint.Value `json:"accruedInterest"`
		// Pre-occupied margin for order. For portfolio margin mode, it returns ""
		TotalOrderIM fixedpoint.Value `json:"totalOrderIM"`
		// Sum of initial margin of all positions + Pre-occupied liquidation fee. For portfolio margin mode, it returns ""
		TotalPositionIM fixedpoint.Value `json:"totalPositionIM"`
		// Sum of maintenance margin for all positions. For portfolio margin mode, it returns ""
		TotalPositionMM fixedpoint.Value `json:"totalPositionMM"`
		// Unrealised P&L
		UnrealisedPnl fixedpoint.Value `json:"unrealisedPnl"`
		// Cumulative Realised P&L
		CumRealisedPnl fixedpoint.Value `json:"cumRealisedPnl"`
		// Bonus. This is a unique field for UNIFIED account
		Bonus fixedpoint.Value `json:"bonus"`
		// Whether it can be used as a margin collateral currency (platform)
		// - When marginCollateral=false, then collateralSwitch is meaningless
		// -  This is a unique field for UNIFIED account
		CollateralSwitch bool `json:"collateralSwitch"`
		// Whether the collateral is turned on by user (user)
		// - When marginCollateral=true, then collateralSwitch is meaningful
		// - This is a unique field for UNIFIED account
		MarginCollateral bool `json:"marginCollateral"`
	} `json:"coin"`
}

//go:generate GetRequest -url "/v5/account/wallet-balance" -type GetWalletBalancesRequest -responseDataType .WalletBalancesResponse
type GetWalletBalancesRequest struct {
	client requestgen.AuthenticatedAPIClient

	// Account type
	// - Unified account: UNIFIED (trade spot/linear/options), CONTRACT(trade inverse)
	// - Normal account: CONTRACT, SPOT
	accountType AccountType `param:"accountType,query" validValues:"SPOT"`
	// Coin name
	// - If not passed, it returns non-zero asset info
	// - You can pass multiple coins to query, separated by comma. USDT,USDC
	coin *string `param:"coin,query"`
}

func (c *RestClient) NewGetWalletBalancesRequest() *GetWalletBalancesRequest {
	return &GetWalletBalancesRequest{
		client:      c,
		accountType: AccountTypeSpot,
	}
}
