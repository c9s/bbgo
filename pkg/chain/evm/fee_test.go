package evm

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

type fakeFeeEstimator struct {
	baseFee  *big.Int
	rewards  [][]*big.Int
	gasPrice *big.Int

	headerErr  error
	historyErr error
	priceErr   error

	percentiles []float64
}

func (f *fakeFeeEstimator) HeaderByNumber(_ context.Context, _ *big.Int) (*ethtypes.Header, error) {
	if f.headerErr != nil {
		return nil, f.headerErr
	}

	return &ethtypes.Header{Number: big.NewInt(100), BaseFee: f.baseFee}, nil
}

func (f *fakeFeeEstimator) FeeHistory(
	_ context.Context, _ uint64, _ *big.Int, rewardPercentiles []float64,
) (*ethereum.FeeHistory, error) {
	if f.historyErr != nil {
		return nil, f.historyErr
	}

	f.percentiles = rewardPercentiles

	return &ethereum.FeeHistory{Reward: f.rewards}, nil
}

func (f *fakeFeeEstimator) SuggestGasPrice(_ context.Context) (*big.Int, error) {
	if f.priceErr != nil {
		return nil, f.priceErr
	}

	return f.gasPrice, nil
}

func rewardsOf(values ...int64) [][]*big.Int {
	out := make([][]*big.Int, 0, len(values))
	for _, v := range values {
		out = append(out, []*big.Int{gwei(v)})
	}

	return out
}

func TestEstimateFeeEIP1559(t *testing.T) {
	ctx := context.Background()
	chain, _ := LookupChain("ethereum")

	estimator := &fakeFeeEstimator{
		baseFee: gwei(10),
		rewards: rewardsOf(1, 2, 3),
	}

	fee, err := EstimateFee(ctx, estimator, chain, FeeConfig{})
	assert.NoError(t, err)
	assert.False(t, fee.Legacy)

	// the median of the percentile rewards
	assert.Equal(t, gwei(2), fee.MaxPriorityFeePerGas)

	// baseFee * 2 + tip
	assert.Equal(t, gwei(22), fee.MaxFeePerGas)
	assert.Equal(t, []float64{DefaultTipPercentile}, estimator.percentiles)
}

func TestEstimateFeeAppliesTheChainTipFloor(t *testing.T) {
	ctx := context.Background()
	chain, _ := LookupChain("polygon")

	// Polygon rejects anything below ~25 gwei of priority fee
	estimator := &fakeFeeEstimator{baseFee: gwei(50), rewards: rewardsOf(1)}

	fee, err := EstimateFee(ctx, estimator, chain, FeeConfig{})
	assert.NoError(t, err)
	assert.Equal(t, chain.MinPriorityFee, fee.MaxPriorityFeePerGas)
	assert.Equal(t, gwei(125), fee.MaxFeePerGas)
}

func TestEstimateFeeWithoutRewards(t *testing.T) {
	ctx := context.Background()
	chain, _ := LookupChain("ethereum")

	estimator := &fakeFeeEstimator{baseFee: gwei(10)}

	fee, err := EstimateFee(ctx, estimator, chain, FeeConfig{})
	assert.NoError(t, err)
	assert.Equal(t, chain.MinPriorityFee, fee.MaxPriorityFeePerGas)
}

func TestEstimateFeeCap(t *testing.T) {
	ctx := context.Background()
	chain, _ := LookupChain("ethereum")
	estimator := &fakeFeeEstimator{baseFee: gwei(10), rewards: rewardsOf(2)}

	// the cap clamps maxFeePerGas
	fee, err := EstimateFee(ctx, estimator, chain, FeeConfig{MaxFeePerGasGwei: 15})
	assert.NoError(t, err)
	assert.Equal(t, gwei(15), fee.MaxFeePerGas)

	// a cap below the current base fee can never be included, so it fails
	_, err = EstimateFee(ctx, estimator, chain, FeeConfig{MaxFeePerGasGwei: 5})
	assert.ErrorIs(t, err, ErrFeeCapExceeded)
}

