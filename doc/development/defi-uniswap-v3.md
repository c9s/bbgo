# Uniswap v3 Adapter

`pkg/defi/uniswap/v3` covers Uniswap v3 and the forks that kept its
interfaces, PancakeSwap v3 among them. It is built on the chain layer
(`doc/development/defi-chain-layer.md`) and the transaction layer.

```go
dex, err := v3.NewClient(ctx, session.EVMChain(), session.Client(), v3.Options{
    Deployment: "pancakeswap",        // empty means the chain default
    Tokens:     session.TokenRegistry(),
    Allowance:  session.AllowanceConfig(),
    Sender:     session.Sender(),     // omit for a read-only client
})
```

## Two decisions that avoid known traps

**Pool addresses come from `Factory.getPool`, not from the CREATE2 derivation.**
The derivation needs the deployment's `POOL_INIT_CODE_HASH`, which differs
between Uniswap and its forks; getting it wrong produces a plausible-looking
address with no code. Asking the factory costs one call and cannot be wrong.

**`QuoterV2` is declared `view` in this package's ABI** although it is
`nonpayable` on chain. It writes nothing — Uniswap left it non-view so the
multi-hop version can revert to return data — so declaring it `view` lets it be
called with `eth_call`, which is the documented way to quote. Without that,
abigen-style bindings generate a transactor and the quote becomes a transaction.

## Deployments

`deployments.go` holds the factory, quoter and router per chain, the default
first. `NewClient` verifies that each one has code on chain, so a wrong address
fails at construction instead of as an empty quote later.

| chain | deployments |
|---|---|
| ethereum | uniswap |
| polygon | uniswap |
| bsc | **pancakeswap** (default, deeper), uniswap |

`Options.Override` points the client at an unlisted fork.

The fee ladders are not the same: Uniswap uses 0.01% / 0.05% / **0.3%** / 1%,
PancakeSwap replaced the 0.3% tier with **0.25%**. A hardcoded `3000` finds no
pool on Pancake, which is why the ladder belongs to the deployment.

## Prices and sizes

- `Pool.Price(decimals0, decimals1)` converts `sqrtPriceX96` into a human price.
  The raw ratio is `(sqrtPriceX96 / 2^96)^2` in base units, and the token
  decimals then scale it by `10^(decimals0-decimals1)`. **Skipping that
  adjustment is the classic v3 bug**: a WBTC(8)/USDC(6) pool reads 1e-12 off.
- `Quote.Price` is the realized price of a given size, fee and impact included.
  It is the number to trade on; the spot price is not.
- `BestQuote` quotes every tier and takes the largest output. That is not the
  same as the deepest pool: a thin tier can be priced better for a small size,
  and a deep tier can still be the only one that absorbs a large one. Each tier
  is a separate `eth_call` because the quoter reverts for a tier with no pool,
  which would abort a batch.

Everything else is batched through Multicall3: `FindPools` probes the whole fee
ladder in one call and then reads the pools that exist in a second one.

## Swapping

```go
result, err := dex.Swap(ctx, v3.SwapRequest{
    TokenIn:     usdt.Address,
    TokenOut:    wbnb.Address,
    AmountIn:    amountIn,       // raw base units
    SlippageBps: 50,             // required unless AmountOutMinimum is set
})
```

`Swap` quotes, derives the minimum output, approves the router under the
session's allowance policy, sends `exactInputSingle` and then decodes what
actually came out of the pool's `Swap` event, so `SwapResult.AmountOut` is the
realized fill and `SwapResult.Slippage()` is the real shortfall against the
quote.

- A request with neither `SlippageBps` nor `AmountOutMinimum` is **rejected**: a
  swap with no minimum output can be sandwiched for its whole value.
- `Fee: 0` asks the client to pick the best tier.
- Only ERC20 to ERC20 swaps are supported. Swapping the native currency needs
  the router's wrapping multicall, which this adapter does not wrap yet.
- SwapRouter02 has no deadline field, unlike the original SwapRouter, so the
  only time bound is the transaction layer's own timeout.

`SwapAmountOut` decodes the realized output from a receipt. The event reports
the pool's own deltas, so the output is the negated amount on the `tokenOut`
side; which side that is follows from the pair ordering, since a v3 pool always
sorts `token0` below `token1`.

## CLI

```bash
bbgo uniswap pools --session bsc --base WBNB --quote USDT [--dex uniswap]
bbgo uniswap quote --session bsc --from WBNB --to USDT --amount 10
bbgo uniswap swap  --session bsc --from USDT --to WBNB --amount 1000 --slippage 50 [--fee 500] [--confirm]
```

`swap` prints the tier, the quote and the floor, and only sends with
`--confirm`. Its plan is a real check rather than a guess: the quote is an
`eth_call` against the quoter, so an unroutable pair or an unusable size fails
the command.

`quote` prints every tier, which makes the size dependence visible. On BSC, 10
WBNB through PancakeSwap:

```
     FEE                     OUTPUT              PRICE      TICKS
   0.01%    6882.356119491663567642       688.23561195          2
   0.05%    6880.363610503306566996       688.03636105          1
   0.25%    6837.298572238516500403       683.72985722          4
   1.00%    2534.444586680500847378       253.44445867         41
```

The 1% tier is so thin that this size crosses 41 initialized ticks and loses
63%. A single spot price cannot express that, which is why the `Quoter` is the
execution path and `slot0` is only a reference.

## Testing

`go test ./pkg/defi/uniswap/v3` runs on a fake deployment that answers the
factory, pool, quoter and ERC20 reads through Multicall3, plus a fake sender
that records the encoded router calls, so the swap parameters and the
`Swap` event decoding are asserted without a node. The fake's 0.3% pool carries
a PancakeSwap-style wide `feeProtocol`, which pins down the `slot0` decoding.

The integration test validates the deployment addresses and every ABI against
live pools, and is skipped unless you opt in:

```bash
TEST_EVM_CHAIN=bsc TEST_EVM_RPC_ENDPOINTS=https://... go test ./pkg/defi/uniswap/v3 -run TestIntegration -v
```

It runs every configured deployment of the chain and asserts the shape: the fee
tier is in the deployment's ladder, `token0` is the lower address, a small quote
prices within 5% of spot (a decimals or `sqrtPriceX96` mistake is orders of
magnitude off), the exact-output quote round-trips to the same size within 1%,
and a pair with no pool reports `ErrNoPool` rather than a zero quote.
