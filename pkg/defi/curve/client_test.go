package curve

import (
	"context"
	"errors"
	"fmt"
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
	testProvider  = common.HexToAddress("0x0000000000000000000000000000000000000e01")
	testRegistry  = common.HexToAddress("0x0000000000000000000000000000000000000e02")
	testMulticall = common.HexToAddress(multicall.Address)

	testUSDC = common.HexToAddress("0x00000000000000000000000000000000000000aa")
	testUSDT = common.HexToAddress("0x00000000000000000000000000000000000000bb")
	testDAI  = common.HexToAddress("0x00000000000000000000000000000000000000cc")
	testWETH = common.HexToAddress("0x00000000000000000000000000000000000000dd")
	testMIM  = common.HexToAddress("0x00000000000000000000000000000000000000ee")
	testLP   = common.HexToAddress("0x00000000000000000000000000000000000000ff")

	testUser = common.HexToAddress("0x1111111111111111111111111111111111111111")

	testStablePool = common.HexToAddress("0x0000000000000000000000000000000000001001")
	testCryptoPool = common.HexToAddress("0x0000000000000000000000000000000000001002")
	testMetaPool   = common.HexToAddress("0x0000000000000000000000000000000000001003")
	testBrokenPool = common.HexToAddress("0x0000000000000000000000000000000000001005")
)

var testChain = &evm.Chain{
	ID:         evm.ChainIDEthereum,
	Name:       "faketh",
	Multicall3: testMulticall,
}

// fakePool is one pool of the fake registry.
type fakePool struct {
	address common.Address

	name      string
	nameFails bool

	coins    []common.Address
	decimals []int
	balances []*big.Int

	underlyingCoins    []common.Address
	underlyingDecimals []int
	underlyingBalances []*big.Int

	isMeta bool

	// index is the index type the pool's ABI speaks. A call with the other one
	// fails the way the real pools do.
	index IndexKind

	// emptyOnWrongIndex makes the pool answer a wrongly typed call with an
	// empty payload instead of reverting, which is what a tricrypto-ng pool
	// does and what makes the length check in decodeQuoteResult necessary.
	emptyOnWrongIndex bool

	// underlyingOnly makes the pool answer get_dy_underlying only, the way a
	// metapool trades a coin of its base pool.
	underlyingOnly bool

	// priceNum over priceDen is the rate the pool quotes, before the decimals
	// of the two coins are applied.
	priceNum int64
	priceDen int64

	fee *big.Int

	a *big.Int

	indicesFail bool
}

func (p *fakePool) coinDecimals(underlying bool) []int {
	if underlying && len(p.underlyingDecimals) > 0 {
		return p.underlyingDecimals
	}

	return p.decimals
}

// quote prices a swap the way the fake pool would.
func (p *fakePool) quote(method string, i, j int64, dx *big.Int) (*big.Int, error) {
	underlying := method == "get_dy_underlying"

	if underlying != p.underlyingOnly {
		return nil, errors.New("execution reverted")
	}

	decimals := p.coinDecimals(underlying)
	if int(i) >= len(decimals) || int(j) >= len(decimals) {
		return nil, errors.New("execution reverted")
	}

	out := new(big.Int).Mul(dx, big.NewInt(p.priceNum))
	out.Div(out, big.NewInt(p.priceDen))

	if exponent := decimals[j] - decimals[i]; exponent > 0 {
		out.Mul(out, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exponent)), nil))
	} else if exponent < 0 {
		out.Div(out, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-exponent)), nil))
	}

	return out, nil
}

// fakeCurve answers the AddressProvider, the MetaRegistry, the pools and the
// ERC20 reads of one chain, through Multicall3 as well as directly.
type fakeCurve struct {
	noCode map[common.Address]bool

	pools []*fakePool

	// findFails makes find_pools_for_coins revert, which is how a chain whose
	// registry has no coin pair index behaves and what forces the pool scan.
	findFails bool

	allowances map[common.Address]*big.Int
	decimals   map[common.Address]uint8
	symbols    map[common.Address]string

	// quoteCalls counts the get_dy and get_dy_underlying calls, so a test can
	// assert that the search stopped early.
	quoteCalls int
}

