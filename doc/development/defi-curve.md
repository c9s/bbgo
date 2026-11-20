# Curve Adapter

`pkg/defi/curve` covers the Curve pools that Curve's own MetaRegistry indexes:
StableSwap and StableSwap-NG, their metapools, and the CryptoSwap pools
(twocrypto, tricrypto and their NG versions). It is built on the chain layer
(`doc/development/defi-chain-layer.md`) and the transaction layer.

```go
dex, err := curve.NewClient(ctx, session.EVMChain(), session.Client(), curve.Options{
    Tokens:    session.TokenRegistry(),
    Allowance: session.AllowanceConfig(),
    Sender:    session.Sender(),     // omit for a read-only client
})
```

Curve is not one interface, which is the whole reason this adapter looks
different from the Uniswap one. Where Uniswap v3 has a single pool ABI and a
router, Curve has a family of pool implementations that disagree about how a
coin is addressed and about how the pool is entered.

## One hardcoded address

`AddressProviderAddress` is the only address in the package, and Curve deploys
it at the same address on every chain, the way Multicall3 is. `NewClient`
resolves the chain's MetaRegistry from it (`get_address(7)`) and checks that the
registry answers `pool_count`, so a chain without Curve fails at construction
rather than as an empty pool list later.

An older AddressProvider lives at `0x0000000022D53366457F9d5E68Ec105046FC4383`.
It is still canonical on Ethereum, where both report the same MetaRegistry, but
it is not maintained elsewhere: on Polygon it points at a MetaRegistry that
indexes 361 pools where the current one indexes 1667, and on BSC its
MetaRegistry entry is the zero address.

| chain | MetaRegistry | pools |
|---|---|---|
| ethereum | `0xF98B45FA17DE75FB1aD0e7aFD971b0ca00e379fC` | 2509 |
| polygon | `0x296d2B5C23833A70D07c8fCBB97d846c1ff90DDD` | 1667 |
| bsc | `0x4C7A5A5D57F98D362f1c00D7135F0dA5B6f82227` | 449 |

## The index type is the trap

A StableSwap pool types a coin index as `int128`; a CryptoSwap pool types it as
`uint256`. The same call therefore has two selectors — `get_dy` is `0x5e0d443f`
on one family and `0x556d6e9f` on the other — and **nothing in the registry
reports which one a pool speaks.** The adapter detects it by calling both in one
batch and keeping whichever answers, then remembers it per pool for the life of
the client. `IndexKind` carries the result; `PoolABI(kind)` returns the matching
ABI.

Detection cannot be written as "success means right":

- A pool without the selector usually reverts, which is an unsuccessful
  multicall result.
- **A tricrypto-ng pool asked with `int128` indices returns successfully with an
  empty payload.** So a usable answer is one that is successful, long enough to
  hold a word, decodes, and is positive.

## Routes, not pools

On Curve the venue is a pool *and* a pair of coin indices, because the same pool
can be entered two ways. `Route` is that unit: the pool, `I` and `J`, whether the
pair is reached through the metapool's underlying coins, and the index kind.

`get_coin_indices` on the registry answers all of it except the index kind, and
its third return value is what decides between `exchange`/`get_dy` and
`exchange_underlying`/`get_dy_underlying`. A metapool holds another pool's LP
token, so a pair that looks tradable is reached through the base pool's coins.

`exchange` is encoded with four arguments, although the NG implementations take a
fifth receiver with a default value: Vyper compiles a default argument into one
selector per arity, so the four-argument form exists on every implementation and
always pays out to the caller. That is why `SwapRequest` has no recipient.

## Finding the pools

`find_pools_for_coins` is the registry's own search and is used when it works.
It is built from a coin pair index that not every chain's registry populates:
**on BSC it reverts for one argument order and returns an empty list for the
other**, although the pools are there. So an empty answer is believed only when
both argument orders agree on it without reverting; anything else falls back to
walking `pool_list` and matching `get_coins` and `get_underlying_coins` locally,
cached for the life of the client. On BSC that is 449 pools, five batched calls.

## Choosing which pools to quote

Curve lists 124 pools for USDC/USDT on Ethereum, most of them dust. Quoting all
of them is wasteful, and quoting only the first is wrong.

