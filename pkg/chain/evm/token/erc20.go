package token

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

const erc20ABIJSON = `[
  {"constant":true,"inputs":[],"name":"name","outputs":[{"name":"","type":"string"}],"type":"function"},
  {"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"type":"function"},
  {"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"type":"function"},
  {"constant":true,"inputs":[],"name":"totalSupply","outputs":[{"name":"","type":"uint256"}],"type":"function"},
  {"constant":true,"inputs":[{"name":"owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"},
  {"constant":true,"inputs":[{"name":"owner","type":"address"},{"name":"spender","type":"address"}],"name":"allowance","outputs":[{"name":"","type":"uint256"}],"type":"function"},
  {"constant":false,"inputs":[{"name":"spender","type":"address"},{"name":"amount","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"type":"function"}
]`

// erc20Bytes32ABIJSON covers the pre-standard tokens, such as MKR, whose
// name() and symbol() return a bytes32 instead of a string.
const erc20Bytes32ABIJSON = `[
  {"constant":true,"inputs":[],"name":"name","outputs":[{"name":"","type":"bytes32"}],"type":"function"},
  {"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"bytes32"}],"type":"function"}
]`

var (
	erc20ABI        = mustParseABI(erc20ABIJSON)
	erc20Bytes32ABI = mustParseABI(erc20Bytes32ABIJSON)
)

func mustParseABI(s string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(s))
	if err != nil {
		panic(fmt.Errorf("invalid built-in abi: %w", err))
	}

	return parsed
}

// ERC20ABI returns the minimal ERC20 ABI used by this package.
func ERC20ABI() abi.ABI {
	return erc20ABI
}

// CallData encodes a call to one of the minimal ERC20 methods. It is also used
// to build the batched reads passed to Multicall3.
func CallData(method string, args ...any) ([]byte, error) {
	return erc20ABI.Pack(method, args...)
}

// DecodeUint256 decodes the return data of a uint256-returning ERC20 call.
func DecodeUint256(method string, data []byte) (*big.Int, error) {
	out, err := erc20ABI.Unpack(method, data)
	if err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("empty return data for %s", method)
	}

	v, ok := out[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected return type %T for %s", out[0], method)
	}

	return v, nil
}

// DecodeDecimals decodes the return data of decimals().
func DecodeDecimals(data []byte) (int, error) {
	out, err := erc20ABI.Unpack("decimals", data)
	if err != nil {
		return 0, err
	}

	if len(out) == 0 {
		return 0, fmt.Errorf("empty return data for decimals")
	}

	v, ok := out[0].(uint8)
	if !ok {
		return 0, fmt.Errorf("unexpected return type %T for decimals", out[0])
	}

	return int(v), nil
}

// DecodeString decodes the return data of name() or symbol(), falling back to
// the bytes32 encoding used by pre-standard tokens.
func DecodeString(method string, data []byte) (string, error) {
	out, err := erc20ABI.Unpack(method, data)
	if err == nil && len(out) > 0 {
		if s, ok := out[0].(string); ok {
			return s, nil
		}
	}

	out, err32 := erc20Bytes32ABI.Unpack(method, data)
	if err32 == nil && len(out) > 0 {
		if b, ok := out[0].([32]byte); ok {
			return string(bytes.TrimRight(b[:], "\x00")), nil
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to decode %s: %w", method, err)
	}

	return "", fmt.Errorf("failed to decode %s", method)
}

func call(
	ctx context.Context, caller bind.ContractCaller, contract common.Address, method string, args ...any,
) ([]byte, error) {
	data, err := erc20ABI.Pack(method, args...)
	if err != nil {
		return nil, err
	}

	return caller.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: data}, nil)
}

// Symbol queries symbol() on an ERC20 contract.
func Symbol(ctx context.Context, caller bind.ContractCaller, contract common.Address) (string, error) {
	data, err := call(ctx, caller, contract, "symbol")
	if err != nil {
		return "", err
	}

	return DecodeString("symbol", data)
}

// Name queries name() on an ERC20 contract.
func Name(ctx context.Context, caller bind.ContractCaller, contract common.Address) (string, error) {
	data, err := call(ctx, caller, contract, "name")
	if err != nil {
		return "", err
	}

	return DecodeString("name", data)
}

// Decimals queries decimals() on an ERC20 contract. It must always be read from
// the chain: the same symbol has different decimals on different chains, for
// example USDT is 6 decimals on Ethereum and 18 decimals on BSC.
func Decimals(ctx context.Context, caller bind.ContractCaller, contract common.Address) (int, error) {
	data, err := call(ctx, caller, contract, "decimals")
	if err != nil {
		return 0, err
	}

	return DecodeDecimals(data)
}

// BalanceOf queries balanceOf(owner) and returns the raw base-unit balance.
func BalanceOf(
	ctx context.Context, caller bind.ContractCaller, contract, owner common.Address,
) (*big.Int, error) {
	data, err := call(ctx, caller, contract, "balanceOf", owner)
	if err != nil {
		return nil, err
	}

	return DecodeUint256("balanceOf", data)
}

// Allowance queries allowance(owner, spender) and returns the raw base-unit
// allowance.
func Allowance(
	ctx context.Context, caller bind.ContractCaller, contract, owner, spender common.Address,
) (*big.Int, error) {
	data, err := call(ctx, caller, contract, "allowance", owner, spender)
	if err != nil {
		return nil, err
	}

	return DecodeUint256("allowance", data)
}

// TotalSupply queries totalSupply().
func TotalSupply(ctx context.Context, caller bind.ContractCaller, contract common.Address) (*big.Int, error) {
	data, err := call(ctx, caller, contract, "totalSupply")
	if err != nil {
		return nil, err
	}

	return DecodeUint256("totalSupply", data)
}

// ApproveCallData encodes approve(spender, amount). Encoding only: this package
// does not send transactions.
func ApproveCallData(spender common.Address, amount *big.Int) ([]byte, error) {
	return erc20ABI.Pack("approve", spender, amount)
}
