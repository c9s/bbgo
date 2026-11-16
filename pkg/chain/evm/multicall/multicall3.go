// Package multicall batches read-only contract calls through Multicall3, which
// is deployed at the same address on Ethereum, BSC and Polygon.
//
// Batching matters here: reading a Uniswap pool, an Aave account or a set of
// token balances all mean "read N contracts at once", and doing that with N
// round trips makes the RPC endpoint the bottleneck.
package multicall

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

// Address is the canonical Multicall3 deployment address, identical on
// Ethereum, BSC and Polygon.
const Address = "0xcA11bde05977b3631167028862bE2a173976CA11"

// DefaultMaxCallsPerBatch bounds one aggregate3 call. Public RPC providers
// limit the response size and the eth_call gas, so large batches are split.
const DefaultMaxCallsPerBatch = 500

const multicall3ABIJSON = `[
  {
    "inputs":[{"components":[
      {"name":"target","type":"address"},
      {"name":"allowFailure","type":"bool"},
      {"name":"callData","type":"bytes"}],
      "name":"calls","type":"tuple[]"}],
    "name":"aggregate3",
    "outputs":[{"components":[
      {"name":"success","type":"bool"},
      {"name":"returnData","type":"bytes"}],
      "name":"returnData","type":"tuple[]"}],
    "stateMutability":"payable","type":"function"
  }
]`

var multicall3ABI = func() abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(multicall3ABIJSON))
	if err != nil {
		panic(fmt.Errorf("invalid built-in multicall3 abi: %w", err))
	}

	return parsed
}()

// Call is one entry of an aggregate3 batch. When AllowFailure is false, a
// revert in this call reverts the whole batch.
type Call struct {
	Target       common.Address
	AllowFailure bool
	CallData     []byte
}

// Result is the per-call outcome of an aggregate3 batch.
type Result struct {
	Success    bool
	ReturnData []byte
}

// Aggregate3 performs the batched eth_call. The returned slice has the same
// length and order as calls.
func Aggregate3(
	ctx context.Context, caller bind.ContractCaller, multicall common.Address, calls []Call,
) ([]Result, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	input, err := multicall3ABI.Pack("aggregate3", calls)
	if err != nil {
		return nil, fmt.Errorf("failed to encode aggregate3: %w", err)
	}

	output, err := caller.CallContract(ctx, ethereum.CallMsg{To: &multicall, Data: input}, nil)
	if err != nil {
		return nil, err
	}

	out, err := multicall3ABI.Unpack("aggregate3", output)
	if err != nil {
		return nil, fmt.Errorf("failed to decode aggregate3 result: %w", err)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("empty aggregate3 result")
	}

	results := *abi.ConvertType(out[0], new([]Result)).(*[]Result)
	if len(results) != len(calls) {
		return nil, fmt.Errorf("aggregate3 returned %d results for %d calls", len(results), len(calls))
	}

	return results, nil
}

// Aggregate3Chunked splits the batch into chunks of at most maxPerBatch calls.
// A maxPerBatch of zero uses DefaultMaxCallsPerBatch.
func Aggregate3Chunked(
	ctx context.Context, caller bind.ContractCaller, multicall common.Address, calls []Call, maxPerBatch int,
) ([]Result, error) {
	if maxPerBatch <= 0 {
		maxPerBatch = DefaultMaxCallsPerBatch
	}

	results := make([]Result, 0, len(calls))
	for start := 0; start < len(calls); start += maxPerBatch {
		end := start + maxPerBatch
		if end > len(calls) {
			end = len(calls)
		}

		chunk, err := Aggregate3(ctx, caller, multicall, calls[start:end])
		if err != nil {
			return nil, fmt.Errorf("failed to aggregate calls [%d, %d): %w", start, end, err)
		}

		results = append(results, chunk...)
	}

	return results, nil
}

// ABI returns the Multicall3 ABI fragment used by this package.
func ABI() abi.ABI {
	return multicall3ABI
}