The ordering is the pool's balance of the output coin, because a pool cannot pay
out more of a coin than it holds: that balance is a hard upper bound on the swap,
so once a pool holds less than the best quote already found, neither it nor
anything behind it can win. `Quotes` walks the routes in that order, in batches
of 8, and stops on exactly that condition; `MaxQuotedPools` (16 by default) caps
the rest.

The bound only holds for a direct route. **The registry does not convert a
lending pool's underlying balances** — the busd pool returns exactly the same
array from `get_underlying_balances` as from `get_balances`, so the number is the
wrapped coin's balance on the wrapped coin's own scale. Read with the underlying
coin's decimals it is orders of magnitude out, which is why `Route` carries
`OutBalanceExact`, such a route is ordered but never bounded, and
`bbgo curve pools` prints `-` instead of a fictitious amount.

## A quote is arithmetic, not a promise

`get_dy` is the pool's own math, fee and price impact included, and that is all
it is. Curve keeps deprecated pools whose wrapped coins can no longer be
redeemed, and those keep quoting prices they cannot honour: the `ib` pool quotes
1000 USDC as 2135 USDT, against a cluster at 1.001 everywhere else. The Curve
front end hides such pools behind a whitelist this adapter deliberately does not
carry, because a whitelist goes stale silently.

What protects a swap instead:

- the transaction layer's mandatory `eth_call` simulation, which reverts before
  anything is broadcast;
- `min_dy`, which makes a fill worse than the floor impossible — `Swap` refuses a
  request with neither `SlippageBps` nor `AmountOutMinimum`;
- `bbgo curve quote` printing the whole table rather than only the winner, and
  `--pool` pinning the swap to a chosen pool.

## Swapping

`Swap` quotes, derives `min_dy`, approves, sends `exchange` or
`exchange_underlying`, and decodes what actually came out.

**The approval goes to the pool, not to a router**: on Curve the pool is the
contract that moves the coins. Only ERC20 to ERC20 swaps are supported; the pools
that accept native currency do it through a payable `exchange` with a `use_eth`
flag that this adapter does not wrap.

The realized output is read from the pool's `TokenExchange` event, whose shape
differs by family and by version — `int128` or `uint256` indices, and the NG
crypto pools append the realized fee and the packed price scale, each difference
changing the topic. All five topics are recognized, and the payload is read as
words rather than through one event's ABI: every variant lays out `sold_id`,
`tokens_sold`, `bought_id` and `tokens_bought` as its first four words and only
the trailing fields differ, so a version this package has not seen still decodes.
`bought_id` is checked against the route, which is what confirms the right event
was found.

## Units

| value | scale |
|---|---|
| every amount | raw base units of the coin, `*big.Int` |
| `Pool.Fees` | Curve's 1e10 fixed point; `FeeDenominator`, `Pool.Fee()` for a fraction |
| `Fees[0]`, `Fees[1]` | swap fee, admin fee |
| `Fees[2]`, `Fees[3]` | `mid_fee`, `out_fee` on a CryptoSwap pool, between which its fee moves |
| `Pool.A` | as the pool reports it, and a CryptoSwap pool uses its own scale |

The registry has a `get_A`, but it reverts for every pool tried on all three
chains, so `A` is read from the pool itself.

A registry read that a pool's handler cannot answer is left at its zero value
rather than failing the whole read: `get_pool_name` reverts for the NG crypto
pools, and a factory-deployed pool sometimes has a raw hex string for a name.

## CLI

```bash
bbgo curve pools --session ethereum --base USDC --quote USDT
bbgo curve pool  --session ethereum --address 0xbEbc44782C7dB0a1A60Cb6fe97d0b483032FF1C7
bbgo curve quote --session ethereum --from USDC --to USDT --amount 1000
bbgo curve swap  --session ethereum --from USDC --to USDT --amount 100 --slippage 50 [--pool 0x...] [--confirm]
```

`swap` prints the pool, the call it will make, the quote and the floor, and sends
only with `--confirm`.

## Not in this adapter

The Curve router and multi-hop routing, native-currency swaps, liquidity
(add/remove), the gauges and CRV rewards, and the `exchange_received` entry point
of the NG pools. The CryptoSwap-plus-underlying combination is handled by
construction — the index kind and the entry point are detected independently —
but no live pool on the three chains exercises it.
