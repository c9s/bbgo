package v3

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/multicall"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

var (
	testProvider     = common.HexToAddress("0x0000000000000000000000000000000000000001")
	testPool         = common.HexToAddress("0x0000000000000000000000000000000000000002")
	testDataProvider = common.HexToAddress("0x0000000000000000000000000000000000000003")
	testOracle       = common.HexToAddress("0x0000000000000000000000000000000000000004")
	testMulticall    = common.HexToAddress(multicall.Address)

	testUSDC = common.HexToAddress("0x00000000000000000000000000000000000000aa")
	testWETH = common.HexToAddress("0x00000000000000000000000000000000000000bb")
	testUser = common.HexToAddress("0x00000000000000000000000000000000000000cc")
)

// fakeMarket answers the Aave v3 reads of one market, including through
// Multicall3, so the adapter can be tested without a node.
type fakeMarket struct {
	providerFails bool

	accountData []any

	reserves        []common.Address
	reserveData     map[common.Address][]any
	userReserveData map[common.Address][]any
	prices          map[common.Address]*big.Int

	allowances map[common.Address]*big.Int
	decimals   map[common.Address]uint8
	symbols    map[common.Address]string
}

func newFakeMarket() *fakeMarket {
	return &fakeMarket{
		accountData: []any{
			big.NewInt(200_00000000), // collateral, base currency with 8 decimals
			big.NewInt(50_00000000),  // debt
			big.NewInt(100_00000000), // available borrows
			big.NewInt(8000),         // liquidation threshold, bps
			big.NewInt(7500),         // ltv, bps
			wad(2.5),                 // health factor
		},
		reserves: []common.Address{testUSDC, testWETH},
		reserveData: map[common.Address][]any{
			testUSDC: reserveDataOf(ray(0.03), ray(0.05)),
			testWETH: reserveDataOf(ray(0.01), ray(0.02)),
		},
		userReserveData: map[common.Address][]any{
			testUSDC: userReserveDataOf(big.NewInt(1_000_000), big.NewInt(0), true),
			testWETH: userReserveDataOf(big.NewInt(0), big.NewInt(0), false),
		},
		prices: map[common.Address]*big.Int{
			testUSDC: big.NewInt(1_0000000),
			testWETH: big.NewInt(3000_00000000),
		},
		allowances: map[common.Address]*big.Int{},
		decimals:   map[common.Address]uint8{testUSDC: 6, testWETH: 18},
		symbols:    map[common.Address]string{testUSDC: "USDC", testWETH: "WETH"},
	}
}

func reserveDataOf(liquidityRate, borrowRate *big.Int) []any {
	return []any{
		big.NewInt(0),          // unbacked
		big.NewInt(0),          // accruedToTreasuryScaled
		big.NewInt(1_000_000),  // totalAToken
		big.NewInt(0),          // totalStableDebt
		big.NewInt(500_000),    // totalVariableDebt
		liquidityRate,          // liquidityRate
		borrowRate,             // variableBorrowRate
		big.NewInt(0),          // stableBorrowRate
		big.NewInt(0),          // averageStableBorrowRate
		ray(1.05),              // liquidityIndex
		ray(1.07),              // variableBorrowIndex
		big.NewInt(1762500000), // lastUpdateTimestamp, uint40
	}
}

func userReserveDataOf(aTokenBalance, variableDebt *big.Int, collateral bool) []any {
	return []any{
		aTokenBalance,
		big.NewInt(0), // currentStableDebt
		variableDebt,
		big.NewInt(0), // principalStableDebt
		big.NewInt(0), // scaledVariableDebt
		big.NewInt(0), // stableBorrowRate
		ray(0.03),     // liquidityRate
		big.NewInt(0), // stableRateLastUpdated, uint40
		collateral,
	}
}

func (f *fakeMarket) CodeAt(_ context.Context, _ common.Address, _ *big.Int) ([]byte, error) {
	return []byte{0x01}, nil
}

func (f *fakeMarket) CallContract(_ context.Context, msg ethereum.CallMsg, _ *big.Int) ([]byte, error) {
	if msg.To != nil && *msg.To == testMulticall {
		return f.aggregate3(msg.Data)
	}

	return f.dispatch(*msg.To, msg.Data)
}

