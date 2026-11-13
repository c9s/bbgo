// Package evm provides the low-level EVM chain access layer used by the DeFi
// protocol adapters under pkg/defi.
//
// Design rules for this package and everything built on top of it:
//
//   - Raw on-chain amounts are always *big.Int (wei / token base units). They
//     must never be stored in fixedpoint.Value: the default (non-dnum) build of
//     fixedpoint is an int64 with 8 decimal places, so 1e18 wei overflows it.
//     Conversion to fixedpoint happens only at the strategy/display boundary and
//     always goes through the token decimals (see pkg/chain/evm/token).
//   - go-ethereum is imported by this package only. Protocol adapters depend on
//     *evm.Client and the generated contract bindings, so the underlying RPC
//     client stays replaceable.
package evm

import (
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type ChainID uint64

const (
	ChainIDEthereum ChainID = 1
	ChainIDBSC      ChainID = 56
	ChainIDPolygon  ChainID = 137
)

func (id ChainID) Big() *big.Int {
	return new(big.Int).SetUint64(uint64(id))
}

func (id ChainID) String() string {
	if chain, ok := LookupChainByID(id); ok {
		return chain.Name
	}

	return fmt.Sprintf("chain-%d", uint64(id))
}

// FeeMarket describes how the transaction fee is priced on a chain.
type FeeMarket string

const (
	// FeeMarketEIP1559 uses maxFeePerGas / maxPriorityFeePerGas.
	FeeMarketEIP1559 FeeMarket = "eip1559"

	// FeeMarketLegacy uses a single gasPrice field. BSC nominally supports
	// EIP-1559 but the base fee is effectively zero, so it is priced as legacy.
	FeeMarketLegacy FeeMarket = "legacy"
)

// Multicall3Address is deployed at the same address on Ethereum, BSC and Polygon.
const Multicall3Address = "0xcA11bde05977b3631167028862bE2a173976CA11"

// Chain holds the per-chain facts that the client, the fee strategy and the
// confirmation logic need. These are protocol facts, not user configuration.
type Chain struct {
	ID   ChainID
	Name string

	// NativeSymbol is the gas currency symbol, NativeDecimals is always 18 on
	// these chains but is kept explicit so no code hardcodes it.
	NativeSymbol   string
	NativeDecimals int

	// WrappedNative is the canonical wrapped native token (WETH / WBNB / WPOL).
	WrappedNative string

	BlockTime time.Duration

	FeeMarket FeeMarket

	// MinPriorityFee is the floor for maxPriorityFeePerGas in wei. Polygon
	// rejects transactions below ~25 gwei priority fee.
	MinPriorityFee *big.Int

	// DefaultConfirmations is the number of blocks to wait before a receipt is
	// considered final. Polygon has historically seen deep reorgs, hence the
	// much larger value.
	DefaultConfirmations int

	// SupportsFinalizedTag reports whether eth_getBlockByNumber accepts the
	// "finalized" tag, which is preferred over a confirmation count.
	SupportsFinalizedTag bool

	// MaxLogRange is a conservative per-request block range for eth_getLogs.
	// Public RPC providers cap this and the cap differs per provider.
	MaxLogRange uint64

	// Multicall3 is the Multicall3 contract address on this chain.
	Multicall3 common.Address

	ExplorerTxURL string
}

func gwei(n int64) *big.Int {
	return new(big.Int).Mul(big.NewInt(n), big.NewInt(1e9))
}

var chains = []*Chain{
	{
		ID:                   ChainIDEthereum,
		Name:                 "ethereum",
		NativeSymbol:         "ETH",
		NativeDecimals:       18,
		WrappedNative:        "WETH",
		BlockTime:            12 * time.Second,
		FeeMarket:            FeeMarketEIP1559,
		MinPriorityFee:       gwei(1),
		DefaultConfirmations: 3,
		SupportsFinalizedTag: true,
		MaxLogRange:          2000,
		Multicall3:           common.HexToAddress(Multicall3Address),
		ExplorerTxURL:        "https://etherscan.io/tx/%s",
	},
	{
		ID:                   ChainIDBSC,
		Name:                 "bsc",
		NativeSymbol:         "BNB",
		NativeDecimals:       18,
		WrappedNative:        "WBNB",
		BlockTime:            750 * time.Millisecond,
		FeeMarket:            FeeMarketLegacy,
		MinPriorityFee:       gwei(1),
		DefaultConfirmations: 15,
		SupportsFinalizedTag: true,
		MaxLogRange:          1000,
		Multicall3:           common.HexToAddress(Multicall3Address),
		ExplorerTxURL:        "https://bscscan.com/tx/%s",
	},
	{
		ID:                   ChainIDPolygon,
		Name:                 "polygon",
		NativeSymbol:         "POL",
		NativeDecimals:       18,
		WrappedNative:        "WPOL",
		BlockTime:            2 * time.Second,
		FeeMarket:            FeeMarketEIP1559,
		MinPriorityFee:       gwei(25),
		DefaultConfirmations: 64,
		SupportsFinalizedTag: true,
		MaxLogRange:          1000,
		Multicall3:           common.HexToAddress(Multicall3Address),
		ExplorerTxURL:        "https://polygonscan.com/tx/%s",
	},
}

// chainAliases maps accepted configuration names to the canonical chain name.
var chainAliases = map[string]string{
	"eth":      "ethereum",
	"mainnet":  "ethereum",
	"ethereum": "ethereum",
	"bsc":      "bsc",
	"bnb":      "bsc",
	"bnbchain": "bsc",
	"polygon":  "polygon",
	"matic":    "polygon",
}

// LookupChain returns the chain by its configuration name or alias.
func LookupChain(name string) (*Chain, bool) {
	canonical, ok := chainAliases[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return nil, false
	}

	for _, chain := range chains {
		if chain.Name == canonical {
			return chain, true
		}
	}

	return nil, false
}

func LookupChainByID(id ChainID) (*Chain, bool) {
	for _, chain := range chains {
		if chain.ID == id {
			return chain, true
		}
	}

	return nil, false
}

// SupportedChains returns the canonical names of the supported chains.
func SupportedChains() []string {
	names := make([]string, 0, len(chains))
	for _, chain := range chains {
		names = append(names, chain.Name)
	}

	return names
}

func (c *Chain) TxURL(hash string) string {
	if c.ExplorerTxURL == "" {
		return hash
	}

	return fmt.Sprintf(c.ExplorerTxURL, hash)
}
