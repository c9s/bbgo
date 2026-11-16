package token

import (
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// The built-in symbol-to-address book. It is a convenience for the CLI and for
// configuration, not a source of truth:
//
//   - decimals are never taken from here, they are always read from the chain
//   - a chain session can override or extend any symbol via its `tokens:` config
//
// Every entry is stored in EIP-55 checksum form and TestKnownAddressChecksum
// enforces that, which catches transcription typos. Verify a new entry against
// the issuer's official documentation before adding it.
var knownTokens = map[uint64]map[string]string{
	// Ethereum
	1: {
		"WETH":   "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
		"USDT":   "0xdAC17F958D2ee523a2206206994597C13D831ec7",
		"USDC":   "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		"DAI":    "0x6B175474E89094C44Da98b954EedeAC495271d0F",
		"WBTC":   "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599",
		"CBBTC":  "0xcbB7C0000aB88B473b1f5aFd9ef808440eed33Bf",
		"TBTC":   "0x18084fbA666a33d37592fA2633fD49a74DD93a88",
		"STETH":  "0xae7ab96520DE3A18E5e111B5EaAb095312D7fE84",
		"WSTETH": "0x7f39C581F595B53c5cb19bD0b3f8dA6c935E2Ca0",
		"AAVE":   "0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9",
		"CRV":    "0xD533a949740bb3306d119CC777fa900bA034cd52",
		"BAL":    "0xba100000625a3754423978a60c9317c58a424e3D",
		"COMP":   "0xc00e94Cb662C3520282E6f5717214004A7f26888",
		"LDO":    "0x5A98FcBEA516Cf06857215779Fd812CA3beF1B32",
	},

	// BSC. Note that USDT and USDC are 18 decimals here, unlike on Ethereum.
	56: {
		"WBNB": "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c",
		"BTCB": "0x7130d2A12B9BCbFAe4f2634d864A1Ee1Ce3Ead9c",
		"USDT": "0x55d398326f99059fF775485246999027B3197955",
		"USDC": "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d",
		"ETH":  "0x2170Ed0880ac9A755fd29B2688956BD959F933F8",
		"BUSD": "0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56",
		"CAKE": "0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82",
	},

	// Polygon PoS
	137: {
		"WPOL":   "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270",
		"WMATIC": "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270",
		"USDT":   "0xc2132D05D31c914a87C6611C10748AEb04B58e8F",
		"USDC":   "0x3c499c542cEF5E3811e1192ce70d8cC03d5c3359",
		"USDC.E": "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174",
		"DAI":    "0x8f3Cf7ad23Cd3CaDbD9735AFf958023239c6A063",
		"WBTC":   "0x1BFD67037B42Cf73acF2047067bd4F2C47D9BfD6",
		"WETH":   "0x7ceB23fD6bC0adD59E62ac25578270cFf1b9f619",
	},
}

// Known returns the built-in symbol-to-address book of a chain.
func Known(chainID uint64) map[string]common.Address {
	entries, ok := knownTokens[chainID]
	if !ok {
		return nil
	}

	out := make(map[string]common.Address, len(entries))
	for symbol, address := range entries {
		out[strings.ToUpper(symbol)] = common.HexToAddress(address)
	}

	return out
}
