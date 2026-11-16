package evm

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/ethereum/go-ethereum"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// ErrFeeCapExceeded is returned when the required fee is above the configured
// absolute ceiling. The caller stops bumping instead of paying through the roof.
var ErrFeeCapExceeded = errors.New("the required gas fee exceeds the configured cap")

const (
	DefaultTipPercentile     = 50.0
	DefaultFeeHistoryBlocks  = 10
	DefaultBaseFeeMultiplier = 2

	// DefaultBumpPercent must stay above the 10% that geth requires to accept a
	// replacement transaction.
	DefaultBumpPercent = 15

	DefaultMaxBumpAttempts = 3
)

// FeeConfig controls the fee estimation and the replacement (bump) policy.
type FeeConfig struct {
	// TipPercentile is the eth_feeHistory reward percentile used as the
	// priority fee. 50 means the median of recent blocks.
	TipPercentile float64 `json:"tipPercentile,omitempty" yaml:"tipPercentile,omitempty"`

	// HistoryBlocks is how many recent blocks eth_feeHistory looks at.
	HistoryBlocks uint64 `json:"historyBlocks,omitempty" yaml:"historyBlocks,omitempty"`

	// BaseFeeMultiplier is the headroom for base fee growth while the
	// transaction is pending. maxFeePerGas = baseFee*multiplier + tip.
	BaseFeeMultiplier int `json:"baseFeeMultiplier,omitempty" yaml:"baseFeeMultiplier,omitempty"`

	// MaxFeePerGasGwei is the absolute ceiling for maxFeePerGas (or gasPrice on
	// a legacy chain), in gwei. Zero means no ceiling, which is not recommended.
	MaxFeePerGasGwei float64 `json:"maxFeePerGasGwei,omitempty" yaml:"maxFeePerGasGwei,omitempty"`

	// BumpPercent is the fee increase of each replacement transaction.
	BumpPercent int `json:"bumpPercent,omitempty" yaml:"bumpPercent,omitempty"`

	// MaxBumpAttempts bounds how many times one transaction is replaced.
	MaxBumpAttempts int `json:"maxBumpAttempts,omitempty" yaml:"maxBumpAttempts,omitempty"`
}

func (c FeeConfig) WithDefaults() FeeConfig {
	if c.TipPercentile <= 0 || c.TipPercentile > 100 {
		c.TipPercentile = DefaultTipPercentile
	}

	if c.HistoryBlocks == 0 {
		c.HistoryBlocks = DefaultFeeHistoryBlocks
	}

	if c.BaseFeeMultiplier <= 0 {
		c.BaseFeeMultiplier = DefaultBaseFeeMultiplier
	}

	if c.BumpPercent < DefaultBumpPercent {
		c.BumpPercent = DefaultBumpPercent
	}

	if c.MaxBumpAttempts < 0 {
		c.MaxBumpAttempts = 0
	} else if c.MaxBumpAttempts == 0 {
		c.MaxBumpAttempts = DefaultMaxBumpAttempts
	}

	return c
}

// MaxFeePerGasWei converts the configured ceiling to wei. It returns nil when
// no ceiling is configured.
func (c FeeConfig) MaxFeePerGasWei() *big.Int {
	if c.MaxFeePerGasGwei <= 0 {
		return nil
	}

	wei := new(big.Float).Mul(big.NewFloat(c.MaxFeePerGasGwei), big.NewFloat(1e9))
	out, _ := wei.Int(nil)

	return out
}

// Fee is a concrete fee decision for one transaction, in one of the two shapes
// the supported chains use.
type Fee struct {
	// Legacy selects the gasPrice shape instead of the EIP-1559 one.
	Legacy bool

	GasPrice *big.Int

	MaxFeePerGas         *big.Int
	MaxPriorityFeePerGas *big.Int
}

func (f *Fee) String() string {
	if f == nil {
		return "<nil>"
	}

	if f.Legacy {
		return fmt.Sprintf("gasPrice=%s gwei", formatGwei(f.GasPrice))
	}

	return fmt.Sprintf("maxFee=%s gwei tip=%s gwei", formatGwei(f.MaxFeePerGas), formatGwei(f.MaxPriorityFeePerGas))
}

func formatGwei(wei *big.Int) string {
	if wei == nil {
		return "0"
	}

	gwei := new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(1e9))

	return gwei.Text('f', 4)
}

// Cap returns the highest price per gas this fee can pay, which is the field
// the configured ceiling applies to.
func (f *Fee) Cap() *big.Int {
	if f.Legacy {
		return f.GasPrice
	}

	return f.MaxFeePerGas
}

