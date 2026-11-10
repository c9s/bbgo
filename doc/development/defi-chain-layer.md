# DeFi Chain Layer (EVM)

`pkg/chain/evm` is the bottom layer of the DeFi integration: raw EVM access for
Ethereum, BSC and Polygon. Protocol adapters (`pkg/defi/<protocol>`) are built on
top of it and are shaped like the contracts they wrap — there is deliberately no
protocol-agnostic `Quoter`/`Swapper` interface yet. It will be extracted once
Uniswap, Curve and Balancer are all implemented and the common shape is known.

## Package layout

```
pkg/chain/evm/              chain registry, endpoint pool, client, signer
pkg/chain/evm/token/        ERC20 metadata, balances, amount conversion
pkg/chain/evm/multicall/    Multicall3 batched reads
pkg/bbgo/chainsession.go    ChainSession: one chain + one address
```

## Two hard rules

1. **Raw amounts are `*big.Int`, never `fixedpoint.Value`.**
   The default (non-dnum) build of `fixedpoint` is an int64 with 8 decimal
   places (`pkg/fixedpoint/convert.go`), so `1e18` wei overflows it and 18-decimal
   dust is unrepresentable. `fixedpoint` appears only at the strategy and display
   boundary, and the conversion always goes through the token decimals:
   `token.ToFixedPoint`, `token.ToRaw`, `token.FormatRaw`. `ToFixedPoint` returns
   an error rather than wrapping around.

2. **Token decimals are read from the chain, never assumed.**
   USDT and USDC are 6 decimals on Ethereum and **18 decimals on BSC**. WBTC is 8.
   `token.Registry` resolves `symbol`, `name` and `decimals` in a single
   Multicall3 call and caches the result. The built-in address book
   (`token/known.go`) maps symbols to addresses only; it never carries decimals.

Reads that touch several contracts go through `multicall.Aggregate3`. Multicall3
is deployed at the same address on all three chains.

## Chain differences

| | Ethereum (1) | BSC (56) | Polygon (137) |
|---|---|---|---|
| native | ETH | BNB | POL |
| fee market | EIP-1559 | legacy (base fee ~0) | EIP-1559, high priority fee floor |
| default confirmations | 3 | 15 | 64 (deep reorg history) |
| block time | ~12s | ~0.75s | ~2s |

These live in `evm.Chain`; look a chain up with `evm.LookupChain("bsc")`. Aliases
(`eth`, `bnb`, `matic`) are accepted in configuration.

## Client

`evm.Client` holds one `evm.EndpointPool`. The pool dials endpoints lazily, keeps
the last known good one sticky, and fails over only on retryable errors — an
`execution reverted` is a deterministic result and is returned immediately
(`evm.IsRetryable`). Endpoint URLs are redacted before logging, because providers
put API keys in the path or query string.

`Client` implements `bind.ContractCaller`, so abigen-generated bindings can be
constructed from it directly. `Client.FilterLogs` splits the requested block
range into `Chain.MaxLogRange` chunks, since public providers cap `eth_getLogs`.

## ChainSession

`ChainSession` is the on-chain counterpart of `ExchangeSession`: **one chain plus
one address**. Use several sessions for several wallets. `Environment` owns them
(`environ.ChainSessions()`, `environ.ChainSession(name)`) and
`BootstrapEnvironment` initializes them, which dials the RPC endpoints and
verifies the reported chain id against the configured chain.

```yaml
chainSessions:
  eth:
    chain: ethereum
    rpcEndpoints:
      - https://mainnet.example/v3/KEY
      - https://backup.example      # failover
    confirmations: 3
    tokens:
      MYTOKEN: "0x..."              # extends or overrides the built-in address book

  bsc:
    chain: bsc
    # rpcEndpoints omitted: read from BSC_RPC_ENDPOINTS or BSC_RPC_URL

  polygon-readonly:
    chain: polygon
    address: "0x..."                # read-only session, no key needed
```

Private keys are never stored in the config file. A session loads
`<SESSION>_PRIVATE_KEY` from the environment (override with `privateKeyVar`), and
a session without a key is read-only, which is what the CLI, reconciliation and
backtesting need. When both a key and an `address` are configured, a mismatch is
a fatal configuration error — it means the wrong wallet would have been used.

Strategies reach the sessions through the injected `*bbgo.Environment`;
`ChainSessionMap` is a map type, and `dynamic.ParseStructAndInject` only injects
pointer, struct and interface fields.

## CLI

No database and no exchange API key needed:

```bash
bbgo chain info --session bsc
bbgo chain balance --session bsc
bbgo chain balance --session eth --tokens WBTC,USDT,0x...  --all
```

`chain balance` reports the native balance plus the ERC20 balances, batched into
one Multicall3 call, skipping zero balances unless `--all` is given.

## Sending transactions

`evm.Sender` owns the whole lifecycle for one address: nonce, simulation, gas,
fee, signing, broadcast, replacement and confirmation.

```go
sender := session.Sender()            // nil on a read-only session
result, err := sender.SendAndWait(ctx, evm.SendRequest{
    To:    &contract,
    Data:  callData,
    Label: "approve USDT",
})
```

