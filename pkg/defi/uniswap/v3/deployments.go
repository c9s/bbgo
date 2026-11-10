package v3

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"github.com/c9s/bbgo/pkg/chain/evm"
)

// Fee tiers, in hundredths of a basis point: 3000 is 0.3%.
//
// Uniswap and PancakeSwap do not use the same ladder: Pancake replaced the
// 0.3% tier with 0.25%, so a hardcoded 3000 finds no pool on Pancake.
const (
	FeeTier100   uint32 = 100   // 0.01%
	FeeTier500   uint32 = 500   // 0.05%
	FeeTier2500  uint32 = 2500  // 0.25%, PancakeSwap only
	FeeTier3000  uint32 = 3000  // 0.3%, Uniswap only
	FeeTier10000 uint32 = 10000 // 1%
)

// Deployment is one v3-style factory and its companion contracts on one chain.
type Deployment struct {
	// Name identifies the deployment in configuration, for example "uniswap".
	Name string

	ChainID evm.ChainID

	Factory    common.Address
	QuoterV2   common.Address
	SwapRouter common.Address

	// FeeTiers is the ladder to probe when searching for a pool.
	FeeTiers []uint32
}

func (d Deployment) String() string {
	return fmt.Sprintf("%s v3 on %s: factory=%s quoter=%s router=%s",
		d.Name, d.ChainID, d.Factory.Hex(), d.QuoterV2.Hex(), d.SwapRouter.Hex())
}

// deployment addresses. Every address is stored in EIP-55 checksum form, which
// TestDeploymentChecksum enforces, and the live integration test verifies that
// each factory, quoter and router actually answers on its chain.
//
// The first deployment of a chain is its default.
var deployments = map[evm.ChainID][]Deployment{
	evm.ChainIDEthereum: {
		{
			Name:       "uniswap",
			Factory:    common.HexToAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984"),
			QuoterV2:   common.HexToAddress("0x61fFE014bA17989E743c5F6cB21bF9697530B21e"),
			SwapRouter: common.HexToAddress("0x68b3465833fb72A70ecDF485E0e4C7bD8665Fc45"),
			FeeTiers:   []uint32{FeeTier100, FeeTier500, FeeTier3000, FeeTier10000},
		},
	},

	evm.ChainIDPolygon: {
		{
			Name:       "uniswap",
			Factory:    common.HexToAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984"),
			QuoterV2:   common.HexToAddress("0x61fFE014bA17989E743c5F6cB21bF9697530B21e"),
			SwapRouter: common.HexToAddress("0x68b3465833fb72A70ecDF485E0e4C7bD8665Fc45"),
			FeeTiers:   []uint32{FeeTier100, FeeTier500, FeeTier3000, FeeTier10000},
		},
	},

	// Uniswap v3 is deployed on BSC, but PancakeSwap v3 holds most of the
	// liquidity there, so both are available and Pancake is the default.
	evm.ChainIDBSC: {
		{
			Name:       "pancakeswap",
			Factory:    common.HexToAddress("0x0BFbCF9fa4f9C56B0F40a671Ad40E0805A091865"),
			QuoterV2:   common.HexToAddress("0xB048Bbc1Ee6b733FFfCFb9e9CeF7375518e25997"),
			SwapRouter: common.HexToAddress("0x1b81D678ffb9C0263b24A97847620C99d213eB14"),
			FeeTiers:   []uint32{FeeTier100, FeeTier500, FeeTier2500, FeeTier10000},
		},
		{
			Name:       "uniswap",
			Factory:    common.HexToAddress("0xdB1d10011AD0Ff90774D0C6Bb92e5C5c8b4461F7"),
			QuoterV2:   common.HexToAddress("0x78D78E420Da98ad378D7799bE8f4AF69033EB077"),
			SwapRouter: common.HexToAddress("0xB971eF87ede563556b2ED4b1C0b0019111Dd85d2"),
			FeeTiers:   []uint32{FeeTier100, FeeTier500, FeeTier3000, FeeTier10000},
		},
	},
}

// LookupDeployment returns a deployment by chain and name. An empty name
// returns the chain's default.
func LookupDeployment(chainID evm.ChainID, name string) (Deployment, error) {
	list, ok := deployments[chainID]
	if !ok || len(list) == 0 {
		return Deployment{}, fmt.Errorf("no uniswap v3 style deployment is configured for chain %s", chainID)
	}

	if name == "" {
		d := list[0]
		d.ChainID = chainID

		return d, nil
	}

	wanted := strings.ToLower(strings.TrimSpace(name))
	for _, d := range list {
		if d.Name == wanted {
			d.ChainID = chainID
			return d, nil
		}
	}

	available := make([]string, 0, len(list))
	for _, d := range list {
		available = append(available, d.Name)
	}

	return Deployment{}, fmt.Errorf("unknown deployment %q on chain %s, available: %s",
		name, chainID, strings.Join(available, ", "))
}

// Deployments lists the configured deployments of a chain, the default first.
func Deployments(chainID evm.ChainID) []Deployment {
	list := deployments[chainID]

	out := make([]Deployment, 0, len(list))
	for _, d := range list {
		d.ChainID = chainID
		out = append(out, d)
	}

	return out
}
