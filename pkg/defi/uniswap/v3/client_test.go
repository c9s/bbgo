package v3

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/multicall"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

var (
	testFactory   = common.HexToAddress("0x0000000000000000000000000000000000000f01")
	testQuoter    = common.HexToAddress("0x0000000000000000000000000000000000000f02")
	testRouter    = common.HexToAddress("0x0000000000000000000000000000000000000f03")
	testMulticall = common.HexToAddress(multicall.Address)

	// testUSDC sorts below testWETH, matching the pool ordering invariant
	testUSDC = common.HexToAddress("0x00000000000000000000000000000000000000aa")
	testWETH = common.HexToAddress("0x00000000000000000000000000000000000000bb")
	testDAI  = common.HexToAddress("0x00000000000000000000000000000000000000cc")
	testUser = common.HexToAddress("0x1111111111111111111111111111111111111111")

	testPool500  = common.HexToAddress("0x0000000000000000000000000000000000000501")
	testPool3000 = common.HexToAddress("0x0000000000000000000000000000000000003001")
)

var testDeployment = Deployment{
	Name:       "test",
	Factory:    testFactory,
	QuoterV2:   testQuoter,
	SwapRouter: testRouter,
	FeeTiers:   []uint32{FeeTier500, FeeTier3000},
}

type fakePool struct {
	token0, token1 common.Address
	fee            uint32
	sqrtPriceX96   *big.Int
	liquidity      *big.Int
	tick           int64
	feeProtocol    uint32
	reverts        bool
}

// fakeDex answers the factory, pool, quoter and ERC20 reads of one deployment.
type fakeDex struct {
	noCode map[common.Address]bool

	// pools is keyed by fee tier
	pools map[uint32]common.Address
	state map[common.Address]*fakePool

	// quoteOut is the amountOut the quoter reports per fee tier; a missing tier
	// reverts the way the real quoter does when no pool exists
	quoteOut map[uint32]*big.Int

	allowances map[common.Address]*big.Int
	decimals   map[common.Address]uint8
	symbols    map[common.Address]string
}

func newFakeDex() *fakeDex {
	return &fakeDex{
		noCode: map[common.Address]bool{},
		pools: map[uint32]common.Address{
			FeeTier500:  testPool500,
			FeeTier3000: testPool3000,
		},
		state: map[common.Address]*fakePool{
			testPool500: {
				token0: testUSDC, token1: testWETH, fee: FeeTier500,
				sqrtPriceX96: Q96, liquidity: big.NewInt(1_000_000), tick: 10,
			},
			testPool3000: {
				token0: testUSDC, token1: testWETH, fee: FeeTier3000,
				sqrtPriceX96: Q96, liquidity: big.NewInt(5_000_000), tick: 12,
				// PancakeSwap widened feeProtocol to uint32; a Uniswap-only
				// uint8 decoding would fail on this pool
				feeProtocol: 100_000,
			},
		},
		quoteOut: map[uint32]*big.Int{
			FeeTier500:  big.NewInt(2000_000000),
			FeeTier3000: big.NewInt(1990_000000),
		},
		allowances: map[common.Address]*big.Int{},
		decimals:   map[common.Address]uint8{testUSDC: 6, testWETH: 18, testDAI: 18},
		symbols:    map[common.Address]string{testUSDC: "USDC", testWETH: "WETH", testDAI: "DAI"},
	}
}

func (f *fakeDex) CodeAt(_ context.Context, address common.Address, _ *big.Int) ([]byte, error) {
	if f.noCode[address] {
		return nil, nil
	}

	return []byte{0x01}, nil
}

func (f *fakeDex) CallContract(_ context.Context, msg ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	if msg.To != nil && *msg.To == testMulticall {
		return f.aggregate3(msg.Data)
	}

	return f.dispatch(*msg.To, msg.Data)
}

