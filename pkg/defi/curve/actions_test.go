package curve

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

// decodeExchange reads back one recorded exchange call.
func decodeExchange(t *testing.T, kind IndexKind, data []byte) (string, []any) {
	t.Helper()

	poolABI := PoolABI(kind)

	method, err := poolABI.MethodById(data)
	require.NoError(t, err)

	args, err := method.Inputs.Unpack(data[4:])
	require.NoError(t, err)

	return method.Name, args
}

func decodeApprove(t *testing.T, data []byte) (common.Address, *big.Int) {
	t.Helper()

	erc20 := token.ERC20ABI()

	method, err := erc20.MethodById(data)
	require.NoError(t, err)
	require.Equal(t, "approve", method.Name)

	args, err := method.Inputs.Unpack(data[4:])
	require.NoError(t, err)

	return args[0].(common.Address), args[1].(*big.Int)
}

func TestSwapThroughAStablePool(t *testing.T) {
	fake := newFakeCurve()
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, fake, sender)

	ctx := context.Background()

	amountIn := big.NewInt(1_000_000)

	quote, err := client.BestQuote(ctx, testUSDC, testUSDT, amountIn)
	require.NoError(t, err)
	require.Equal(t, testStablePool, quote.Route.Pool)

	result, err := client.Swap(ctx, SwapRequest{
		TokenIn:     testUSDC,
		TokenOut:    testUSDT,
		AmountIn:    amountIn,
		SlippageBps: 50,
	})
	require.NoError(t, err)

	// the allowance goes to the pool: on Curve there is no router to approve
	require.Len(t, sender.requests, 2)

	spender, allowance := decodeApprove(t, sender.requests[0].Data)
	assert.Equal(t, testStablePool, spender)
	assert.Equal(t, 0, allowance.Cmp(amountIn))
	assert.Equal(t, testUSDC, *sender.requests[0].To)

	assert.Equal(t, testStablePool, *sender.requests[1].To)

	name, args := decodeExchange(t, IndexInt128, sender.requests[1].Data)
	assert.Equal(t, "exchange", name)
	assert.Equal(t, int64(0), args[0].(*big.Int).Int64())
	assert.Equal(t, int64(1), args[1].(*big.Int).Int64())
	assert.Equal(t, 0, args[2].(*big.Int).Cmp(amountIn))

	// min_dy is the quote minus 50 bps
	expected := AmountOutMinimum(quote.AmountOut, 50)
	assert.Equal(t, 0, args[3].(*big.Int).Cmp(expected), "min_dy is %s, want %s", args[3], expected)
	assert.Equal(t, 0, result.AmountOutMinimum.Cmp(expected))
}

func TestSwapThroughAMetapoolUsesTheUnderlyingEntry(t *testing.T) {
	fake := newFakeCurve()
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, fake, sender)

	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

	_, err := client.Swap(context.Background(), SwapRequest{
		TokenIn:     testMIM,
		TokenOut:    testUSDC,
		AmountIn:    amountIn,
		SlippageBps: 100,
		Pool:        testMetaPool,
	})
	require.NoError(t, err)

	require.Len(t, sender.requests, 2)

	spender, _ := decodeApprove(t, sender.requests[0].Data)
	assert.Equal(t, testMetaPool, spender)

	name, args := decodeExchange(t, IndexInt128, sender.requests[1].Data)
	assert.Equal(t, "exchange_underlying", name)
	assert.Equal(t, int64(0), args[0].(*big.Int).Int64())
	assert.Equal(t, int64(1), args[1].(*big.Int).Int64())
}

