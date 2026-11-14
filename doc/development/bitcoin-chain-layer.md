# Bitcoin Chain Layer

`pkg/chain/btc` is the lowest Bitcoin layer: a JSON-RPC client for Bitcoin
Core (and providers that speak its API) with Go types that mirror the RPC JSON
field for field. `pkg/chain/btc/stream` follows the chain block by block on
top of it (see "Block stream" below). There is no indexer and no
`ChainSession` integration yet.

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

## Block stream

`pkg/chain/btc/stream` delivers the blocks of the node's chain in order, with
the same design as `pkg/chain/evm/stream`: something signals that the chain
moved, then every block from the cursor up to the head is read and
parent-checked.

- **The signal is a poll of `getbestblockhash`** (`PollInterval`, 5s by
  default), which is one cheap call against a ten-minute block time. The
  header is read only when the hash changed.
- **Backfill.** Blocks are read by height from the cursor up to the head
  minus `Confirmations`, so nothing is lost while the node was unreachable.
  Persist `Cursor().Height + 1` as `FromHeight` to resume.
- **Reorgs.** Every block's `previousblockhash` is checked against the last
  delivered block; a mismatch walks the recent window (`MaxReorgDepth`, 64)
  back to the common ancestor and emits `Reorg{Ancestor, Removed}`, newest
  first. Deeper than the window, `Run` returns `ErrReorgTooDeep`.
- **Same-height reorgs.** A competing block can replace the tip without the
  height moving, and the next block that would reveal it is ten minutes away.
  So whenever the best block hash changes, the stream also checks that its
  cursor is still the node's block at that height.
- **Shorter chains.** The chain with the most work wins, not the longest, so
  after a reorg the node's height can be lower than the cursor. Heads are not
  monotonic here, unlike the EVM stream; the stream waits until the node's
  chain reaches the cursor's height again before deciding what was removed.
- **Lagging backends.** A "height out of range" (-8) or "not found" (-5)
  answer for the next block means the backend that answered is behind the
  one that reported the head; the stream waits for the next poll instead of
  failing.
- **`FullTransactions`** reads blocks with verbosity=3, so every input has
  its prevout and every transaction its fee. Without it only the header and
  `TxIDs` are read; a full mainnet block is around 15 MB of JSON.
- **`Chain`** ("main", "testnet4", ...) makes `Run` fail when the node
  follows another chain, instead of retrying.

Callbacks (`OnBlock`, `OnReorg`) run on the `Run` goroutine in chain order.

Not here yet: ZMQ `hashblock` notifications (polling is enough at Bitcoin's
block time) and the mempool.

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

```
BTC_RPC_ENDPOINT=https://bitcoin-rpc.publicnode.com go test ./pkg/chain/btc/stream -run TestLiveStream -v
```

streams the last three blocks with full transactions and checks they chain.
