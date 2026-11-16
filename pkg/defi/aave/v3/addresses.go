package v3

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/c9s/bbgo/pkg/chain/evm"
)

// The PoolAddressesProvider of the main Aave v3 market on each supported chain.
// It is the only hardcoded address: the Pool, the PoolDataProvider and the
// oracle are resolved from it at startup, which is how Aave expects clients to
// discover a market and keeps upgrades from breaking this adapter.
//
// Every entry is stored in EIP-55 checksum form, which TestProviderChecksum
// enforces, and the live integration test verifies that each one resolves to a
// deployed pool.
var providers = map[evm.ChainID]string{
	evm.ChainIDEthereum: "0x2f39d218133AFaB8F2B819B1066c7E434Ad94E9e",
	evm.ChainIDBSC:      "0xff75B6da14FfbbfD355Daf7a2731456b3562Ba6D",
	evm.ChainIDPolygon:  "0xa97684ead0e402dC232d5A977953DF7ECBaB3CDb",
}

// ProviderAddress returns the PoolAddressesProvider of the main market on the
// given chain.
func ProviderAddress(chainID evm.ChainID) (common.Address, error) {
	hex, ok := providers[chainID]
	if !ok {
		return common.Address{}, fmt.Errorf("aave v3 is not configured for chain %s", chainID)
	}

	return common.HexToAddress(hex), nil
}

// SupportedChains lists the chains with a configured Aave v3 market.
func SupportedChains() []evm.ChainID {
	ids := make([]evm.ChainID, 0, len(providers))
	for id := range providers {
		ids = append(ids, id)
	}

	return ids
}
