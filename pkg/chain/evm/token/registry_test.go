package token

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/chain/evm/multicall"
)

var testMulticallAddress = common.HexToAddress(multicall.Address)

type allowanceKey struct {
	owner   common.Address
	spender common.Address
}

type fakeToken struct {
	symbol        string
	name          string
	decimals      uint8
	bytes32Symbol bool
	balances      map[common.Address]*big.Int
	allowances    map[allowanceKey]*big.Int
	revertBalance bool
}

// fakeChain implements bind.ContractCaller over an in-memory set of ERC20
// contracts, including the Multicall3 aggregate3 entry point.
type fakeChain struct {
	tokens map[common.Address]*fakeToken
	calls  int
}

func (f *fakeChain) CodeAt(_ context.Context, _ common.Address, _ *big.Int) ([]byte, error) {
	return []byte{0x01}, nil
}

func (f *fakeChain) CallContract(_ context.Context, msg ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	f.calls++

	if msg.To != nil && *msg.To == testMulticallAddress {
		return f.aggregate3(msg.Data)
	}

	data, err := f.dispatch(*msg.To, msg.Data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (f *fakeChain) aggregate3(input []byte) ([]byte, error) {
	method := multicall.ABI().Methods["aggregate3"]

	args, err := method.Inputs.Unpack(input[4:])
	if err != nil {
		return nil, err
	}

	calls := *abi.ConvertType(args[0], new([]multicall.Call)).(*[]multicall.Call)

	results := make([]multicall.Result, 0, len(calls))
	for _, c := range calls {
		data, err := f.dispatch(c.Target, c.CallData)
		if err != nil {
			if !c.AllowFailure {
				return nil, err
			}

			results = append(results, multicall.Result{Success: false})
			continue
		}

		results = append(results, multicall.Result{Success: true, ReturnData: data})
	}

	return method.Outputs.Pack(results)
}

func (f *fakeChain) dispatch(target common.Address, callData []byte) ([]byte, error) {
	token, ok := f.tokens[target]
	if !ok {
		return nil, assertRevert("no contract at " + target.Hex())
	}

	erc20 := ERC20ABI()
	method, err := erc20.MethodById(callData)
	if err != nil {
		return nil, err
	}

	args, err := method.Inputs.Unpack(callData[4:])
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case "symbol":
		if token.bytes32Symbol {
			var b [32]byte
			copy(b[:], token.symbol)
			return packBytes32(b)
		}

		if token.symbol == "" {
			return nil, assertRevert("symbol() reverted")
		}

		return method.Outputs.Pack(token.symbol)

	case "name":
		if token.name == "" {
			return nil, assertRevert("name() reverted")
		}

		return method.Outputs.Pack(token.name)

	case "decimals":
		return method.Outputs.Pack(token.decimals)

	case "allowance":
		owner := args[0].(common.Address)
		spender := args[1].(common.Address)

		allowance, ok := token.allowances[allowanceKey{owner, spender}]
		if !ok {
			allowance = big.NewInt(0)
		}

		return method.Outputs.Pack(allowance)

	case "balanceOf":
		if token.revertBalance {
			return nil, assertRevert("balanceOf() reverted")
		}

		owner := args[0].(common.Address)
		balance, ok := token.balances[owner]
		if !ok {
			balance = big.NewInt(0)
		}

		return method.Outputs.Pack(balance)
	}

	return nil, assertRevert("unsupported method " + method.Name)
}

type assertRevert string

func (e assertRevert) Error() string { return "execution reverted: " + string(e) }

func packBytes32(b [32]byte) ([]byte, error) {
	arguments := abi.Arguments{{Type: mustType("bytes32")}}
	return arguments.Pack(b)
}

func mustType(s string) abi.Type {
	t, err := abi.NewType(s, "", nil)
	if err != nil {
		panic(err)
	}

	return t
}

var (
	testWBTC  = common.HexToAddress("0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599")
	testUSDT  = common.HexToAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7")
	testMKR   = common.HexToAddress("0x9f8F72aA9304c8B593d555F12eF6589cC3A579A2")
	testOwner = common.HexToAddress("0x1111111111111111111111111111111111111111")
)

func newFakeChain() *fakeChain {
	return &fakeChain{
		tokens: map[common.Address]*fakeToken{
			testWBTC: {
				symbol:   "WBTC",
				name:     "Wrapped BTC",
				decimals: 8,
				balances: map[common.Address]*big.Int{testOwner: big.NewInt(123456789)},
			},
			testUSDT: {
				symbol:   "USDT",
				name:     "Tether USD",
				decimals: 6,
				balances: map[common.Address]*big.Int{testOwner: big.NewInt(2500000)},
			},
			testMKR: {
				symbol:        "MKR",
				name:          "Maker",
				decimals:      18,
				bytes32Symbol: true,
			},
		},
	}
}

func TestRegistryLookup(t *testing.T) {
	ctx := context.Background()
	chain := newFakeChain()
	registry := NewRegistry(chain, testMulticallAddress)

	token, err := registry.Lookup(ctx, testWBTC)
	assert.NoError(t, err)
	assert.Equal(t, "WBTC", token.Symbol)
	assert.Equal(t, "Wrapped BTC", token.Name)
	assert.Equal(t, 8, token.Decimals)

	// metadata is read in one batched call
	assert.Equal(t, 1, chain.calls)

	// and cached afterwards
	_, err = registry.Lookup(ctx, testWBTC)
	assert.NoError(t, err)
	assert.Equal(t, 1, chain.calls)

	cached, ok := registry.Cached(testWBTC)
	assert.True(t, ok)
	assert.Equal(t, token, cached)
}

func TestRegistryLookupBytes32Symbol(t *testing.T) {
	registry := NewRegistry(newFakeChain(), testMulticallAddress)

	token, err := registry.Lookup(context.Background(), testMKR)
	assert.NoError(t, err)
	assert.Equal(t, "MKR", token.Symbol)
	assert.Equal(t, 18, token.Decimals)
}

func TestRegistryResolve(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry(newFakeChain(), testMulticallAddress)
	registry.AddSymbol("wbtc", testWBTC)

	token, err := registry.Resolve(ctx, "WBTC")
	assert.NoError(t, err)
	assert.Equal(t, testWBTC, token.Address)

	// addresses resolve without being registered
	token, err = registry.Resolve(ctx, testUSDT.Hex())
	assert.NoError(t, err)
	assert.Equal(t, "USDT", token.Symbol)

	_, err = registry.Resolve(ctx, "NOTATOKEN")
	assert.ErrorContains(t, err, "unknown token")

	assert.Equal(t, []string{"USDT", "WBTC"}, registry.Symbols())
}

func TestRegistryBalances(t *testing.T) {
	ctx := context.Background()
	chain := newFakeChain()
	chain.tokens[testMKR].revertBalance = true

	registry := NewRegistry(chain, testMulticallAddress)

	tokens, err := registry.ResolveAll(ctx, []string{testWBTC.Hex(), testUSDT.Hex(), testMKR.Hex()})
	assert.NoError(t, err)

	balances, err := registry.Balances(ctx, testOwner, tokens)
	assert.NoError(t, err)

	// the token whose balanceOf reverts is skipped, the others still report
	assert.Len(t, balances, 2)
	assert.Equal(t, "1.23456789 WBTC", balances[0].String())
	assert.Equal(t, "2.5 USDT", balances[1].String())
}

func TestERC20Calls(t *testing.T) {
	ctx := context.Background()
	chain := newFakeChain()

	symbol, err := Symbol(ctx, chain, testUSDT)
	assert.NoError(t, err)
	assert.Equal(t, "USDT", symbol)

	name, err := Name(ctx, chain, testUSDT)
	assert.NoError(t, err)
	assert.Equal(t, "Tether USD", name)

	decimals, err := Decimals(ctx, chain, testUSDT)
	assert.NoError(t, err)
	assert.Equal(t, 6, decimals)

	balance, err := BalanceOf(ctx, chain, testUSDT, testOwner)
	assert.NoError(t, err)
	assert.Equal(t, big.NewInt(2500000), balance)

	_, err = Decimals(ctx, chain, common.HexToAddress("0xdead"))
	assert.Error(t, err)
}

func TestApproveCallData(t *testing.T) {
	data, err := ApproveCallData(testWBTC, big.NewInt(1))
	assert.NoError(t, err)
	assert.Len(t, data, 4+32+32)

	erc20 := ERC20ABI()
	method, err := erc20.MethodById(data)
	assert.NoError(t, err)
	assert.Equal(t, "approve", method.Name)
}

func TestRegistryBalancesDeduplicatesAddresses(t *testing.T) {
	ctx := context.Background()
	chain := newFakeChain()
	registry := NewRegistry(chain, testMulticallAddress)

	// WPOL and WMATIC on Polygon are the same contract under two symbols
	tokens, err := registry.ResolveAll(ctx, []string{testWBTC.Hex(), testWBTC.Hex()})
	assert.NoError(t, err)
	assert.Len(t, tokens, 2)

	balances, err := registry.Balances(ctx, testOwner, tokens)
	assert.NoError(t, err)
	assert.Len(t, balances, 1)
}