func (f *fakeDex) aggregate3(input []byte) ([]byte, error) {
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

func (f *fakeDex) dispatch(target common.Address, data []byte) ([]byte, error) {
	switch target {
	case testFactory:
		return f.dispatchFactory(data)
	case testQuoter:
		return f.dispatchQuoter(data)
	default:
		if _, ok := f.state[target]; ok {
			return f.dispatchPool(target, data)
		}

		return f.dispatchERC20(target, data)
	}
}

func (f *fakeDex) dispatchFactory(data []byte) ([]byte, error) {
	method, err := factoryABI.MethodById(data)
	if err != nil {
		return nil, err
	}

	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		return nil, err
	}

	tokenA := args[0].(common.Address)
	tokenB := args[1].(common.Address)
	fee := uint32(args[2].(*big.Int).Uint64())

	// only the USDC/WETH pair exists in the fake
	pair := (tokenA == testUSDC && tokenB == testWETH) || (tokenA == testWETH && tokenB == testUSDC)
	if !pair {
		return method.Outputs.Pack(common.Address{})
	}

	return method.Outputs.Pack(f.pools[fee])
}

func (f *fakeDex) dispatchPool(target common.Address, data []byte) ([]byte, error) {
	pool := f.state[target]
	if pool.reverts {
		return nil, errors.New("execution reverted")
	}

	method, err := poolABI.MethodById(data)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case "slot0":
		return method.Outputs.Pack(
			pool.sqrtPriceX96,
			big.NewInt(pool.tick),
			uint16(0), uint16(1), uint16(1),
			pool.feeProtocol,
			true)
	case "liquidity":
		return method.Outputs.Pack(pool.liquidity)
	case "token0":
		return method.Outputs.Pack(pool.token0)
	case "token1":
		return method.Outputs.Pack(pool.token1)
	case "fee":
		return method.Outputs.Pack(new(big.Int).SetUint64(uint64(pool.fee)))
	case "tickSpacing":
		return method.Outputs.Pack(big.NewInt(60))
	}

	return nil, errors.New("execution reverted: unknown pool method")
}

func (f *fakeDex) dispatchQuoter(data []byte) ([]byte, error) {
	method, err := quoterABI.MethodById(data)
	if err != nil {
		return nil, err
	}

	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case "quoteExactInputSingle":
		params := *abi.ConvertType(args[0], new(quoteParams)).(*quoteParams)

		out, ok := f.quoteOut[uint32(params.Fee.Uint64())]
		if !ok {
			return nil, errors.New("execution reverted")
		}

		return method.Outputs.Pack(out, Q96, uint32(1), big.NewInt(120000))

	case "quoteExactOutputSingle":
		params := *abi.ConvertType(args[0], new(quoteOutputParams)).(*quoteOutputParams)

		if _, ok := f.quoteOut[uint32(params.Fee.Uint64())]; !ok {
			return nil, errors.New("execution reverted")
		}

		// the fake prices the round trip exactly
		amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

		return method.Outputs.Pack(amountIn, Q96, uint32(1), big.NewInt(120000))
	}

	return nil, errors.New("execution reverted: unknown quoter method")
}

func (f *fakeDex) dispatchERC20(target common.Address, data []byte) ([]byte, error) {
	erc20 := token.ERC20ABI()

	method, err := erc20.MethodById(data)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case "allowance":
		allowance, ok := f.allowances[target]
		if !ok {
			allowance = big.NewInt(0)
		}

		return method.Outputs.Pack(allowance)
	case "decimals":
		return method.Outputs.Pack(f.decimals[target])
	case "symbol":
		return method.Outputs.Pack(f.symbols[target])
	}

	return nil, errors.New("execution reverted: unknown erc20 method")
}

type fakeSender struct {
	address  common.Address
	requests []evm.SendRequest
	receipt  *ethtypes.Receipt
}

func (s *fakeSender) Address() common.Address {
	return s.address
}

func (s *fakeSender) SendAndWait(_ context.Context, req evm.SendRequest) (*evm.TxResult, error) {
	s.requests = append(s.requests, req)

	return &evm.TxResult{
		Hash:    common.HexToHash("0xabc"),
		Status:  1,
		GasUsed: 150000,
		GasCost: big.NewInt(1),
		Receipt: s.receipt,
	}, nil
}