func newFakeCurve() *fakeCurve {
	oneE12 := new(big.Int).Exp(big.NewInt(10), big.NewInt(12), nil)
	oneE18 := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	oneE24 := new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)

	return &fakeCurve{
		noCode: map[common.Address]bool{},
		pools: []*fakePool{
			{
				address:  testStablePool,
				name:     "3pool",
				coins:    []common.Address{testUSDC, testUSDT, testDAI},
				decimals: []int{6, 6, 18},
				balances: []*big.Int{oneE12, new(big.Int).Mul(oneE12, big.NewInt(2)), oneE24},
				index:    IndexInt128,
				priceNum: 1001, priceDen: 1000,
				fee: big.NewInt(1_500_000),
				a:   big.NewInt(2000),
			},
			{
				address:   testCryptoPool,
				nameFails: true,
				coins:     []common.Address{testUSDC, testWETH},
				decimals:  []int{6, 18},
				balances:  []*big.Int{oneE12, new(big.Int).Mul(oneE18, big.NewInt(500))},
				index:     IndexUint256,
				// the pool answers an int128 call with an empty payload
				emptyOnWrongIndex: true,
				priceNum:          1, priceDen: 2000,
				fee: big.NewInt(26_505_000),
			},
			{
				address:            testMetaPool,
				name:               "mim-3crv",
				isMeta:             true,
				coins:              []common.Address{testMIM, testLP},
				decimals:           []int{18, 18},
				balances:           []*big.Int{oneE24, oneE24},
				underlyingCoins:    []common.Address{testMIM, testUSDC, testUSDT, testDAI},
				underlyingDecimals: []int{18, 6, 6, 18},
				underlyingBalances: []*big.Int{oneE24, oneE12, oneE12, oneE24},
				index:              IndexInt128,
				underlyingOnly:     true,
				priceNum:           999, priceDen: 1000,
				fee: big.NewInt(4_000_000),
			},
			{
				address:     testBrokenPool,
				coins:       []common.Address{testUSDC, testUSDT},
				decimals:    []int{6, 6},
				balances:    []*big.Int{oneE12, oneE12},
				index:       IndexInt128,
				indicesFail: true,
				priceNum:    1, priceDen: 1,
			},
		},
		allowances: map[common.Address]*big.Int{},
		decimals: map[common.Address]uint8{
			testUSDC: 6, testUSDT: 6, testDAI: 18, testWETH: 18, testMIM: 18, testLP: 18,
		},
		symbols: map[common.Address]string{
			testUSDC: "USDC", testUSDT: "USDT", testDAI: "DAI",
			testWETH: "WETH", testMIM: "MIM", testLP: "3CRV",
		},
	}
}

// addShallowPool appends a pool that trades the same pair with less of the
// output coin than the pool before it, which is what the early stop prunes.
func (f *fakeCurve) addShallowPool(index int, balance *big.Int) *fakePool {
	pool := &fakePool{
		address:  common.BigToAddress(big.NewInt(int64(0x2000 + index))),
		name:     fmt.Sprintf("shallow-%d", index),
		coins:    []common.Address{testUSDC, testUSDT},
		decimals: []int{6, 6},
		balances: []*big.Int{balance, balance},
		index:    IndexInt128,
		priceNum: 1000, priceDen: 1000,
		fee: big.NewInt(4_000_000),
	}

	f.pools = append(f.pools, pool)

	return pool
}

func (f *fakeCurve) pool(address common.Address) *fakePool {
	for _, pool := range f.pools {
		if pool.address == address {
			return pool
		}
	}

	return nil
}

func (f *fakeCurve) CodeAt(_ context.Context, address common.Address, _ *big.Int) ([]byte, error) {
	if f.noCode[address] {
		return nil, nil
	}

	return []byte{0x01}, nil
}

func (f *fakeCurve) CallContract(_ context.Context, msg ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	if msg.To != nil && *msg.To == testMulticall {
		return f.aggregate3(msg.Data)
	}

	return f.dispatch(*msg.To, msg.Data)
}

func (f *fakeCurve) aggregate3(input []byte) ([]byte, error) {
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

		// a nil payload with no error is the successful empty answer that a
		// wrongly typed call gets from some pools
		results = append(results, multicall.Result{Success: true, ReturnData: data})
	}

	return method.Outputs.Pack(results)
}

