package v3

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	log "github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/chain/evm"
	"github.com/c9s/bbgo/pkg/chain/evm/token"
)

// SwapResult is the outcome of one swap.
type SwapResult struct {
	// Quote is what the quoter promised before sending.
	Quote *Quote

	// AmountOutMinimum is the bound the router enforced.
	AmountOutMinimum *big.Int

	// AmountOut is what actually came out, decoded from the pool's Swap event.
	// It is nil when the event could not be found.
	AmountOut *big.Int

	Pool common.Address

	Tx *evm.TxResult
}

// Slippage returns the shortfall of the realized output against the quote, as a
// fraction. A negative value means the swap filled better than quoted.
func (r *SwapResult) Slippage() float64 {
	if r.Quote == nil || r.Quote.AmountOut == nil || r.Quote.AmountOut.Sign() == 0 || r.AmountOut == nil {
		return 0
	}

	shortfall := new(big.Float).Sub(
		new(big.Float).SetInt(r.Quote.AmountOut), new(big.Float).SetInt(r.AmountOut))

	ratio, _ := new(big.Float).Quo(shortfall, new(big.Float).SetInt(r.Quote.AmountOut)).Float64()

	return ratio
}

func (r *SwapResult) String() string {
	out := "unknown"
	if r.AmountOut != nil {
		out = r.AmountOut.String()
	}

	return fmt.Sprintf("swapped %s for %s (quoted %s, floor %s, slippage %.4f%%)",
		r.Quote.AmountIn, out, r.Quote.AmountOut, r.AmountOutMinimum, r.Slippage()*100)
}

// Swap performs one exact-input single-hop swap: it quotes, derives the minimum
// output, approves the router if needed, sends the swap and decodes what
// actually came out.
//
// Only ERC20 to ERC20 swaps are supported. Swapping the native currency means
// wrapping it first, which the router does through a multicall this adapter
// does not wrap yet.
func (c *Client) Swap(ctx context.Context, req SwapRequest) (*SwapResult, error) {
	sender, err := c.requireSender()
	if err != nil {
		return nil, err
	}

	if req.AmountIn == nil || req.AmountIn.Sign() <= 0 {
		return nil, fmt.Errorf("the input amount must be positive")
	}

	if req.TokenIn == req.TokenOut {
		return nil, fmt.Errorf("the input and the output token are the same")
	}

	if req.AmountOutMinimum == nil && (req.SlippageBps <= 0 || req.SlippageBps >= 10000) {
		return nil, fmt.Errorf(
			"slippageBps must be between 1 and 9999, or set amountOutMinimum: " +
				"a swap without a minimum output can be sandwiched for its whole value")
	}

	var quote *Quote
	if req.Fee == 0 {
		quote, err = c.BestQuote(ctx, req.TokenIn, req.TokenOut, req.AmountIn)
	} else {
		quote, err = c.Quote(ctx, req.TokenIn, req.TokenOut, req.Fee, req.AmountIn)
	}

	if err != nil {
		return nil, err
	}

	if quote.AmountOut.Sign() == 0 {
		return nil, fmt.Errorf("%w: the quote for this size is zero", ErrNoPool)
	}

	minimumOut := req.AmountOutMinimum
	if minimumOut == nil {
		minimumOut = quote.AmountOutMinimum(req.SlippageBps)
	}

	if minimumOut.Sign() <= 0 {
		return nil, fmt.Errorf("the minimum output works out to zero, refusing to swap")
	}

	recipient := req.Recipient
	if recipient == (common.Address{}) {
		recipient = sender.Address()
	}

	pool, err := c.PoolAddress(ctx, req.TokenIn, req.TokenOut, quote.Fee)
	if err != nil {
		return nil, err
	}

	if err := c.ensureAllowance(ctx, req.TokenIn, req.AmountIn); err != nil {
		return nil, err
	}

	params := exactInputSingleParams{
		TokenIn:           req.TokenIn,
		TokenOut:          req.TokenOut,
		Fee:               new(big.Int).SetUint64(uint64(quote.Fee)),
		Recipient:         recipient,
		AmountIn:          req.AmountIn,
		AmountOutMinimum:  minimumOut,
		SqrtPriceLimitX96: new(big.Int),
	}

	data, err := routerABI.Pack("exactInputSingle", params)
	if err != nil {
		return nil, fmt.Errorf("failed to encode exactInputSingle: %w", err)
	}

	router := c.deployment.SwapRouter

	txResult, err := sender.SendAndWait(ctx, evm.SendRequest{
		To:    &router,
		Data:  data,
		Label: "uniswap v3 swap",
	})

	result := &SwapResult{
		Quote:            quote,
		AmountOutMinimum: minimumOut,
		Pool:             pool,
		Tx:               txResult,
	}

	if txResult != nil && txResult.Receipt != nil {
		if amountOut, decodeErr := SwapAmountOut(
			txResult.Receipt, pool, req.TokenIn, req.TokenOut,
		); decodeErr == nil {
			result.AmountOut = amountOut
		} else {
			log.WithError(decodeErr).Warnf("uniswap v3: could not decode the realized output of %s",
				txResult.Hash.Hex())
		}
	}

	if err != nil {
		return result, err
	}

	log.Infof("uniswap v3: %s", result)

	return result, nil
}

// SwapAmountOut decodes the realized output of a swap from the pool's Swap
// event.
//
// The event reports the pool's own deltas: positive means the token went into
// the pool, negative means it left. The output is therefore the negated amount
// on the side of tokenOut. Which side that is follows from the pair ordering,
// since a v3 pool always sorts token0 below token1.
func SwapAmountOut(
	receipt *ethtypes.Receipt, pool common.Address, tokenIn, tokenOut common.Address,
) (*big.Int, error) {
	event, ok := poolABI.Events["Swap"]
	if !ok {
		return nil, fmt.Errorf("the pool abi has no Swap event")
	}

	for _, entry := range receipt.Logs {
		if entry.Address != pool {
			continue
		}

		if len(entry.Topics) == 0 || entry.Topics[0] != event.ID {
			continue
		}

		values, err := event.Inputs.NonIndexed().Unpack(entry.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode the Swap event: %w", err)
		}

		amount0, ok := values[0].(*big.Int)
		if !ok {
			return nil, fmt.Errorf("unexpected amount0 type %T", values[0])
		}

		amount1, ok := values[1].(*big.Int)
		if !ok {
			return nil, fmt.Errorf("unexpected amount1 type %T", values[1])
		}

		// token0 is the lower address, so the output side follows from the
		// ordering of the pair
		outIsToken1 := bytes.Compare(tokenIn.Bytes(), tokenOut.Bytes()) < 0

		out := amount0
		if outIsToken1 {
			out = amount1
		}

		return new(big.Int).Neg(out), nil
	}

	return nil, fmt.Errorf("no Swap event from the pool %s is present in the receipt", pool.Hex())
}

// ensureAllowance approves the router to move amount of the input token,
// following the session's approval policy.
func (c *Client) ensureAllowance(ctx context.Context, asset common.Address, amount *big.Int) error {
	sender, err := c.requireSender()
	if err != nil {
		return err
	}

	t, err := c.Token(ctx, asset)
	if err != nil {
		return err
	}

	result, err := token.EnsureAllowance(
		ctx, c.caller, sender, t, c.deployment.SwapRouter, amount, c.allowance)
	if err != nil {
		return err
	}

	if result != nil {
		log.Infof("uniswap v3: approved the router to spend %s %s, %s",
			t.FormatRaw(amount), t.Symbol, result)
	}

	return nil
}
