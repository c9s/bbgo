package token

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/c9s/bbgo/pkg/chain/evm"
)

var testSpender = common.HexToAddress("0x2222222222222222222222222222222222222222")

type fakeTxSender struct {
	address  common.Address
	requests []evm.SendRequest
	err      error
}

func (s *fakeTxSender) Address() common.Address {
	return s.address
}

func (s *fakeTxSender) SendAndWait(_ context.Context, req evm.SendRequest) (*evm.TxResult, error) {
	s.requests = append(s.requests, req)

	if s.err != nil {
		return nil, s.err
	}

	return &evm.TxResult{Status: 1, GasUsed: 46000, GasCost: big.NewInt(1)}, nil
}

// decodeApprove reads back the spender and amount of an encoded approve call.
func decodeApprove(t *testing.T, data []byte) (common.Address, *big.Int) {
	t.Helper()

	erc20 := ERC20ABI()
	method, err := erc20.MethodById(data)
	require.NoError(t, err)
	require.Equal(t, "approve", method.Name)

	args, err := method.Inputs.Unpack(data[4:])
	require.NoError(t, err)

	return args[0].(common.Address), args[1].(*big.Int)
}

func newAllowanceFixture(t *testing.T, current *big.Int) (*fakeChain, *fakeTxSender, Token) {
	t.Helper()

	chain := newFakeChain()
	chain.tokens[testUSDT].allowances = map[allowanceKey]*big.Int{
		{owner: testOwner, spender: testSpender}: current,
	}

	registry := NewRegistry(chain, testMulticallAddress)

	usdt, err := registry.Lookup(context.Background(), testUSDT)
	require.NoError(t, err)

	return chain, &fakeTxSender{address: testOwner}, usdt
}

func TestEnsureAllowanceSufficient(t *testing.T) {
	chain, sender, usdt := newAllowanceFixture(t, big.NewInt(5_000_000))

	result, err := EnsureAllowance(
		context.Background(), chain, sender, usdt, testSpender, big.NewInt(2_000_000), AllowanceConfig{})

	assert.NoError(t, err)
	assert.Nil(t, result, "no approval is needed")
	assert.Empty(t, sender.requests)
}

func TestEnsureAllowanceExact(t *testing.T) {
	chain, sender, usdt := newAllowanceFixture(t, big.NewInt(0))

	amount := big.NewInt(2_000_000)

	result, err := EnsureAllowance(context.Background(), chain, sender, usdt, testSpender, amount, AllowanceConfig{})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, sender.requests, 1)
	assert.Equal(t, usdt.Address, *sender.requests[0].To)
	assert.Equal(t, "approve USDT", sender.requests[0].Label)

	spender, approved := decodeApprove(t, sender.requests[0].Data)
	assert.Equal(t, testSpender, spender)
	assert.Equal(t, amount, approved)
}

func TestEnsureAllowanceUnlimited(t *testing.T) {
	chain, sender, usdt := newAllowanceFixture(t, big.NewInt(0))

	_, err := EnsureAllowance(context.Background(), chain, sender, usdt, testSpender, big.NewInt(1),
		AllowanceConfig{Policy: AllowanceUnlimited})
	require.NoError(t, err)

	require.Len(t, sender.requests, 1)
	_, approved := decodeApprove(t, sender.requests[0].Data)
	assert.Equal(t, MaxUint256, approved)
}

func TestEnsureAllowanceResetsFirst(t *testing.T) {
	chain, sender, usdt := newAllowanceFixture(t, big.NewInt(1_000))

	amount := big.NewInt(2_000_000)

	_, err := EnsureAllowance(context.Background(), chain, sender, usdt, testSpender, amount,
		AllowanceConfig{ResetBeforeApprove: true})
	require.NoError(t, err)

	// tokens that refuse to change a non-zero allowance need the zero first
	require.Len(t, sender.requests, 2)

	_, first := decodeApprove(t, sender.requests[0].Data)
	assert.Equal(t, 0, first.Sign())

	_, second := decodeApprove(t, sender.requests[1].Data)
	assert.Equal(t, amount, second)
}

func TestEnsureAllowanceRejectsNonPositive(t *testing.T) {
	chain, sender, usdt := newAllowanceFixture(t, big.NewInt(0))

	_, err := EnsureAllowance(context.Background(), chain, sender, usdt, testSpender, big.NewInt(0), AllowanceConfig{})
	assert.Error(t, err)

	_, err = EnsureAllowance(context.Background(), chain, sender, usdt, testSpender, nil, AllowanceConfig{})
	assert.Error(t, err)

	assert.Empty(t, sender.requests)
}

func TestEnsureAllowancePropagatesSendError(t *testing.T) {
	chain, sender, usdt := newAllowanceFixture(t, big.NewInt(0))
	sender.err = errors.New("broadcast failed")

	_, err := EnsureAllowance(
		context.Background(), chain, sender, usdt, testSpender, big.NewInt(1), AllowanceConfig{})
	assert.ErrorContains(t, err, "broadcast failed")
}

func TestTransferCallData(t *testing.T) {
	data, err := TransferCallData(testSpender, big.NewInt(1_500_000))
	require.NoError(t, err)

	erc20 := ERC20ABI()
	_, err = erc20.MethodById(data)

	// transfer is not part of the minimal read ABI, so it has its own encoder
	assert.Error(t, err)
	assert.Len(t, data, 4+32+32)
}

func TestAllowanceConfigPolicyDefault(t *testing.T) {
	assert.Equal(t, AllowanceExact, AllowanceConfig{}.ResolvedPolicy())
	assert.Equal(t, AllowanceExact, AllowanceConfig{Policy: "nonsense"}.ResolvedPolicy())
	assert.Equal(t, AllowanceUnlimited, AllowanceConfig{Policy: "UNLIMITED"}.ResolvedPolicy())
}