// Bump returns a replacement fee increased by percent, and ErrFeeCapExceeded
// when that would go above cap.
func (f *Fee) Bump(percent int, feeCap *big.Int) (*Fee, error) {
	if percent < DefaultBumpPercent {
		percent = DefaultBumpPercent
	}

	scale := func(v *big.Int) *big.Int {
		if v == nil {
			return nil
		}

		out := new(big.Int).Mul(v, big.NewInt(int64(100+percent)))

		return out.Div(out, big.NewInt(100))
	}

	bumped := &Fee{
		Legacy:               f.Legacy,
		GasPrice:             scale(f.GasPrice),
		MaxFeePerGas:         scale(f.MaxFeePerGas),
		MaxPriorityFeePerGas: scale(f.MaxPriorityFeePerGas),
	}

	if feeCap != nil && feeCap.Sign() > 0 && bumped.Cap().Cmp(feeCap) > 0 {
		return nil, fmt.Errorf("%w: bumping to %s gwei is above the cap of %s gwei",
			ErrFeeCapExceeded, formatGwei(bumped.Cap()), formatGwei(feeCap))
	}

	return bumped, nil
}

// FeeEstimator is the subset of the client used for fee estimation.
type FeeEstimator interface {
	HeaderByNumber(ctx context.Context, number *big.Int) (*ethtypes.Header, error)
	FeeHistory(
		ctx context.Context, blockCount uint64, lastBlock *big.Int, rewardPercentiles []float64,
	) (*ethereum.FeeHistory, error)
	SuggestGasPrice(ctx context.Context) (*big.Int, error)
}

// EstimateFee prices a transaction for the given chain.
//
// On an EIP-1559 chain the priority fee comes from the eth_feeHistory
// percentile, floored at Chain.MinPriorityFee (Polygon rejects anything below
// ~25 gwei), and maxFeePerGas leaves room for the base fee to grow while the
// transaction is pending. On a legacy chain, which is how BSC is priced here,
// the node's suggestion is used directly.
func EstimateFee(ctx context.Context, estimator FeeEstimator, chain *Chain, cfg FeeConfig) (*Fee, error) {
	cfg = cfg.WithDefaults()
	feeCap := cfg.MaxFeePerGasWei()

	if chain.FeeMarket == FeeMarketLegacy {
		return estimateLegacyFee(ctx, estimator, feeCap)
	}

	header, err := estimator.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to read the latest header for fee estimation: %w", err)
	}

	// a chain configured as EIP-1559 whose head has no base fee is a
	// misconfiguration or a pre-London node: fall back rather than fail
	if header.BaseFee == nil {
		return estimateLegacyFee(ctx, estimator, feeCap)
	}

	tip, err := estimateTip(ctx, estimator, chain, cfg)
	if err != nil {
		return nil, err
	}

	maxFee := new(big.Int).Mul(header.BaseFee, big.NewInt(int64(cfg.BaseFeeMultiplier)))
	maxFee.Add(maxFee, tip)

	if feeCap != nil && feeCap.Sign() > 0 && maxFee.Cmp(feeCap) > 0 {
		// the ceiling must still cover the current base fee, otherwise the
		// transaction can never be included
		if header.BaseFee.Cmp(feeCap) >= 0 {
			return nil, fmt.Errorf("%w: the base fee is %s gwei but the cap is %s gwei",
				ErrFeeCapExceeded, formatGwei(header.BaseFee), formatGwei(feeCap))
		}

		maxFee = new(big.Int).Set(feeCap)

		if tip.Cmp(maxFee) > 0 {
			tip = new(big.Int).Set(maxFee)
		}
	}

	return &Fee{MaxFeePerGas: maxFee, MaxPriorityFeePerGas: tip}, nil
}

func estimateLegacyFee(ctx context.Context, estimator FeeEstimator, feeCap *big.Int) (*Fee, error) {
	gasPrice, err := estimator.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read the suggested gas price: %w", err)
	}

	if feeCap != nil && feeCap.Sign() > 0 && gasPrice.Cmp(feeCap) > 0 {
		return nil, fmt.Errorf("%w: the suggested gas price is %s gwei but the cap is %s gwei",
			ErrFeeCapExceeded, formatGwei(gasPrice), formatGwei(feeCap))
	}

	return &Fee{Legacy: true, GasPrice: gasPrice}, nil
}

func estimateTip(ctx context.Context, estimator FeeEstimator, chain *Chain, cfg FeeConfig) (*big.Int, error) {
	history, err := estimator.FeeHistory(ctx, cfg.HistoryBlocks, nil, []float64{cfg.TipPercentile})
	if err != nil {
		return nil, fmt.Errorf("failed to read the fee history: %w", err)
	}

	rewards := make([]*big.Int, 0, len(history.Reward))
	for _, blockRewards := range history.Reward {
		for _, reward := range blockRewards {
			if reward != nil && reward.Sign() > 0 {
				rewards = append(rewards, reward)
			}
		}
	}

	tip := new(big.Int)
	if len(rewards) > 0 {
		sort.Slice(rewards, func(i, j int) bool { return rewards[i].Cmp(rewards[j]) < 0 })
		tip.Set(rewards[len(rewards)/2])
	}

	if chain.MinPriorityFee != nil && tip.Cmp(chain.MinPriorityFee) < 0 {
		tip.Set(chain.MinPriorityFee)
	}

	return tip, nil
}