// swapReceipt builds a receipt carrying one pool Swap event. The amounts are
// the pool's own deltas, so the token leaving the pool carries the negative
// one; in the fake pair USDC is token0 and WETH is token1.
func swapReceipt(t *testing.T, pool common.Address, amount0, amount1 *big.Int) *ethtypes.Receipt {
	t.Helper()

	event := poolABI.Events["Swap"]

	data, err := event.Inputs.NonIndexed().Pack(amount0, amount1, Q96, big.NewInt(1000), big.NewInt(10))
	require.NoError(t, err)

	return &ethtypes.Receipt{
		Status: 1,
		Logs: []*ethtypes.Log{
			{Address: common.HexToAddress("0xdead"), Topics: []common.Hash{common.HexToHash("0x1")}},
			{Address: pool, Topics: []common.Hash{event.ID}, Data: data},
		},
	}
}

func newTestClient(t *testing.T, dex *fakeDex, sender TxSender) *Client {
	t.Helper()

	chain, ok := evm.LookupChain("ethereum")
	require.True(t, ok)

	deployment := testDeployment

	client, err := NewClient(context.Background(), chain, dex, Options{
		Override: &deployment,
		Sender:   sender,
	})
	require.NoError(t, err)

	return client
}

func TestNewClientChecksTheContracts(t *testing.T) {
	chain, _ := evm.LookupChain("ethereum")
	deployment := testDeployment

	dex := newFakeDex()
	dex.noCode[testQuoter] = true

	_, err := NewClient(context.Background(), chain, dex, Options{Override: &deployment})
	assert.ErrorContains(t, err, "no contract is deployed at the quoter address")

	// the router is only required when the client can write
	dex = newFakeDex()
	dex.noCode[testRouter] = true

	_, err = NewClient(context.Background(), chain, dex, Options{Override: &deployment})
	assert.NoError(t, err, "a read-only client needs no router")

	_, err = NewClient(context.Background(), chain, dex, Options{
		Override: &deployment,
		Sender:   &fakeSender{address: testUser},
	})
	assert.ErrorContains(t, err, "no contract is deployed at the router address")
}

func TestNewClientResolvesTheChainDefault(t *testing.T) {
	chain, _ := evm.LookupChain("bsc")

	// the fake answers for any address, so this only checks which deployment
	// was selected
	client, err := NewClient(context.Background(), chain, newFakeDex(), Options{})
	require.NoError(t, err)
	assert.Equal(t, "pancakeswap", client.Deployment().Name, "pancakeswap is the bsc default")
	assert.Contains(t, client.Deployment().FeeTiers, FeeTier2500)
	assert.NotContains(t, client.Deployment().FeeTiers, FeeTier3000)

	client, err = NewClient(context.Background(), chain, newFakeDex(), Options{Deployment: "uniswap"})
	require.NoError(t, err)
	assert.Equal(t, "uniswap", client.Deployment().Name)
	assert.Contains(t, client.Deployment().FeeTiers, FeeTier3000)

	_, err = NewClient(context.Background(), chain, newFakeDex(), Options{Deployment: "sushiswap"})
	assert.ErrorContains(t, err, "unknown deployment")
}

func TestNewClientGuardsAgainstATypedNilSender(t *testing.T) {
	var nilSender *evm.Sender

	chain, _ := evm.LookupChain("ethereum")
	deployment := testDeployment

	client, err := NewClient(context.Background(), chain, newFakeDex(), Options{
		Override: &deployment,
		Sender:   nilSender,
	})
	require.NoError(t, err)
	assert.False(t, client.CanWrite())

	_, err = client.Swap(context.Background(), SwapRequest{})
	assert.ErrorIs(t, err, ErrReadOnly)
}

func TestPoolAddress(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t, newFakeDex(), nil)

	address, err := client.PoolAddress(ctx, testUSDC, testWETH, FeeTier500)
	require.NoError(t, err)
	assert.Equal(t, testPool500, address)

	// a pair with no pool returns the zero address rather than an error
	address, err = client.PoolAddress(ctx, testUSDC, testDAI, FeeTier500)
	require.NoError(t, err)
	assert.Equal(t, common.Address{}, address)
}

