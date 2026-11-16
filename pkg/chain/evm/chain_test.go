package evm

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestLookupChain(t *testing.T) {
	cases := map[string]ChainID{
		"ethereum": ChainIDEthereum,
		"eth":      ChainIDEthereum,
		"Ethereum": ChainIDEthereum,
		" bsc ":    ChainIDBSC,
		"bnb":      ChainIDBSC,
		"polygon":  ChainIDPolygon,
		"matic":    ChainIDPolygon,
	}

	for name, expected := range cases {
		chain, ok := LookupChain(name)
		if assert.Truef(t, ok, "chain %q should be found", name) {
			assert.Equal(t, expected, chain.ID)
		}
	}

	_, ok := LookupChain("arbitrum")
	assert.False(t, ok)
}

func TestLookupChainByID(t *testing.T) {
	chain, ok := LookupChainByID(ChainIDBSC)
	assert.True(t, ok)
	assert.Equal(t, "bsc", chain.Name)
	assert.Equal(t, "BNB", chain.NativeSymbol)
	assert.Equal(t, FeeMarketLegacy, chain.FeeMarket)

	_, ok = LookupChainByID(ChainID(42161))
	assert.False(t, ok)
}

func TestChainDefaults(t *testing.T) {
	for _, name := range SupportedChains() {
		chain, ok := LookupChain(name)
		if !assert.True(t, ok) {
			continue
		}

		assert.Equal(t, 18, chain.NativeDecimals, "%s native decimals", name)
		assert.NotZero(t, chain.DefaultConfirmations, "%s confirmations", name)
		assert.NotZero(t, chain.MaxLogRange, "%s max log range", name)
		assert.NotNil(t, chain.MinPriorityFee, "%s min priority fee", name)
		assert.Equal(t, common.HexToAddress(Multicall3Address), chain.Multicall3, "%s multicall3", name)
		assert.Contains(t, chain.TxURL("0xdeadbeef"), "0xdeadbeef")
	}
}

func TestMulticall3AddressIsChecksummed(t *testing.T) {
	assert.Equal(t, Multicall3Address, common.HexToAddress(Multicall3Address).Hex())
}

func TestChainIDString(t *testing.T) {
	assert.Equal(t, "ethereum", ChainIDEthereum.String())
	assert.Equal(t, "chain-42161", ChainID(42161).String())
	assert.Equal(t, int64(56), ChainIDBSC.Big().Int64())
}