func TestSwapThroughACryptoPoolUsesTheUint256Entry(t *testing.T) {
	fake := newFakeCurve()
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, fake, sender)

	amountIn := big.NewInt(2000_000000)

	_, err := client.Swap(context.Background(), SwapRequest{
		TokenIn:     testUSDC,
		TokenOut:    testWETH,
		AmountIn:    amountIn,
		SlippageBps: 100,
	})
	require.NoError(t, err)

	require.Len(t, sender.requests, 2)

	// the selector has to be the uint256 one, which is a different four bytes
	// from the int128 exchange of the same name
	name, args := decodeExchange(t, IndexUint256, sender.requests[1].Data)
	assert.Equal(t, "exchange", name)
	assert.Equal(t, int64(0), args[0].(*big.Int).Int64())
	assert.Equal(t, int64(1), args[1].(*big.Int).Int64())

	stableABI := PoolABI(IndexInt128)

	_, err = stableABI.MethodById(sender.requests[1].Data)
	assert.Error(t, err, "the call was encoded with the stableswap selector")
}

func TestSwapPinnedToAPoolThatDoesNotTradeThePair(t *testing.T) {
	fake := newFakeCurve()
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, fake, sender)

	_, err := client.Swap(context.Background(), SwapRequest{
		TokenIn:     testUSDC,
		TokenOut:    testWETH,
		AmountIn:    big.NewInt(1_000_000),
		SlippageBps: 50,
		Pool:        testStablePool,
	})
	assert.ErrorContains(t, err, "does not trade this pair")
	assert.Empty(t, sender.requests, "nothing may be sent when the route cannot be built")
}

func TestSwapRequiresASlippageBound(t *testing.T) {
	fake := newFakeCurve()
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, fake, sender)

	_, err := client.Swap(context.Background(), SwapRequest{
		TokenIn:  testUSDC,
		TokenOut: testUSDT,
		AmountIn: big.NewInt(1_000_000),
	})
	assert.ErrorContains(t, err, "slippageBps")
	assert.Empty(t, sender.requests)

	// an explicit floor is accepted instead
	_, err = client.Swap(context.Background(), SwapRequest{
		TokenIn:          testUSDC,
		TokenOut:         testUSDT,
		AmountIn:         big.NewInt(1_000_000),
		AmountOutMinimum: big.NewInt(999_000),
	})
	require.NoError(t, err)

	_, args := decodeExchange(t, IndexInt128, sender.requests[1].Data)
	assert.Equal(t, int64(999_000), args[3].(*big.Int).Int64())
}

func TestSwapSkipsAnApprovalThatIsAlreadyInPlace(t *testing.T) {
	fake := newFakeCurve()
	fake.allowances[testUSDC] = token.MaxUint256

	sender := &fakeSender{address: testUser}
	client := newTestClient(t, fake, sender)

	_, err := client.Swap(context.Background(), SwapRequest{
		TokenIn:     testUSDC,
		TokenOut:    testUSDT,
		AmountIn:    big.NewInt(1_000_000),
		SlippageBps: 50,
	})
	require.NoError(t, err)

	require.Len(t, sender.requests, 1)
	assert.Equal(t, testStablePool, *sender.requests[0].To)
}

// exchangeLog builds a TokenExchange log of one variant.
func exchangeLog(pool common.Address, topic common.Hash, words ...int64) *ethtypes.Log {
	data := make([]byte, 0, len(words)*32)
	for _, word := range words {
		data = append(data, common.BigToHash(big.NewInt(word)).Bytes()...)
	}

	return &ethtypes.Log{
		Address: pool,
		Topics:  []common.Hash{topic, common.BytesToHash(testUser.Bytes())},
		Data:    data,
	}
}

func TestExchangeAmountOutAcrossTheEventVariants(t *testing.T) {
	route := &Route{Pool: testStablePool, I: 0, J: 1}

	cases := []struct {
		name  string
		topic common.Hash
		words []int64
	}{
		{
			name:  "stableswap",
			topic: stablePoolABI.Events["TokenExchange"].ID,
			words: []int64{0, 1_000_000, 1, 1_001_000},
		},
		{
			name:  "stableswap underlying",
			topic: stablePoolABI.Events["TokenExchangeUnderlying"].ID,
			words: []int64{0, 1_000_000, 1, 1_001_000},
		},
		{
			name:  "cryptoswap",
			topic: cryptoPoolABI.Events["TokenExchange"].ID,
			words: []int64{0, 1_000_000, 1, 1_001_000},
		},
		{
			// the NG pools append the fee and the packed price scale, which the
			// word based decoding ignores
			name:  "cryptoswap ng",
			topic: cryptoNGEventABI.Events["TokenExchange"].ID,
			words: []int64{0, 1_000_000, 1, 1_001_000, 4_000_000, 123456},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			receipt := &ethtypes.Receipt{
				Logs: []*ethtypes.Log{exchangeLog(testStablePool, c.topic, c.words...)},
			}

			amountOut, err := ExchangeAmountOut(receipt, route)
			require.NoError(t, err)
			assert.Equal(t, int64(1_001_000), amountOut.Int64())
		})
	}
}