func (f *fakeCurve) dispatch(target common.Address, data []byte) ([]byte, error) {
	switch target {
	case testProvider:
		return f.dispatchProvider(data)
	case testRegistry:
		return f.dispatchRegistry(data)
	}

	if pool := f.pool(target); pool != nil {
		return f.dispatchPool(pool, data)
	}

	return f.dispatchERC20(target, data)
}

func (f *fakeCurve) dispatchProvider(data []byte) ([]byte, error) {
	method, err := addressProviderABI.MethodById(data)
	if err != nil {
		return nil, err
	}

	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		return nil, err
	}

	if args[0].(*big.Int).Int64() != MetaRegistryID {
		return method.Outputs.Pack(common.Address{})
	}

	return method.Outputs.Pack(testRegistry)
}

func (f *fakeCurve) dispatchRegistry(data []byte) ([]byte, error) {
	method, err := metaRegistryABI.MethodById(data)
	if err != nil {
		return nil, err
	}

	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case "pool_count":
		return method.Outputs.Pack(big.NewInt(int64(len(f.pools))))

	case "pool_list":
		index := args[0].(*big.Int).Int64()
		if index < 0 || index >= int64(len(f.pools)) {
			return nil, errors.New("execution reverted")
		}

		return method.Outputs.Pack(f.pools[index].address)

	case "find_pools_for_coins":
		if f.findFails {
			return nil, errors.New("execution reverted")
		}

		from := args[0].(common.Address)
		to := args[1].(common.Address)

		var found []common.Address

		for _, pool := range f.pools {
			if containsBoth(pool.coins, from, to) || containsBoth(pool.underlyingCoins, from, to) {
				found = append(found, pool.address)
			}
		}

		return method.Outputs.Pack(found)
	}

	pool := f.pool(args[0].(common.Address))
	if pool == nil {
		return nil, errors.New("execution reverted")
	}

	switch method.Name {
	case "get_coin_indices":
		if pool.indicesFail {
			return nil, errors.New("execution reverted: No available market")
		}

		from := args[1].(common.Address)
		to := args[2].(common.Address)

		if i, j, ok := indexOfPair(pool.coins, from, to); ok {
			return method.Outputs.Pack(big.NewInt(i), big.NewInt(j), false)
		}

		if i, j, ok := indexOfPair(pool.underlyingCoins, from, to); ok {
			return method.Outputs.Pack(big.NewInt(i), big.NewInt(j), true)
		}

		return nil, errors.New("execution reverted: No available market")

	case "get_coins":
		return method.Outputs.Pack(toAddressArray(pool.coins))

	case "get_underlying_coins":
		if len(pool.underlyingCoins) == 0 {
			return method.Outputs.Pack(toAddressArray(pool.coins))
		}

		return method.Outputs.Pack(toAddressArray(pool.underlyingCoins))

	case "get_decimals":
		return method.Outputs.Pack(toUintArray(pool.decimals))

	case "get_underlying_decimals":
		if len(pool.underlyingDecimals) == 0 {
			return nil, errors.New("execution reverted")
		}

		return method.Outputs.Pack(toUintArray(pool.underlyingDecimals))

	case "get_balances":
		return method.Outputs.Pack(toBigArray(pool.balances))

	case "get_underlying_balances":
		if len(pool.underlyingBalances) == 0 {
			return nil, errors.New("execution reverted")
		}

		return method.Outputs.Pack(toBigArray(pool.underlyingBalances))

	case "get_n_coins":
		return method.Outputs.Pack(big.NewInt(int64(len(pool.coins))))

	case "get_fees":
		var fees [MaxFees]*big.Int
		for i := range fees {
			fees[i] = new(big.Int)
		}

		if pool.fee != nil {
			fees[0] = pool.fee
		}

		fees[1] = big.NewInt(5_000_000_000)

		return method.Outputs.Pack(fees)

	case "is_meta":
		return method.Outputs.Pack(pool.isMeta)

	case "get_pool_name":
		if pool.nameFails {
			return nil, errors.New("execution reverted")
		}

		return method.Outputs.Pack(pool.name)

	case "get_lp_token":
		return method.Outputs.Pack(testLP)

	case "get_base_pool":
		if !pool.isMeta {
			return method.Outputs.Pack(common.Address{})
		}

		return method.Outputs.Pack(testStablePool)
	}

	return nil, errors.New("execution reverted: unknown registry method")
}