`Submit` returns a `*PendingTx` handle immediately after the broadcast, and
`SendAndWait` is the blocking wrapper around it. `PendingTx.Wait` respects the
caller's context; abandoning the wait does not stop the background watch, which
is bounded by `SenderConfig.Timeout` instead.

The rules it enforces:

- **One transaction in flight per sender.** `Submit` blocks until the previous
  transaction of the same sender reaches a terminal state, so the nonce sequence
  is never ambiguous and a failure cannot leave a gap behind.
- **Nonces are counted locally.** `NonceManager` reads `eth_getTransactionCount`
  (pending) once and then counts; a nonce that is allocated but not broadcast is
  handed back, and `nonce too low` triggers a resynchronization. Nothing is
  persisted: a restart resynchronizes from the chain.
- **Every transaction is simulated first.** `eth_call` plus `eth_estimateGas`
  run before signing, and a revert is turned into an error wrapping
  `evm.ErrReverted` with the decoded reason (`revert.go` handles both the
  `Error(string)` return data and the message text). `SkipSimulation` opts out of
  the `eth_call`, not the estimation.
- **Fees come from `eth_feeHistory`.** The priority fee is a percentile of
  recent blocks, floored at `Chain.MinPriorityFee`, and
  `maxFeePerGas = baseFee*multiplier + tip`. BSC is priced as legacy
  (`eth_gasPrice`). `FeeConfig.MaxFeePerGasGwei` is an absolute ceiling; a
  ceiling below the current base fee fails fast with `ErrFeeCapExceeded` instead
  of sending something that can never be included.
- **Stuck transactions are replaced.** After `BumpInterval` the same nonce is
  re-signed at a higher fee (at least the 10% geth requires), up to
  `MaxBumpAttempts` and never above the ceiling. Every hash of the nonce is
  polled, because a bumped transaction may still be mined under an earlier hash,
  and `TxResult.ReplacedHashes` records the ones that were not.
- **Finality, then a re-read.** Confirmation prefers the `finalized` block tag
  and falls back to counting confirmations; afterwards the receipt is read again,
  so a transaction that was reorged out resumes waiting instead of being
  reported as done.
- **A revert on chain still returns a result.** `TxResult` plus an error wrapping
  `ErrTxReverted`, so the caller can account for the gas it burned.

`TxResult.GasCost` is `GasUsed * EffectiveGasPrice` in wei. It is not converted
to a quote currency: this layer has no price source, and that conversion belongs
to whoever owns the position.

`SendRequest.Private` reserves the MEV-protection hook. It is rejected today
(`ErrPrivateRelayNotConfigured` / `ErrPrivateRelayNotImplemented`) rather than
silently sending through the public mempool.

### Allowances

`token.EnsureAllowance` tops an allowance up only when it is short, and returns
a nil result when nothing had to be sent. The default policy approves the exact
amount the operation needs; `unlimited` approves `MaxUint256` once, and
`resetBeforeApprove` first approves zero for tokens that refuse to change a
non-zero allowance directly.

```yaml
chainSessions:
  eth:
    chain: ethereum
    rpcEndpoints: [ "https://mainnet.example/v3/KEY" ]
    tx:
      gasLimitMultiplier: 1.3
      bumpInterval: 45s
      timeout: 10m
      fee:
        maxFeePerGasGwei: 120     # absolute ceiling
        tipPercentile: 50
        bumpPercent: 15
        maxBumpAttempts: 3
      allowance:
        policy: exact             # or unlimited
```

## CLI

No database and no exchange API key needed. The read-only commands:

```bash
bbgo chain info --session bsc
bbgo chain balance --session bsc
bbgo chain allowance --session eth --token USDT --spender 0x...
```

The write commands simulate and print the plan by default; they only broadcast
with `--confirm`:

```bash
bbgo chain send --session bsc --to 0x... --amount 0.01
bbgo chain send --session bsc --to 0x... --amount 25 --token USDT --confirm
bbgo chain approve --session eth --token USDT --spender 0x... --amount 1000
bbgo chain approve --session eth --token USDT --spender 0x... --unlimited --confirm
```

A dry run is a real check: it runs the same `eth_call` and `eth_estimateGas` the
sender would, so a revert fails the command.

## Testing

`go test ./pkg/chain/...` runs entirely on fakes: `fakeTxClient` drives the whole
lifecycle (bumping, reorg, confirmation fallback, serialization) with no node.

Two integration tests are skipped unless you opt in:

```bash
# read-only, against any public endpoint
TEST_EVM_CHAIN=bsc TEST_EVM_RPC_ENDPOINTS=https://... go test ./pkg/chain/evm -run TestIntegration -v

# the full send path against a local node; needs foundry on PATH
go test ./pkg/chain/evm -run TestAnvil -v
```

The anvil tests start their own node with `--chain-id 56`, so the BSC entry of
the chain registry applies, and they use the standard anvil account #0.

## What is not here yet

- Private (MEV-protected) submission: the hook exists, the transport does not.
- Permit2 and signature-based approvals.
- A WebSocket log subscription stream, which is what a protocol adapter will
  want for watching pool events rather than polling.
- Notifications and Prometheus metrics for the transaction lifecycle; today it
  only logs.