func TestPoolDecodesWideFeeProtocol(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t, newFakeDex(), nil)

	// the 0.3% pool carries a PancakeSwap-style uint32 feeProtocol
	pool, err := client.Pool(ctx, testPool3000)
	require.NoError(t, err)

	assert.Equal(t, testPool3000, pool.Address)
	assert.Equal(t, testUSDC, pool.Token0)
	assert.Equal(t, testWETH, pool.Token1)
	assert.Equal(t, FeeTier3000, pool.Fee)
	assert.Equal(t, int32(60), pool.TickSpacing)
	assert.Equal(t, int32(12), pool.Tick)
	assert.Equal(t, Q96, pool.SqrtPriceX96)
	assert.True(t, pool.Unlocked)
}

func TestFindPoolsSortsByLiquidity(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t, newFakeDex(), nil)

	pools, err := client.FindPools(ctx, testUSDC, testWETH)
	require.NoError(t, err)
	require.Len(t, pools, 2)

	assert.Equal(t, testPool3000, pools[0].Address, "the deepest pool comes first")
	assert.Equal(t, testPool500, pools[1].Address)

	deepest, err := client.FindPool(ctx, testUSDC, testWETH)
	require.NoError(t, err)
	assert.Equal(t, testPool3000, deepest.Address)
}

func TestFindPoolsSkipsMissingAndBrokenPools(t *testing.T) {
	ctx := context.Background()

	dex := newFakeDex()
	delete(dex.pools, FeeTier500)          // no pool at this tier
	dex.state[testPool3000].reverts = true // and the other one is broken

	client := newTestClient(t, dex, nil)

	pools, err := client.FindPools(ctx, testUSDC, testWETH)
	require.NoError(t, err)
	assert.Empty(t, pools)

	_, err = client.FindPool(ctx, testUSDC, testWETH)
	assert.ErrorIs(t, err, ErrNoPool)
}

func TestQuote(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t, newFakeDex(), nil)

	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	quote, err := client.Quote(ctx, testWETH, testUSDC, FeeTier500, amountIn)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(2000_000000), quote.AmountOut)
	assert.Equal(t, FeeTier500, quote.Fee)
	assert.Equal(t, uint32(1), quote.InitializedTicksCrossed)
	assert.Equal(t, big.NewInt(120000), quote.GasEstimate)

	// a tier with no pool reverts, and the revert is surfaced as one
	_, err = client.Quote(ctx, testWETH, testUSDC, FeeTier10000, amountIn)
	assert.ErrorIs(t, err, evm.ErrReverted)

	_, err = client.Quote(ctx, testWETH, testUSDC, FeeTier500, big.NewInt(0))
	assert.Error(t, err)
}

func TestQuoteExactOut(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t, newFakeDex(), nil)

	quote, err := client.QuoteExactOut(ctx, testWETH, testUSDC, FeeTier500, big.NewInt(2000_000000))
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(2000_000000), quote.AmountOut)
	assert.Equal(t, new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil), quote.AmountIn)
}

func TestBestQuotePicksTheLargestOutput(t *testing.T) {
	ctx := context.Background()

	dex := newFakeDex()
	dex.quoteOut[FeeTier3000] = big.NewInt(2100_000000) // better than the 0.05% tier

	client := newTestClient(t, dex, nil)

	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	best, err := client.BestQuote(ctx, testWETH, testUSDC, amountIn)
	require.NoError(t, err)
	assert.Equal(t, FeeTier3000, best.Fee, "the best price wins, not the deepest pool")
	assert.Equal(t, big.NewInt(2100_000000), best.AmountOut)
}

