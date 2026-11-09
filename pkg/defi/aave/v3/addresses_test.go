package v3

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/chain/evm"
)

// TestProviderChecksum catches a transcription typo: an address whose EIP-55
// casing does not round-trip was mistyped.
func TestProviderChecksum(t *testing.T) {
	for chainID, address := range providers {
		assert.Truef(t, common.IsHexAddress(address), "chain %s: %s is not an address", chainID, address)
		assert.Equalf(t, address, common.HexToAddress(address).Hex(),
			"chain %s: the provider address is not in EIP-55 checksum form", chainID)
	}
}

func TestProviderAddress(t *testing.T) {
	for _, chainID := range []evm.ChainID{evm.ChainIDEthereum, evm.ChainIDBSC, evm.ChainIDPolygon} {
		address, err := ProviderAddress(chainID)
		assert.NoError(t, err)
		assert.NotEqual(t, common.Address{}, address)
	}

	_, err := ProviderAddress(evm.ChainID(42161))
	assert.ErrorContains(t, err, "not configured")
}

func TestSupportedChains(t *testing.T) {
	assert.Len(t, SupportedChains(), 3)
}