func TestEstimateFeeLegacy(t *testing.T) {
	ctx := context.Background()
	chain, _ := LookupChain("bsc")
	estimator := &fakeFeeEstimator{gasPrice: gwei(3)}

	fee, err := EstimateFee(ctx, estimator, chain, FeeConfig{})
	assert.NoError(t, err)
	assert.True(t, fee.Legacy)
	assert.Equal(t, gwei(3), fee.GasPrice)
	assert.Equal(t, gwei(3), fee.Cap())

	_, err = EstimateFee(ctx, estimator, chain, FeeConfig{MaxFeePerGasGwei: 1})
	assert.ErrorIs(t, err, ErrFeeCapExceeded)
}

// an EIP-1559 chain whose head carries no base fee falls back instead of failing
func TestEstimateFeeFallsBackWithoutBaseFee(t *testing.T) {
	ctx := context.Background()
	chain, _ := LookupChain("ethereum")
	estimator := &fakeFeeEstimator{gasPrice: gwei(7)}

	fee, err := EstimateFee(ctx, estimator, chain, FeeConfig{})
	assert.NoError(t, err)
	assert.True(t, fee.Legacy)
	assert.Equal(t, gwei(7), fee.GasPrice)
}

func TestEstimateFeeErrors(t *testing.T) {
	ctx := context.Background()
	chain, _ := LookupChain("ethereum")

	_, err := EstimateFee(ctx, &fakeFeeEstimator{headerErr: errors.New("boom")}, chain, FeeConfig{})
	assert.ErrorContains(t, err, "boom")

	_, err = EstimateFee(ctx, &fakeFeeEstimator{baseFee: gwei(1), historyErr: errors.New("nope")}, chain, FeeConfig{})
	assert.ErrorContains(t, err, "nope")
}

func TestFeeBump(t *testing.T) {
	fee := &Fee{MaxFeePerGas: gwei(100), MaxPriorityFeePerGas: gwei(10)}

	bumped, err := fee.Bump(20, nil)
	assert.NoError(t, err)
	assert.Equal(t, gwei(120), bumped.MaxFeePerGas)
	assert.Equal(t, gwei(12), bumped.MaxPriorityFeePerGas)

	// a bump below the replacement threshold is raised to it
	bumped, err = fee.Bump(1, nil)
	assert.NoError(t, err)
	assert.Equal(t, gwei(115), bumped.MaxFeePerGas)

	_, err = fee.Bump(20, gwei(110))
	assert.ErrorIs(t, err, ErrFeeCapExceeded)

	legacy := &Fee{Legacy: true, GasPrice: gwei(10)}
	bumped, err = legacy.Bump(50, nil)
	assert.NoError(t, err)
	assert.True(t, bumped.Legacy)
	assert.Equal(t, gwei(15), bumped.GasPrice)
	assert.Nil(t, bumped.MaxFeePerGas)
}

func TestFeeConfigDefaults(t *testing.T) {
	cfg := FeeConfig{}.WithDefaults()
	assert.Equal(t, DefaultTipPercentile, cfg.TipPercentile)
	assert.Equal(t, uint64(DefaultFeeHistoryBlocks), cfg.HistoryBlocks)
	assert.Equal(t, DefaultBaseFeeMultiplier, cfg.BaseFeeMultiplier)
	assert.Equal(t, DefaultBumpPercent, cfg.BumpPercent)
	assert.Equal(t, DefaultMaxBumpAttempts, cfg.MaxBumpAttempts)
	assert.Nil(t, cfg.MaxFeePerGasWei())

	cfg = FeeConfig{BumpPercent: 5, MaxFeePerGasGwei: 1.5}.WithDefaults()
	assert.Equal(t, DefaultBumpPercent, cfg.BumpPercent, "a bump below the replacement threshold is raised")
	assert.Equal(t, big.NewInt(1_500_000_000), cfg.MaxFeePerGasWei())
}