func TestBestQuoteWithoutAnyPool(t *testing.T) {
	ctx := context.Background()

	dex := newFakeDex()
	dex.quoteOut = map[uint32]*big.Int{}

	client := newTestClient(t, dex, nil)

	_, err := client.BestQuote(ctx, testWETH, testUSDC,
		new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	assert.ErrorIs(t, err, ErrNoPool)
}

// decodeSwapCall reads back the router parameters of an encoded swap.
func decodeSwapCall(t *testing.T, req evm.SendRequest) exactInputSingleParams {
	t.Helper()

	assert.Equal(t, testRouter, *req.To)

	method, err := routerABI.MethodById(req.Data)
	require.NoError(t, err)
	require.Equal(t, "exactInputSingle", method.Name)

	args, err := method.Inputs.Unpack(req.Data[4:])
	require.NoError(t, err)

	return *abi.ConvertType(args[0], new(exactInputSingleParams)).(*exactInputSingleParams)
}

func TestSwap(t *testing.T) {
	ctx := context.Background()

	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	// the pool pays out 1995 USDC, slightly below the 2000 quote
	sender := &fakeSender{
		address: testUser,
		receipt: swapReceipt(t, testPool500, big.NewInt(-1995_000000), amountIn),
	}

	client := newTestClient(t, newFakeDex(), sender)

	result, err := client.Swap(ctx, SwapRequest{
		TokenIn:     testWETH,
		TokenOut:    testUSDC,
		Fee:         FeeTier500,
		AmountIn:    amountIn,
		SlippageBps: 50,
	})
	require.NoError(t, err)

	// the allowance is zero in the fake, so the approval comes first
	require.Len(t, sender.requests, 2)
	assert.Equal(t, testWETH, *sender.requests[0].To)
	assert.Equal(t, "approve WETH", sender.requests[0].Label)

	params := decodeSwapCall(t, sender.requests[1])
	assert.Equal(t, testWETH, params.TokenIn)
	assert.Equal(t, testUSDC, params.TokenOut)
	assert.Equal(t, big.NewInt(int64(FeeTier500)), params.Fee)
	assert.Equal(t, testUser, params.Recipient, "the sender is the default recipient")
	assert.Equal(t, amountIn, params.AmountIn)
	assert.Equal(t, big.NewInt(1990_000000), params.AmountOutMinimum, "2000 less 50 bps")
	assert.Equal(t, 0, params.SqrtPriceLimitX96.Sign())

	// the realized output comes from the pool's Swap event, not from the quote
	assert.Equal(t, big.NewInt(1995_000000), result.AmountOut)
	assert.Equal(t, testPool500, result.Pool)
	assert.InEpsilon(t, 0.0025, result.Slippage(), 1e-9, "1995 against a 2000 quote is 25 bps")
	assert.Contains(t, result.String(), "1995000000")
}

func TestSwapPicksTheBestTierWhenFeeIsZero(t *testing.T) {
	ctx := context.Background()

	dex := newFakeDex()
	dex.quoteOut[FeeTier3000] = big.NewInt(2100_000000)

	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	sender := &fakeSender{
		address: testUser,
		receipt: swapReceipt(t, testPool3000, big.NewInt(-2100_000000), amountIn),
	}

	client := newTestClient(t, dex, sender)

	result, err := client.Swap(ctx, SwapRequest{
		TokenIn:     testWETH,
		TokenOut:    testUSDC,
		AmountIn:    amountIn,
		SlippageBps: 100,
	})
	require.NoError(t, err)

	params := decodeSwapCall(t, sender.requests[1])
	assert.Equal(t, big.NewInt(int64(FeeTier3000)), params.Fee)
	assert.Equal(t, testPool3000, result.Pool)
}

func TestSwapHonoursAnExplicitMinimumAndRecipient(t *testing.T) {
	ctx := context.Background()

	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	sender := &fakeSender{
		address: testUser,
		receipt: swapReceipt(t, testPool500, big.NewInt(-1995_000000), amountIn),
	}

	client := newTestClient(t, newFakeDex(), sender)

	recipient := common.HexToAddress("0x2222222222222222222222222222222222222222")

	_, err := client.Swap(ctx, SwapRequest{
		TokenIn:          testWETH,
		TokenOut:         testUSDC,
		Fee:              FeeTier500,
		AmountIn:         amountIn,
		AmountOutMinimum: big.NewInt(1_500_000_000),
		Recipient:        recipient,
	})
	require.NoError(t, err)

	params := decodeSwapCall(t, sender.requests[1])
	assert.Equal(t, big.NewInt(1_500_000_000), params.AmountOutMinimum)
	assert.Equal(t, recipient, params.Recipient)
}

func TestSwapRejectsUnsafeRequests(t *testing.T) {
	ctx := context.Background()

	sender := &fakeSender{address: testUser}
	client := newTestClient(t, newFakeDex(), sender)

	amountIn := big.NewInt(1000)

	// no slippage bound and no explicit minimum
	_, err := client.Swap(ctx, SwapRequest{TokenIn: testWETH, TokenOut: testUSDC, AmountIn: amountIn})
	assert.ErrorContains(t, err, "slippageBps")

	_, err = client.Swap(ctx, SwapRequest{
		TokenIn: testWETH, TokenOut: testUSDC, AmountIn: amountIn, SlippageBps: 10000,
	})
	assert.ErrorContains(t, err, "slippageBps")

	// non-positive size
	_, err = client.Swap(ctx, SwapRequest{TokenIn: testWETH, TokenOut: testUSDC, SlippageBps: 50})
	assert.ErrorContains(t, err, "positive")

	// same token on both sides
	_, err = client.Swap(ctx, SwapRequest{
		TokenIn: testWETH, TokenOut: testWETH, AmountIn: amountIn, SlippageBps: 50,
	})
	assert.ErrorContains(t, err, "same")

	assert.Empty(t, sender.requests, "nothing may be sent for a rejected request")
}

func TestSwapSkipsTheApprovalWhenTheAllowanceIsEnough(t *testing.T) {
	ctx := context.Background()

	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	dex := newFakeDex()
	dex.allowances[testWETH] = new(big.Int).Mul(amountIn, big.NewInt(10))

	sender := &fakeSender{
		address: testUser,
		receipt: swapReceipt(t, testPool500, big.NewInt(-1995_000000), amountIn),
	}

	client := newTestClient(t, dex, sender)

	_, err := client.Swap(ctx, SwapRequest{
		TokenIn: testWETH, TokenOut: testUSDC, Fee: FeeTier500,
		AmountIn: amountIn, SlippageBps: 50,
	})
	require.NoError(t, err)

	require.Len(t, sender.requests, 1)
	decodeSwapCall(t, sender.requests[0])
}

func TestSwapAmountOut(t *testing.T) {
	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	// WETH in, USDC out: USDC sorts lower, so it is token0 and its delta is
	// the negative one
	receipt := swapReceipt(t, testPool500, big.NewInt(-1995_000000), amountIn)

	out, err := SwapAmountOut(receipt, testPool500, testWETH, testUSDC)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(1995_000000), out)

	// and the other direction: USDC in, WETH out
	receipt = swapReceipt(t, testPool500, big.NewInt(2000_000000), new(big.Int).Neg(amountIn))

	out, err = SwapAmountOut(receipt, testPool500, testUSDC, testWETH)
	require.NoError(t, err)
	assert.Equal(t, amountIn, out)

	// a receipt without the pool's event is an error rather than a zero
	_, err = SwapAmountOut(&ethtypes.Receipt{}, testPool500, testWETH, testUSDC)
	assert.ErrorContains(t, err, "no Swap event")
}