func (f *fakeMarket) aggregate3(input []byte) ([]byte, error) {
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

func (f *fakeMarket) dispatch(target common.Address, data []byte) ([]byte, error) {
	switch target {
	case testProvider:
		if f.providerFails {
			return nil, errors.New("execution reverted")
		}

		return f.dispatchProvider(data)

	case testPool:
		return f.dispatchPool(data)

	case testDataProvider:
		return f.dispatchDataProvider(data)

	case testOracle:
		return f.dispatchOracle(data)

	default:
		return f.dispatchERC20(target, data)
	}
}

func (f *fakeMarket) dispatchProvider(data []byte) ([]byte, error) {
	contractABI := addressesProviderABI

	method, err := contractABI.MethodById(data)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case "getPool":
		return method.Outputs.Pack(testPool)
	case "getPoolDataProvider":
		return method.Outputs.Pack(testDataProvider)
	case "getPriceOracle":
		return method.Outputs.Pack(testOracle)
	case "getMarketId":
		return method.Outputs.Pack("Test Market")
	}

	return nil, errors.New("execution reverted: unknown provider method")
}

func (f *fakeMarket) dispatchPool(data []byte) ([]byte, error) {
	contractABI := poolABI

	method, err := contractABI.MethodById(data)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case "getUserAccountData":
		return method.Outputs.Pack(f.accountData...)
	case "getReservesList":
		return method.Outputs.Pack(f.reserves)
	}

	return nil, errors.New("execution reverted: unknown pool method")
}

func (f *fakeMarket) dispatchDataProvider(data []byte) ([]byte, error) {
	contractABI := dataProviderABI

	method, err := contractABI.MethodById(data)
	if err != nil {
		return nil, err
	}

	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		return nil, err
	}

	asset := args[0].(common.Address)

	switch method.Name {
	case "getReserveData":
		values, ok := f.reserveData[asset]
		if !ok {
			return nil, errors.New("execution reverted: unknown reserve")
		}

		return method.Outputs.Pack(values...)

	case "getUserReserveData":
		values, ok := f.userReserveData[asset]
		if !ok {
			return nil, errors.New("execution reverted: unknown reserve")
		}

		return method.Outputs.Pack(values...)

	case "getReserveConfigurationData":
		return method.Outputs.Pack(
			big.NewInt(int64(f.decimals[asset])),
			big.NewInt(7500), big.NewInt(8000), big.NewInt(10500), big.NewInt(1000),
			true, true, false, true, false)

	case "getReserveTokensAddresses":
		return method.Outputs.Pack(
			common.HexToAddress("0x00000000000000000000000000000000000000a1"),
			common.Address{},
			common.HexToAddress("0x00000000000000000000000000000000000000a3"))
	}

	return nil, errors.New("execution reverted: unknown data provider method")
}

func (f *fakeMarket) dispatchOracle(data []byte) ([]byte, error) {
	contractABI := oracleABI

	method, err := contractABI.MethodById(data)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case "BASE_CURRENCY_UNIT":
		return method.Outputs.Pack(big.NewInt(1e8))

	case "BASE_CURRENCY":
		return method.Outputs.Pack(common.Address{})

	case "getAssetPrice":
		args, err := method.Inputs.Unpack(data[4:])
		if err != nil {
			return nil, err
		}

		price, ok := f.prices[args[0].(common.Address)]
		if !ok {
			return nil, errors.New("execution reverted: no price")
		}

		return method.Outputs.Pack(price)

	case "getAssetsPrices":
		args, err := method.Inputs.Unpack(data[4:])
		if err != nil {
			return nil, err
		}

		assets := args[0].([]common.Address)

		prices := make([]*big.Int, 0, len(assets))
		for _, asset := range assets {
			price, ok := f.prices[asset]
			if !ok {
				return nil, errors.New("execution reverted: no price")
			}

			prices = append(prices, price)
		}

		return method.Outputs.Pack(prices)
	}

	return nil, errors.New("execution reverted: unknown oracle method")
}

