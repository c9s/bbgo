# Aave v3 Adapter

`pkg/defi/aave/v3` is the first protocol adapter built on the chain layer
(`doc/development/defi-chain-layer.md`). It is shaped like the Aave contracts,
not like a generic lending interface: the same deployment layout exists on
Ethereum, BSC and Polygon, so one set of bindings covers all three chains.

```go
aave, err := v3.NewClient(ctx, session.EVMChain(), session.Client(), v3.Options{
    Tokens:    session.TokenRegistry(),
    Allowance: session.AllowanceConfig(),
    Sender:    session.Sender(),     // omit for a read-only client
})
```

## Only the provider address is hardcoded

`addresses.go` holds one address per chain, the `PoolAddressesProvider` of the
main market. `NewClient` resolves the `Pool`, the `PoolDataProvider`, the oracle
and the market id from it in a single Multicall3 call, which is how Aave expects
a client to discover a market and keeps an upgrade from breaking this adapter.
That resolution is also the configuration check: a wrong provider, or a chain
without Aave, fails at construction instead of at the first trade.

`Options.Provider` points the client at another market or at a fork.

## Units, which Aave does not make obvious

| value | scale |
|---|---|
| `getUserAccountData` amounts | oracle base currency, **USD with 8 decimals** — not token units |
| `HealthFactor` | WAD (1e18), and **max uint256 when there is no debt** |
| `LiquidityRate`, `VariableBorrowRate` | RAY (1e27) **per-second** rates, not percentages |
| LTV, liquidation threshold, reserve factor | basis points |
| aToken balance | rebasing: scaled balance times the liquidity index |

`rates.go` has the conversions: `APR` is the nominal rate, `APY` compounds it
every second the way Aave's own interface reports it, and
`AccountData.HealthFactorFloat` returns `+Inf` for an account with no debt
rather than a nonsense number. `BaseCurrencyUnit` is read from the oracle, so
the base amounts are converted with the market's own scale.

## Reads

`AccountData`, `ReservesList`, `Reserve`, `Reserves`, `UserReserve`,
`UserReserves`, `ReserveConfig`, `ReserveTokens`, `AssetPrice`, `AssetPrices`.

`Reserves` and `UserReserves` read the whole market in one batched Multicall3
call — Ethereum lists 67 reserves, so doing that one request at a time would
make the RPC endpoint the bottleneck. `UserReserves` drops the reserves where
the account holds neither a balance nor a debt.

## Writes

`Supply`, `Withdraw`, `Borrow`, `Repay`, `RepayAll`, `SetUseAsCollateral`. They
need a sender; without one they return `ErrReadOnly`.

- `Supply` and `Repay` approve the pool first, following the session's allowance
  policy (exact amount by default).
- `Borrow` always uses the variable rate mode. Aave v3 removed the stable rate.
- `MaxAmount` means "everything" to `Withdraw` and `Repay`.
- `RepayAll` sizes the allowance from the current debt plus 0.5%, because the
  debt keeps accruing between the approval and the repayment, and then repays
  `MaxAmount` so the pool takes exactly what is owed.

`Options.Sender` is an interface, and `NewClient` rejects a typed nil: passing a
nil `*evm.Sender` into it would otherwise make `CanWrite` report true and panic
on the first write.

## CLI

```bash
bbgo aave market  --session bsc [--asset USDT]
bbgo aave account --session bsc [--address 0x...]

bbgo aave supply   --session bsc --asset USDT --amount 100 [--confirm]
bbgo aave withdraw --session bsc --asset USDT --amount 100 | --all [--confirm]
bbgo aave borrow   --session bsc --asset USDT --amount 50  [--confirm]
bbgo aave repay    --session bsc --asset USDT --amount 50  | --all [--confirm]
```

The write commands print the action and the account state before it, and only
send with `--confirm`. Unlike `bbgo chain send` they do **not** simulate first:
supplying and repaying revert until the pool is approved, so simulating an
unapproved first supply would report a failure the real path does not have.

## Testing

`go test ./pkg/defi/aave/v3` runs on a fake market that answers every ABI,
including through Multicall3, and a fake sender that records the encoded calls —
so the argument encoding of every write is asserted without a node.

The integration test validates the hardcoded provider addresses and every ABI
against a live market, and is skipped unless you opt in:

```bash
TEST_EVM_CHAIN=bsc TEST_EVM_RPC_ENDPOINTS=https://... go test ./pkg/defi/aave/v3 -run TestIntegration -v
```

It has been run against all three chains. It asserts the shape rather than
exact values: rates below 1000% APY (a unit error is orders of magnitude off), a
liquidity index at or above 1.0, a `uint40` timestamp after the v3 launch and
not in the future, a positive oracle price for every reserve, an LTV at or below
the liquidation threshold, and an infinite health factor for an account with no
debt.

## What this adapter does not do

- The stable rate mode, flash loans, e-mode, and the L2 encoded `Pool` entry
  points.
- aToken level accounting: the scaled balance and the liquidity index are
  exposed, but nothing reconciles a rebasing balance into a bbgo position yet.
