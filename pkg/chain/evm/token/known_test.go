package token

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

// TestKnownAddressChecksum catches transcription typos in the address book: an
// address whose EIP-55 checksum casing does not round-trip was mistyped.
func TestKnownAddressChecksum(t *testing.T) {
	for chainID, entries := range knownTokens {
		for symbol, address := range entries {
			assert.Truef(t, common.IsHexAddress(address), "chain %d %s: %s is not an address", chainID, symbol, address)
			assert.Equalf(t, address, common.HexToAddress(address).Hex(),
				"chain %d %s: address is not in EIP-55 checksum form", chainID, symbol)
		}
	}
}

func TestKnown(t *testing.T) {
	ethereum := Known(1)
	assert.Equal(t, common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"), ethereum["WETH"])

	bsc := Known(56)
	assert.Equal(t, common.HexToAddress("0x7130d2A12B9BCbFAe4f2634d864A1Ee1Ce3Ead9c"), bsc["BTCB"])

	assert.Nil(t, Known(42161))
}