func TestTokenFallsBackToDirectReads(t *testing.T) {
	client := newTestClient(t, newFakeDex(), nil)

	t2, err := client.Token(context.Background(), testUSDC)
	require.NoError(t, err)
	assert.Equal(t, "USDC", t2.Symbol)
	assert.Equal(t, 6, t2.Decimals)
}

func TestDeploymentChecksum(t *testing.T) {
	for chainID, list := range deployments {
		for _, d := range list {
			for name, address := range map[string]common.Address{
				"factory": d.Factory,
				"quoter":  d.QuoterV2,
				"router":  d.SwapRouter,
			} {
				assert.NotEqualf(t, common.Address{}, address,
					"chain %s deployment %s: %s is unset", chainID, d.Name, name)
			}

			assert.NotEmptyf(t, d.FeeTiers, "chain %s deployment %s has no fee ladder", chainID, d.Name)
		}
	}
}

func TestLookupDeployment(t *testing.T) {
	d, err := LookupDeployment(evm.ChainIDEthereum, "")
	require.NoError(t, err)
	assert.Equal(t, "uniswap", d.Name)
	assert.Equal(t, evm.ChainIDEthereum, d.ChainID)

	d, err = LookupDeployment(evm.ChainIDBSC, "UNISWAP")
	require.NoError(t, err)
	assert.Equal(t, "uniswap", d.Name)

	_, err = LookupDeployment(evm.ChainID(42161), "")
	assert.ErrorContains(t, err, "no uniswap v3 style deployment")

	assert.Len(t, Deployments(evm.ChainIDBSC), 2)
	assert.Empty(t, Deployments(evm.ChainID(42161)))
}