// dispatchPool answers a pool call, but only when the selector matches the
// index type the pool speaks.
func (f *fakeCurve) dispatchPool(pool *fakePool, data []byte) ([]byte, error) {
	own := PoolABI(pool.index)
	other := PoolABI(IndexInt128)

	if pool.index == IndexInt128 {
		other = PoolABI(IndexUint256)
	}

	method, err := own.MethodById(data)
	if err != nil {
		// the selector belongs to the other index type, or to nothing at all
		if _, otherErr := other.MethodById(data); otherErr == nil {
			f.quoteCalls++

			if pool.emptyOnWrongIndex {
				return nil, nil
			}
		}

		return nil, errors.New("execution reverted")
	}

	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case "A":
		if pool.a == nil {
			return nil, errors.New("execution reverted")
		}

		return method.Outputs.Pack(pool.a)

	case "get_dy", "get_dy_underlying":
		f.quoteCalls++

		out, err := pool.quote(method.Name,
			args[0].(*big.Int).Int64(), args[1].(*big.Int).Int64(), args[2].(*big.Int))
		if err != nil {
			return nil, err
		}

		return method.Outputs.Pack(out)
	}

	return nil, errors.New("execution reverted: unknown pool method")
}

func (f *fakeCurve) dispatchERC20(target common.Address, data []byte) ([]byte, error) {
	erc20 := token.ERC20ABI()

	method, err := erc20.MethodById(data)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case "allowance":
		allowance, ok := f.allowances[target]
		if !ok {
			allowance = new(big.Int)
		}

		return method.Outputs.Pack(allowance)
	case "decimals":
		return method.Outputs.Pack(f.decimals[target])
	case "symbol":
		return method.Outputs.Pack(f.symbols[target])
	}

	return nil, errors.New("execution reverted: unknown erc20 method")
}

func indexOfPair(coins []common.Address, from, to common.Address) (int64, int64, bool) {
	i, j := int64(-1), int64(-1)

	for index, coin := range coins {
		switch coin {
		case from:
			i = int64(index)
		case to:
			j = int64(index)
		}
	}

	return i, j, i >= 0 && j >= 0 && i != j
}

func toAddressArray(coins []common.Address) [MaxCoins]common.Address {
	var out [MaxCoins]common.Address
	for i := 0; i < len(coins) && i < MaxCoins; i++ {
		out[i] = coins[i]
	}

	return out
}

func toUintArray(values []int) [MaxCoins]*big.Int {
	var out [MaxCoins]*big.Int
	for i := range out {
		out[i] = new(big.Int)
	}

	for i := 0; i < len(values) && i < MaxCoins; i++ {
		out[i] = big.NewInt(int64(values[i]))
	}

	return out
}

