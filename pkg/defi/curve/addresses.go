package curve

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// AddressProviderAddress is Curve's AddressProvider, the one address this
// package hardcodes. Curve deploys it at the same address on every chain it
// supports, the way Multicall3 is deployed at one address everywhere, so there
// is no per-chain table to go stale.
//
// An older AddressProvider lives at 0x0000000022D53366457F9d5E68Ec105046FC4383.
// It is still the canonical one on Ethereum, where both report the same
// MetaRegistry, but it is not kept up to date elsewhere: on Polygon it points
// at a MetaRegistry that indexes 361 pools where this one indexes 1667, and on
// BSC its MetaRegistry entry is the zero address.
const AddressProviderAddress = "0x5ffe7FB82894076ECB99A30D6A32e969e6e35E98"

// MetaRegistryID is the AddressProvider id of the MetaRegistry.
const MetaRegistryID = 7

// MaxCoins is the width of every coin array the MetaRegistry returns, and
// MaxFees the width of the fee array, regardless of how many values a pool
// actually has.
const (
	MaxCoins = 8
	MaxFees  = 10
)

// FeeDenominator is Curve's fixed point for fees: a fee of 4000000 is 0.04%.
const FeeDenominator = 1e10

// AddressProvider returns the address this package resolves the MetaRegistry
// from.
func AddressProvider() common.Address {
	return common.HexToAddress(AddressProviderAddress)
}

// FeeFraction converts a raw Curve fee into a fraction, so 4000000 becomes
// 0.0004.
func FeeFraction(raw *big.Int) float64 {
	if raw == nil {
		return 0
	}

	value, _ := new(big.Float).Quo(new(big.Float).SetInt(raw), big.NewFloat(FeeDenominator)).Float64()

	return value
}