func (f *fakeMarket) dispatchERC20(target common.Address, data []byte) ([]byte, error) {
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

// fakeSender records the transactions the adapter would send.
type fakeSender struct {
	address  common.Address
	requests []evm.SendRequest
}

func (s *fakeSender) Address() common.Address {
	return s.address
}

func (s *fakeSender) SendAndWait(_ context.Context, req evm.SendRequest) (*evm.TxResult, error) {
	s.requests = append(s.requests, req)

	return &evm.TxResult{Status: 1, GasUsed: 100000, GasCost: big.NewInt(1)}, nil
}

func newTestClient(t *testing.T, market *fakeMarket, sender TxSender) *Client {
	t.Helper()

	chain, ok := evm.LookupChain("ethereum")
	require.True(t, ok)

	client, err := NewClient(context.Background(), chain, market, Options{
		Provider: testProvider,
		Sender:   sender,
	})
	require.NoError(t, err)

	return client
}

func TestNewClientResolvesTheMarket(t *testing.T) {
	client := newTestClient(t, newFakeMarket(), nil)

	addresses := client.Addresses()
	assert.Equal(t, testProvider, addresses.Provider)
	assert.Equal(t, testPool, addresses.Pool)
	assert.Equal(t, testDataProvider, addresses.DataProvider)
	assert.Equal(t, testOracle, addresses.Oracle)
	assert.Equal(t, "Test Market", addresses.MarketID)
	assert.Equal(t, 8, addresses.BaseCurrencyDecimals())
	assert.Equal(t, testPool, client.PoolAddress())
	assert.False(t, client.CanWrite())
}

func TestNewClientRejectsANonMarket(t *testing.T) {
	market := newFakeMarket()
	market.providerFails = true

	chain, _ := evm.LookupChain("ethereum")

	_, err := NewClient(context.Background(), chain, market, Options{Provider: testProvider})
	assert.ErrorContains(t, err, "aave v3")
}

func TestNewClientUsesTheChainProvider(t *testing.T) {
	chain, _ := evm.LookupChain("bsc")

	// the built-in provider address is not the one the fake answers for, so
	// this only checks that the lookup happens
	_, err := NewClient(context.Background(), chain, newFakeMarket(), Options{})
	assert.Error(t, err)
}

func TestAccountData(t *testing.T) {
	client := newTestClient(t, newFakeMarket(), nil)

	account, err := client.AccountData(context.Background(), testUser)
	require.NoError(t, err)

	assert.Equal(t, 200.0, account.TotalCollateral())
	assert.Equal(t, 50.0, account.TotalDebt())
	assert.Equal(t, 100.0, account.AvailableBorrows())
	assert.InDelta(t, 2.5, account.HealthFactorFloat(), 1e-12)
	assert.Equal(t, 0.75, BpsToFloat(account.LTVBps))
	assert.True(t, account.HasDebt())
	assert.False(t, account.IsLiquidatable())
}

func TestReserves(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t, newFakeMarket(), nil)

	assets, err := client.ReservesList(ctx)
	require.NoError(t, err)
	assert.Equal(t, []common.Address{testUSDC, testWETH}, assets)

	reserve, err := client.Reserve(ctx, testUSDC)
	require.NoError(t, err)
	assert.Equal(t, testUSDC, reserve.Asset)
	assert.InDelta(t, 0.03, APR(reserve.LiquidityRate), 1e-12)
	assert.Greater(t, reserve.BorrowAPY(), reserve.SupplyAPY())
	assert.Equal(t, int64(1762500000), reserve.LastUpdated.Unix())

	// the whole market in one batched call
	reserves, err := client.Reserves(ctx)
	require.NoError(t, err)
	assert.Len(t, reserves, 2)
}

func TestUserReservesSkipsEmptyPositions(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t, newFakeMarket(), nil)

	position, err := client.UserReserve(ctx, testUSDC, testUser)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(1_000_000), position.ATokenBalance)
	assert.True(t, position.UsageAsCollateralEnabled)

	// WETH holds nothing, so only USDC is returned
	positions, err := client.UserReserves(ctx, testUser)
	require.NoError(t, err)
	require.Len(t, positions, 1)
	assert.Equal(t, testUSDC, positions[0].Asset)
}

func TestReserveConfigAndTokens(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t, newFakeMarket(), nil)

	config, err := client.ReserveConfig(ctx, testUSDC)
	require.NoError(t, err)
	assert.Equal(t, 6, config.Decimals)
	assert.Equal(t, 0.75, BpsToFloat(config.LTVBps))
	assert.Equal(t, 0.8, BpsToFloat(config.LiquidationThresholdBps))
	assert.True(t, config.IsActive)
	assert.False(t, config.IsFrozen)

	tokens, err := client.ReserveTokens(ctx, testUSDC)
	require.NoError(t, err)
	assert.NotEqual(t, common.Address{}, tokens.AToken)
	assert.Equal(t, common.Address{}, tokens.StableDebtToken, "v3 no longer deploys stable debt tokens")
}

