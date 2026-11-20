package curve

import (
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
	// Quote is what the pool promised before sending.
	Quote *Quote

	// AmountOutMinimum is the min_dy the pool enforced.
	AmountOutMinimum *big.Int

	// AmountOut is what actually came out, decoded from the pool's
	// TokenExchange event. It is nil when the event could not be found.
	AmountOut *big.Int

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

// Swap performs one exact-input swap through one pool: it quotes, derives
// min_dy, approves the pool, sends exchange or exchange_underlying, and decodes
// what actually came out.
//
// The approval goes to the pool, not to a router: on Curve the pool is the
// contract that moves the coins. Only ERC20 to ERC20 swaps are supported; the
// pools that accept native currency do it through a payable exchange with a
// use_eth flag, which this adapter does not wrap.
func (c *Client) Swap(ctx context.Context, req SwapRequest) (*SwapResult, error) {
	sender, err := c.requireSender()
	if err != nil {
		return nil, err
	}

	if req.AmountIn == nil || req.AmountIn.Sign() <= 0 {
		return nil, fmt.Errorf("the input amount must be positive")
	}

	if req.TokenIn == req.TokenOut {
		return nil, fmt.Errorf("the input and the output coin are the same")
	}

	if req.AmountOutMinimum == nil && (req.SlippageBps <= 0 || req.SlippageBps >= 10000) {
		return nil, fmt.Errorf(
			"slippageBps must be between 1 and 9999, or set amountOutMinimum: " +
				"a swap without a minimum output can be sandwiched for its whole value")
	}

	var quote *Quote

	if req.Pool == (common.Address{}) {
		quote, err = c.BestQuote(ctx, req.TokenIn, req.TokenOut, req.AmountIn)
	} else {
		var route *Route

		route, err = c.RouteForPool(ctx, req.Pool, req.TokenIn, req.TokenOut)
		if err == nil {
			quote, err = c.Quote(ctx, route, req.AmountIn)
		}
	}

	if err != nil {
		return nil, err
	}

	minimumOut := req.AmountOutMinimum
	if minimumOut == nil {
		minimumOut = quote.AmountOutMinimum(req.SlippageBps)
	}

	if minimumOut.Sign() <= 0 {
		return nil, fmt.Errorf("the minimum output works out to zero, refusing to swap")
	}

	route := quote.Route

	if err := c.ensureAllowance(ctx, route.Pool, req.TokenIn, req.AmountIn); err != nil {
		return nil, err
	}

	data, err := PoolABI(route.Index).Pack(route.Method(),
		big.NewInt(route.I), big.NewInt(route.J), req.AmountIn, minimumOut)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s: %w", route.Method(), err)
	}

	pool := route.Pool

	txResult, err := sender.SendAndWait(ctx, evm.SendRequest{
		To:    &pool,
		Data:  data,
		Label: "curve " + route.Method(),
	})

	result := &SwapResult{
		Quote:            quote,
		AmountOutMinimum: minimumOut,
		Tx:               txResult,
	}

	if txResult != nil && txResult.Receipt != nil {
		if amountOut, decodeErr := ExchangeAmountOut(txResult.Receipt, route); decodeErr == nil {
			result.AmountOut = amountOut
		} else {
			log.WithError(decodeErr).Warnf("curve: could not decode the realized output of %s",
				txResult.Hash.Hex())
		}
	}

	if err != nil {
		return result, err
	}

	log.Infof("curve: %s", result)

	return result, nil
}

// exchangeEvents are the TokenExchange variants a Curve pool can emit, keyed by
// topic. The shapes differ between the pool families and between their
// versions: the coin indices are int128 on a StableSwap pool and uint256 on a
// CryptoSwap one, and the NG crypto pools append the realized fee and the
// packed price scale. Each difference changes the topic of an otherwise
// identical event, so recognizing a Curve swap means knowing all of them.
var exchangeEvents = map[common.Hash]string{
	stablePoolABI.Events["TokenExchange"].ID:           "stableswap TokenExchange",
	stablePoolABI.Events["TokenExchangeUnderlying"].ID: "stableswap TokenExchangeUnderlying",
	cryptoPoolABI.Events["TokenExchange"].ID:           "cryptoswap TokenExchange",
	cryptoPoolABI.Events["TokenExchangeUnderlying"].ID: "cryptoswap TokenExchangeUnderlying",
	cryptoNGEventABI.Events["TokenExchange"].ID:        "cryptoswap-ng TokenExchange",
}

// ExchangeAmountOut decodes the realized output of a swap from the pool's
// TokenExchange event.
//
// The payload is read as words rather than through one event's ABI on purpose.
// Every variant lays out sold_id, tokens_sold, bought_id and tokens_bought as
// its first four words, whatever the index type, and only the trailing fields
// differ; reading the words keeps a pool version this package has not seen
// decodable, and the bought_id check is what confirms the right event was
// found.
func ExchangeAmountOut(receipt *ethtypes.Receipt, route *Route) (*big.Int, error) {
	const wordSize = 32

	if route == nil {
		return nil, fmt.Errorf("a route is required")
	}

	for _, entry := range receipt.Logs {
		if entry.Address != route.Pool {
			continue
		}

		if len(entry.Topics) == 0 {
			continue
		}

		if _, ok := exchangeEvents[entry.Topics[0]]; !ok {
			continue
		}

		if len(entry.Data) < 4*wordSize {
			return nil, fmt.Errorf("the exchange event of %s carries only %d bytes",
				route.Pool.Hex(), len(entry.Data))
		}

		boughtID := new(big.Int).SetBytes(entry.Data[2*wordSize : 3*wordSize])
		if boughtID.Int64() != route.J {
			continue
		}

		return new(big.Int).SetBytes(entry.Data[3*wordSize : 4*wordSize]), nil
	}

	return nil, fmt.Errorf("no exchange event for coin %d from the pool %s is present in the receipt",
		route.J, route.Pool.Hex())
}

// ensureAllowance approves the pool to move amount of the input coin, following
// the session's approval policy.
func (c *Client) ensureAllowance(
	ctx context.Context, pool, asset common.Address, amount *big.Int,
) error {
	sender, err := c.requireSender()
	if err != nil {
		return err
	}

	t, err := c.Token(ctx, asset)
	if err != nil {
		return err
	}

	result, err := token.EnsureAllowance(ctx, c.caller, sender, t, pool, amount, c.allowance)
	if err != nil {
		return err
	}

	if result != nil {
		log.Infof("curve: approved the pool %s to spend %s %s, %s",
			pool.Hex(), t.FormatRaw(amount), t.Symbol, result)
	}

	return nil
}
