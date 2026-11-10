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

## What is not here yet

Transaction sending: nonce management, EIP-1559/legacy fee strategy, gas bumping,
receipt waiting with confirmation depth and reorg checks, `eth_call` simulation
and allowance management. That is the next phase, and it belongs in this package
so that no protocol adapter reimplements it.