func TestAssetPrices(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t, newFakeMarket(), nil)

	price, err := client.AssetPrice(ctx, testWETH)
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(3000_00000000), price)

	prices, err := client.AssetPrices(ctx, []common.Address{testUSDC, testWETH})
	require.NoError(t, err)
	assert.Len(t, prices, 2)
	assert.Equal(t, big.NewInt(1_0000000), prices[testUSDC])
}

// decodePoolCall reads back the arguments of an encoded Pool call.
func decodePoolCall(t *testing.T, req evm.SendRequest, name string) []any {
	t.Helper()

	assert.Equal(t, testPool, *req.To)

	method, err := poolABI.MethodById(req.Data)
	require.NoError(t, err)
	require.Equal(t, name, method.Name)

	args, err := method.Inputs.Unpack(req.Data[4:])
	require.NoError(t, err)

	return args
}

func TestSupplyApprovesThenSupplies(t *testing.T) {
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, newFakeMarket(), sender)

	amount := big.NewInt(2_500_000)

	result, err := client.Supply(context.Background(), testUSDC, amount)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), result.Status)

	// the allowance is zero in the fake, so an approval comes first
	require.Len(t, sender.requests, 2)
	assert.Equal(t, testUSDC, *sender.requests[0].To)
	assert.Equal(t, "approve USDC", sender.requests[0].Label)

	args := decodePoolCall(t, sender.requests[1], "supply")
	assert.Equal(t, testUSDC, args[0])
	assert.Equal(t, amount, args[1])
	assert.Equal(t, testUser, args[2], "supplied on behalf of the sender")
	assert.Equal(t, uint16(0), args[3])
}

func TestSupplySkipsTheApprovalWhenTheAllowanceIsEnough(t *testing.T) {
	market := newFakeMarket()
	market.allowances[testUSDC] = big.NewInt(1_000_000_000)

	sender := &fakeSender{address: testUser}
	client := newTestClient(t, market, sender)

	_, err := client.Supply(context.Background(), testUSDC, big.NewInt(2_500_000))
	require.NoError(t, err)

	require.Len(t, sender.requests, 1)
	decodePoolCall(t, sender.requests[0], "supply")
}

func TestWithdraw(t *testing.T) {
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, newFakeMarket(), sender)

	_, err := client.Withdraw(context.Background(), testUSDC, MaxAmount)
	require.NoError(t, err)

	require.Len(t, sender.requests, 1, "withdrawing needs no approval")

	args := decodePoolCall(t, sender.requests[0], "withdraw")
	assert.Equal(t, testUSDC, args[0])
	assert.Equal(t, MaxAmount, args[1])
	assert.Equal(t, testUser, args[2])
}

func TestBorrowUsesTheVariableRateMode(t *testing.T) {
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, newFakeMarket(), sender)

	_, err := client.Borrow(context.Background(), testWETH, big.NewInt(1000))
	require.NoError(t, err)

	args := decodePoolCall(t, sender.requests[0], "borrow")
	assert.Equal(t, testWETH, args[0])
	assert.Equal(t, big.NewInt(1000), args[1])
	assert.Equal(t, big.NewInt(InterestRateModeVariable), args[2])
	assert.Equal(t, testUser, args[4])
}

func TestRepay(t *testing.T) {
	market := newFakeMarket()
	market.userReserveData[testUSDC] = userReserveDataOf(big.NewInt(0), big.NewInt(1_000_000), false)

	sender := &fakeSender{address: testUser}
	client := newTestClient(t, market, sender)

	_, err := client.Repay(context.Background(), testUSDC, big.NewInt(400_000))
	require.NoError(t, err)

	require.Len(t, sender.requests, 2)
	assert.Equal(t, "approve USDC", sender.requests[0].Label)

	args := decodePoolCall(t, sender.requests[1], "repay")
	assert.Equal(t, big.NewInt(400_000), args[1])
	assert.Equal(t, big.NewInt(InterestRateModeVariable), args[2])
}