func TestExchangeAmountOutIgnoresTheWrongEvent(t *testing.T) {
	route := &Route{Pool: testStablePool, I: 0, J: 1}
	topic := stablePoolABI.Events["TokenExchange"].ID

	// another pool's swap in the same transaction
	other := &ethtypes.Receipt{
		Logs: []*ethtypes.Log{exchangeLog(testMetaPool, topic, 0, 1_000_000, 1, 500)},
	}

	_, err := ExchangeAmountOut(other, route)
	assert.ErrorContains(t, err, "no exchange event")

	// the right pool, but a swap into a different coin: bought_id is what
	// confirms the right event was found
	wrongCoin := &ethtypes.Receipt{
		Logs: []*ethtypes.Log{exchangeLog(testStablePool, topic, 0, 1_000_000, 2, 500)},
	}

	_, err = ExchangeAmountOut(wrongCoin, route)
	assert.ErrorContains(t, err, "no exchange event")

	// the pool's own transfer events are not swaps
	unrelated := &ethtypes.Receipt{
		Logs: []*ethtypes.Log{{
			Address: testStablePool,
			Topics:  []common.Hash{common.HexToHash("0xdeadbeef")},
			Data:    make([]byte, 128),
		}},
	}

	_, err = ExchangeAmountOut(unrelated, route)
	assert.ErrorContains(t, err, "no exchange event")

	// a truncated payload is reported rather than read past
	truncated := &ethtypes.Receipt{
		Logs: []*ethtypes.Log{exchangeLog(testStablePool, topic, 0, 1_000_000)},
	}

	_, err = ExchangeAmountOut(truncated, route)
	assert.ErrorContains(t, err, "carries only")
}

func TestSwapDecodesTheRealizedOutput(t *testing.T) {
	fake := newFakeCurve()

	sender := &fakeSender{address: testUser}
	client := newTestClient(t, fake, sender)

	amountIn := big.NewInt(1_000_000)

	quote, err := client.BestQuote(context.Background(), testUSDC, testUSDT, amountIn)
	require.NoError(t, err)

	// the pool fills one unit short of the quote
	filled := new(big.Int).Sub(quote.AmountOut, big.NewInt(1))

	sender.receipt = &ethtypes.Receipt{
		Logs: []*ethtypes.Log{exchangeLog(testStablePool,
			stablePoolABI.Events["TokenExchange"].ID,
			0, amountIn.Int64(), 1, filled.Int64())},
	}

	result, err := client.Swap(context.Background(), SwapRequest{
		TokenIn:     testUSDC,
		TokenOut:    testUSDT,
		AmountIn:    amountIn,
		SlippageBps: 50,
	})
	require.NoError(t, err)

	require.NotNil(t, result.AmountOut)
	assert.Equal(t, 0, result.AmountOut.Cmp(filled))
	assert.Positive(t, result.Slippage())
	assert.Less(t, result.Slippage(), 0.0001)
}

func TestSwapRejectsTheSameCoinTwice(t *testing.T) {
	fake := newFakeCurve()
	sender := &fakeSender{address: testUser}
	client := newTestClient(t, fake, sender)

	_, err := client.Swap(context.Background(), SwapRequest{
		TokenIn:     testUSDC,
		TokenOut:    testUSDC,
		AmountIn:    big.NewInt(1),
		SlippageBps: 50,
	})
	assert.ErrorContains(t, err, "the same")
	assert.Empty(t, sender.requests)
}