func toBigArray(values []*big.Int) [MaxCoins]*big.Int {
	var out [MaxCoins]*big.Int
	for i := range out {
		out[i] = new(big.Int)
	}

	for i := 0; i < len(values) && i < MaxCoins; i++ {
		out[i] = values[i]
	}

	return out
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

func newTestClient(t *testing.T, fake *fakeCurve, sender TxSender) *Client {
	t.Helper()

	client, err := NewClient(context.Background(), testChain, fake, Options{
		AddressProvider: testProvider,
		Sender:          sender,
		Multicall:       testMulticall,
	})
	require.NoError(t, err)

	return client
}

func TestNewClientResolvesTheRegistry(t *testing.T) {
	fake := newFakeCurve()

	client := newTestClient(t, fake, nil)

	assert.Equal(t, testRegistry, client.MetaRegistryAddress())
	assert.Equal(t, int64(len(fake.pools)), client.PoolCount())
	assert.False(t, client.CanWrite())
}

func TestNewClientRejectsAMissingDeployment(t *testing.T) {
	fake := newFakeCurve()
	fake.noCode[testProvider] = true

	_, err := NewClient(context.Background(), testChain, fake, Options{
		AddressProvider: testProvider,
		Multicall:       testMulticall,
	})
	assert.ErrorContains(t, err, "no contract is deployed")
}

func TestNewClientRejectsAChainWithoutAMetaRegistry(t *testing.T) {
	fake := newFakeCurve()
	fake.noCode[testRegistry] = true

	_, err := NewClient(context.Background(), testChain, fake, Options{
		AddressProvider: testProvider,
		Multicall:       testMulticall,
	})
	assert.ErrorContains(t, err, "no contract is deployed")
}

// a typed nil sender must not make the client look writable, which would panic
// on the first swap
func TestNewClientRejectsATypedNilSender(t *testing.T) {
	fake := newFakeCurve()

	var sender *evm.Sender

	client, err := NewClient(context.Background(), testChain, fake, Options{
		AddressProvider: testProvider,
		Sender:          sender,
		Multicall:       testMulticall,
	})
	require.NoError(t, err)

	assert.False(t, client.CanWrite())

	_, err = client.Swap(context.Background(), SwapRequest{
		TokenIn: testUSDC, TokenOut: testUSDT, AmountIn: big.NewInt(1), SlippageBps: 50,
	})
	assert.ErrorIs(t, err, ErrReadOnly)
}

func TestRoutesOrderedByOutputBalance(t *testing.T) {
	fake := newFakeCurve()
	client := newTestClient(t, fake, nil)

	routes, err := client.Routes(context.Background(), testUSDC, testUSDT)
	require.NoError(t, err)

	// the stable pool and the metapool trade the pair; the broken pool reports
	// no market and is dropped
	require.Len(t, routes, 2)

	for _, route := range routes {
		assert.NotEqual(t, testBrokenPool, route.Pool)
	}

	assert.GreaterOrEqual(t, routes[0].OutBalance.Cmp(routes[1].OutBalance), 0)

	// the metapool reaches USDT through its base pool, so the registry routes
	// it through the underlying coins and the balance bound is the base pool's
	for _, route := range routes {
		if route.Pool == testMetaPool {
			assert.True(t, route.Underlying)
			assert.Equal(t, "exchange_underlying", route.Method())
			assert.Equal(t, int64(1), route.I)
			assert.Equal(t, int64(2), route.J)
		}

		if route.Pool == testStablePool {
			assert.False(t, route.Underlying)
			assert.Equal(t, "exchange", route.Method())
			assert.Equal(t, int64(0), route.I)
			assert.Equal(t, int64(1), route.J)
		}
	}
}

func TestRoutesWithoutAPool(t *testing.T) {
	fake := newFakeCurve()
	client := newTestClient(t, fake, nil)

	_, err := client.Routes(context.Background(), testUSDC, common.HexToAddress("0xdead"))
	assert.ErrorIs(t, err, ErrNoPool)
}

// the index type is the one thing the registry does not report, so it is
// detected from the pool and then remembered
func TestQuoteDetectsTheIndexKind(t *testing.T) {
	fake := newFakeCurve()
	client := newTestClient(t, fake, nil)

	ctx := context.Background()

	stable, err := client.BestQuote(ctx, testUSDC, testUSDT, big.NewInt(1_000_000))
	require.NoError(t, err)
	assert.Equal(t, IndexInt128, stable.Route.Index)

	// the crypto pool answers an int128 call successfully, with an empty
	// payload, rather than reverting: the detection must still land on uint256
	// instead of reading that as an answer
	crypto, err := client.BestQuote(ctx, testUSDC, testWETH, big.NewInt(2000_000000))
	require.NoError(t, err)
	assert.Equal(t, IndexUint256, crypto.Route.Index)
	assert.Equal(t, testCryptoPool, crypto.Route.Pool)

	// 2000 USDC at 1 WETH per 2000 USDC, with the decimals applied
	assert.Equal(t, 0, crypto.AmountOut.Cmp(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)),
		"got %s", crypto.AmountOut)

	// the detected kind is cached, so quoting again costs one call per pool
	before := fake.quoteCalls

	_, err = client.BestQuote(ctx, testUSDC, testWETH, big.NewInt(2000_000000))
	require.NoError(t, err)

	assert.Equal(t, 1, fake.quoteCalls-before, "the index kind was not remembered")
}