func TestRepayAllApprovesABufferOverTheDebt(t *testing.T) {
	market := newFakeMarket()
	market.userReserveData[testUSDC] = userReserveDataOf(big.NewInt(0), big.NewInt(1_000_000), false)

	sender := &fakeSender{address: testUser}
	client := newTestClient(t, market, sender)

	_, err := client.RepayAll(context.Background(), testUSDC)
	require.NoError(t, err)

	require.Len(t, sender.requests, 2)

	// the debt keeps accruing between the approval and the repayment, so the
	// allowance covers a little more than the current debt
	erc20 := token.ERC20ABI()
	method, err := erc20.MethodById(sender.requests[0].Data)
	require.NoError(t, err)
	require.Equal(t, "approve", method.Name)

	approveArgs, err := method.Inputs.Unpack(sender.requests[0].Data[4:])
	require.NoError(t, err)
	assert.Equal(t, testPool, approveArgs[0])
	assert.Equal(t, big.NewInt(1_005_000), approveArgs[1])

	// the repayment itself asks the pool to take exactly what is owed
	args := decodePoolCall(t, sender.requests[1], "repay")
	assert.Equal(t, MaxAmount, args[1])
}

func TestRepayAllWithoutDebt(t *testing.T) {
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, newFakeMarket(), sender)

	_, err := client.RepayAll(context.Background(), testWETH)
	assert.ErrorContains(t, err, "no debt to repay")
	assert.Empty(t, sender.requests)
}

func TestRepayMaxAmountRepaysEverything(t *testing.T) {
	market := newFakeMarket()
	market.userReserveData[testUSDC] = userReserveDataOf(big.NewInt(0), big.NewInt(1_000_000), false)

	sender := &fakeSender{address: testUser}
	client := newTestClient(t, market, sender)

	_, err := client.Repay(context.Background(), testUSDC, MaxAmount)
	require.NoError(t, err)

	args := decodePoolCall(t, sender.requests[1], "repay")
	assert.Equal(t, MaxAmount, args[1])
}

func TestSetUseAsCollateral(t *testing.T) {
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, newFakeMarket(), sender)

	_, err := client.SetUseAsCollateral(context.Background(), testUSDC, false)
	require.NoError(t, err)

	args := decodePoolCall(t, sender.requests[0], "setUserUseReserveAsCollateral")
	assert.Equal(t, testUSDC, args[0])
	assert.Equal(t, false, args[1])
}

func TestWriteMethodsRequireASender(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t, newFakeMarket(), nil)

	_, err := client.Supply(ctx, testUSDC, big.NewInt(1))
	assert.ErrorIs(t, err, ErrReadOnly)

	_, err = client.Withdraw(ctx, testUSDC, big.NewInt(1))
	assert.ErrorIs(t, err, ErrReadOnly)

	_, err = client.Borrow(ctx, testUSDC, big.NewInt(1))
	assert.ErrorIs(t, err, ErrReadOnly)

	_, err = client.Repay(ctx, testUSDC, big.NewInt(1))
	assert.ErrorIs(t, err, ErrReadOnly)

	_, err = client.RepayAll(ctx, testUSDC)
	assert.ErrorIs(t, err, ErrReadOnly)

	_, err = client.SetUseAsCollateral(ctx, testUSDC, true)
	assert.ErrorIs(t, err, ErrReadOnly)
}

func TestWriteMethodsRejectNonPositiveAmounts(t *testing.T) {
	ctx := context.Background()
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, newFakeMarket(), sender)

	for _, amount := range []*big.Int{nil, big.NewInt(0), big.NewInt(-1)} {
		_, err := client.Supply(ctx, testUSDC, amount)
		assert.Error(t, err)

		_, err = client.Withdraw(ctx, testUSDC, amount)
		assert.Error(t, err)

		_, err = client.Borrow(ctx, testUSDC, amount)
		assert.Error(t, err)

		_, err = client.Repay(ctx, testUSDC, amount)
		assert.Error(t, err)
	}

	assert.Empty(t, sender.requests)
}

func TestTokenFallsBackToDirectReads(t *testing.T) {
	client := newTestClient(t, newFakeMarket(), nil)

	t2, err := client.Token(context.Background(), testUSDC)
	require.NoError(t, err)
	assert.Equal(t, "USDC", t2.Symbol)
	assert.Equal(t, 6, t2.Decimals)
}

func TestNonNilSenderGuardsAgainstATypedNil(t *testing.T) {
	// a caller passing a nil *evm.Sender into the interface field would
	// otherwise make CanWrite report true and panic on the first write
	var nilSender *evm.Sender

	chain, _ := evm.LookupChain("ethereum")

	client, err := NewClient(context.Background(), chain, newFakeMarket(), Options{
		Provider: testProvider,
		Sender:   nilSender,
	})
	require.NoError(t, err)

	assert.False(t, client.CanWrite())

	_, err = client.Supply(context.Background(), testUSDC, big.NewInt(1))
	assert.ErrorIs(t, err, ErrReadOnly)
}
