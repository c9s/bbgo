# Bitcoin Chain Layer

`pkg/chain/btc` is the lowest Bitcoin layer: a JSON-RPC client for Bitcoin
Core (and providers that speak its API) with Go types that mirror the RPC JSON
field for field. It has no block stream, no indexer and no `ChainSession`
integration yet; those are meant to be built on top of it.

Read this before building on it. Bitcoin is not an EVM chain with a different
RPC: the data model is different, and most EVM habits are wrong here.

## The UTXO model

There are no accounts and no balances on chain. A transaction spends earlier
outputs (its inputs) and creates new outputs; an output that has not been
spent yet is a UTXO.

```
TxIn  = Outpoint (txid:vout of the spent output) + scriptSig / witness
TxOut = Value (satoshis) + ScriptPubKey (the locking script; usually an address)
fee   = Σ input values − Σ output values   (not written in the transaction)
```

Consequences for anyone reading the chain:

- **A transaction has no sender field.** The sender is the `ScriptPubKey` of
  the output each input spends. `GetBlock` asks for verbosity=3 so the node
  attaches it as `TxIn.Prevout` (Core 23+); `GetRawTransaction` uses
  verbosity=2 for the same (Core 25+). On older nodes `Prevout` is nil.
- **The first transaction of a block is the coinbase.** Its single input
  spends nothing (`TxIn.Coinbase` is set, `TxID` is zero), so it has no
  prevout and no fee. Its outputs may claim at most `BlockSubsidy(height)` plus
  the fees of the block, and are spendable only after `CoinbaseMaturity`
  (100) confirmations.
- **The node has no address index.** It cannot list the transactions or the
  balance of an address. `GetTxOut` answers for one known outpoint (unspent or
  not), `ScanTxOutSet` walks the whole UTXO set for a set of descriptors (tens
  of seconds on mainnet, usually disabled by public providers). Address
  history needs an external indexer (electrs, Fulcrum, Esplora).
- **Wallets change addresses.** HD wallets use a fresh receive address per
  payment and send change to a fresh address, so one owner is many addresses.

## Gotchas handled by the types

- **Hash byte order.** `Hash` holds the bytes as the double SHA-256 produced
  them (wire order). The RPC, explorers and `Hash.String()` show them
  reversed. Never hex-encode the array directly to compare it with an RPC
  value; use `NewHashFromString` / `String`.
- **Amounts are exact.** The RPC writes BTC amounts as JSON numbers
  (`0.17777402`). `Amount` is an int64 of satoshis parsed from the number
  literal, never through a float64. Unlike EVM wei, 21 million BTC in satoshis
  fits in an int64, so no `*big.Int` is needed.
- **Two error dialects.** The client sends JSON-RPC 2.0. Core 28+ answers in
  2.0; older nodes ignore the version and answer the legacy way, with an HTTP
  500 or 404 and a JSON body carrying the error. The client reads the body
  whatever the status and returns `*RPCError`; `IsNotFound` checks for code
  -5, which is what Core answers for a missing block or transaction.
- **Null results.** `gettxout` answers `null` for a spent or unknown output;
  `GetTxOut` turns that into `ErrNotFound`.

## Authentication

Credentials in the endpoint URL (`http://user:pass@127.0.0.1:8332`) are used as
basic auth and stripped from `Endpoint()`. `WithCookieFile` reads the `.cookie`
a local bitcoind writes; the cookie changes on every node restart, so it is
read again after a 401.

## Providers

- `https://bitcoin-rpc.publicnode.com` (Core 32 at the time of writing)
  answers `getblock` verbosity=3, `gettxout`, the mempool calls and batches,
  but not `scantxoutset`.

## Exercising it

```
BTC_RPC_ENDPOINT=https://bitcoin-rpc.publicnode.com go test ./pkg/chain/btc -run TestLiveNode -v
```

reads the tip block, checks every transaction's fee against its prevouts, and
checks that the coinbase claims no more than the subsidy and the fees.