func TestQuotesSkipPoolsTooShallowToWin(t *testing.T) {
	fake := newFakeCurve()

	// twelve pools that trade the pair with progressively less of the output
	// coin, all of them below what the stable pool quotes
	for i := 0; i < 12; i++ {
		fake.addShallowPool(i, big.NewInt(int64(1000-i)))
	}

	client := newTestClient(t, fake, nil)

	ctx := context.Background()

	amountIn := big.NewInt(1_000_000)

	quotes, err := client.Quotes(ctx, testUSDC, testUSDT, amountIn)
	require.NoError(t, err)
	require.NotEmpty(t, quotes)

	// the deep pools win, and the search stops instead of quoting the tail
	assert.Equal(t, testStablePool, quotes[0].Route.Pool)
	assert.LessOrEqual(t, len(quotes), 8)

	routes, err := client.Routes(ctx, testUSDC, testUSDT)
	require.NoError(t, err)
	assert.Greater(t, len(routes), len(quotes), "no pool was pruned")
}

func TestQuotesRespectTheMaxQuotedPools(t *testing.T) {
	fake := newFakeCurve()

	for i := 0; i < 12; i++ {
		// every pool holds more than the stable pool quotes, so nothing can be
		// pruned and only the cap can stop the search
		fake.addShallowPool(i, new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil))
	}

	client, err := NewClient(context.Background(), testChain, fake, Options{
		AddressProvider: testProvider,
		Multicall:       testMulticall,
		MaxQuotedPools:  3,
	})
	require.NoError(t, err)

	quotes, err := client.Quotes(context.Background(), testUSDC, testUSDT, big.NewInt(1_000_000))
	require.NoError(t, err)
	assert.LessOrEqual(t, len(quotes), 3)
}

// a chain whose registry cannot answer find_pools_for_coins must still find the
// pools, by walking the pool list
func TestFindPoolsFallsBackToScanning(t *testing.T) {
	fake := newFakeCurve()
	fake.findFails = true

	client := newTestClient(t, fake, nil)

	ctx := context.Background()

	pools, err := client.FindPoolAddresses(ctx, testUSDC, testUSDT)
	require.NoError(t, err)

	assert.Contains(t, pools, testStablePool)
	assert.Contains(t, pools, testMetaPool, "the scan missed a metapool's underlying coins")

	quote, err := client.BestQuote(ctx, testUSDC, testUSDT, big.NewInt(1_000_000))
	require.NoError(t, err)
	assert.Equal(t, testStablePool, quote.Route.Pool)
}

// an empty answer from a working registry is believed, so the scan is not paid
// for on every unknown pair
func TestFindPoolsTrustsAnEmptyAnswer(t *testing.T) {
	fake := newFakeCurve()
	client := newTestClient(t, fake, nil)

	pools, err := client.FindPoolAddresses(context.Background(), testWETH, testDAI)
	require.NoError(t, err)
	assert.Empty(t, pools)
	assert.Zero(t, fake.quoteCalls)
}

func TestPoolMetadata(t *testing.T) {
	fake := newFakeCurve()
	client := newTestClient(t, fake, nil)

	ctx := context.Background()

	stable, err := client.Pool(ctx, testStablePool)
	require.NoError(t, err)

	assert.Equal(t, "3pool", stable.Name)
	assert.False(t, stable.IsMeta)
	assert.Equal(t, []common.Address{testUSDC, testUSDT, testDAI}, stable.Coins)
	assert.Equal(t, []int{6, 6, 18}, stable.Decimals)
	assert.Len(t, stable.Balances, 3)
	assert.InDelta(t, 0.00015, stable.Fee(), 1e-9)
	assert.Equal(t, big.NewInt(2000), stable.A)

	// a plain pool's underlying coins are its own coins, which is noise
	assert.Empty(t, stable.UnderlyingCoins)

	meta, err := client.Pool(ctx, testMetaPool)
	require.NoError(t, err)

	assert.True(t, meta.IsMeta)
	assert.Equal(t, testStablePool, meta.BasePool)
	assert.Equal(t, []common.Address{testMIM, testUSDC, testUSDT, testDAI}, meta.UnderlyingCoins)
	assert.Equal(t, []int{18, 6, 6, 18}, meta.UnderlyingDecimals)

	// the crypto pool answers neither get_pool_name nor get_A, and must still
	// come back rather than fail the read
	crypto, err := client.Pool(ctx, testCryptoPool)
	require.NoError(t, err)

	assert.Empty(t, crypto.Name)
	assert.Nil(t, crypto.A)
	assert.Equal(t, []common.Address{testUSDC, testWETH}, crypto.Coins)
	assert.InDelta(t, 0.0026505, crypto.Fee(), 1e-9)
}
