# Bitcoin Chain Layer

`pkg/chain/btc` is the lowest Bitcoin layer: a JSON-RPC client for Bitcoin
Core (and providers that speak its API) with Go types that mirror the RPC JSON
field for field. `pkg/chain/btc/stream` follows the chain block by block on
top of it (see "Block stream" below). There is no indexer and no
`ChainSession` integration yet.

The design record, with what is deliberately not supported yet, is
`doc/design/bitcoin-chain-layer.md`.

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

## Address watcher

`pkg/chain/btc/watch` tracks the UTXOs of a set of addresses (or raw
scripts, for outputs without an address) over a full-transaction stream:

```go
s := stream.New(client, stream.Config{FullTransactions: true, Confirmations: 1})
w := watch.New(watch.Config{Addresses: []string{"bc1q..."}})
_ = w.Attach(s) // fails unless the stream reads full transactions

w.OnEvent(func(e watch.Event) { ... })  // Received / Spent
w.OnRevert(func(e watch.Event) { ... }) // the same events, undone by a reorg
```

- **Events.** `Received` is an output paid to a watched address; `Spent` is
  a watched output spent by an input, with `TxID` the spending transaction and
  `Block` where it happened. Transactions are applied in block order, so an
  output created and spent in one block yields both.
- **Reorgs.** The watcher keeps each block's events for `MaxReorgDepth`
  blocks (the stream's, via `Attach`). On a reorg it undoes the removed
  blocks newest first and emits `OnRevert` for their events in reverse order,
  so `UTXOs()` and `Balances()` always match the stream's chain.
- **It only knows what it saw.** There is no address index to ask for the
  starting state, so a balance counts only the outputs received after the
  watch started. A spend of an older output is still reported from the
  input's prevout, with `Known` false. `Seed` adds outputs known from
  elsewhere: `scantxoutset` reports the height its UTXO set is from, so
  seeding from it and starting the stream at that height + 1 is exact.
  Seeded outputs cannot be reverted by a reorg below their height.
- **Matching** is on the address the node writes (bech32 lowercased, base58
  as is), or the script hex. A spend of a tracked output is matched by
  outpoint, so it works without a prevout too.
- **Coinbase outputs** carry `Coinbase`; `UTXO.Mature(tip)` reports whether
  they can be spent in the next block (`btc.CoinbaseMaturity`).

The watcher is safe to query from other goroutines; callbacks run on the
stream's goroutine, after the state is updated. `Snapshot()` hands its UTXOs
and tip to `balance.Compute` (see "Balances").

## Balances

There are no balances on chain: an address's balance is the sum of the
unspent outputs locked to it, and how much of it is spendable depends on the
chain tip. `pkg/chain/btc/balance` separates the two concerns.

- **`Snapshot{Height, UTXOs}`** is a set of `btc.UTXO` as of a tip height.
  `btc.UTXO` is the shared UTXO shape: outpoint, address (or script hex),
  value, the height it was created at (0 in the mempool) and the coinbase
  flag.
- **`Compute(snapshot, minConfirmations)`** sums it per address into
  `Confirmed` (at least `minConfirmations`), `Pending` (fewer, mempool
  included) and `Immature` (coinbase outputs younger than 100 blocks,
  whatever `minConfirmations` is). `Sum` and `Of` total and look up.
- **Sources** produce the snapshot:

  | Source | Sees | Caveats |
  |---|---|---|
  | `CoreSource` (`scantxoutset`) | confirmed outputs, coinbase flag, exact height | your own node; scans the whole UTXO set (tens of seconds); takes descriptors too |
  | `EsploraSource` (`pkg/chain/btc/esplora`) | confirmed and mempool outputs, minus those spent in the mempool | the operator learns the addresses; no coinbase flag, so immature coinbase counts as confirmed; public instances cap busy addresses (HTTP 400 "Too many unspent outputs") |
  | `watch.Watcher.Snapshot()` | what the watcher has seen since it started, plus seeded outputs | follows reorgs |

  `EsploraSource` lists the addresses one by one and reads the tip after
  them, so the snapshot height is at least the height of every UTXO; it is
  not atomic across addresses.

## Providers

- `https://bitcoin-rpc.publicnode.com` (Core 32 at the time of writing)
  answers `getblock` verbosity=3, `gettxout`, the mempool calls and batches,
  but not `scantxoutset`.

## Exercising it

`bbgo btc` talks to the node directly, without a config file or a session.
The endpoint comes from `--rpc` or `BTC_RPC_ENDPOINT` (credentials may be in
the URL), a local node's cookie from `--rpc-cookie` or `BTC_RPC_COOKIE`.

```
bbgo btc info                                  # chain, height, best block
bbgo btc block 975765 --tx --min-value 20      # fees, miner tag, transactions moving >= 20 BTC
bbgo btc tx <txid> [--block <hash>]            # inputs with their prevouts, outputs, fee
bbgo btc utxo <txid>:<vout>                    # gettxout: unspent or not
bbgo btc scan <address|descriptor>...          # scantxoutset; needs your own node
bbgo btc stream [--full-tx --min-value 100] [--confirmations N] [--from-height N] [--chain main]
bbgo btc watch <address>... [--seed | --from-height N] [--confirmations N]   # received / spent, balances
bbgo btc balance <address|descriptor>... [--esplora https://mempool.space/api] [--min-conf 6] [--utxos]
```

`--min-value` compares the sum of a transaction's outputs, change included.

The live tests:

```
BTC_RPC_ENDPOINT=https://bitcoin-rpc.publicnode.com go test ./pkg/chain/btc -run TestLiveNode -v
```

reads the tip block, checks every transaction's fee against its prevouts, and
checks that the coinbase claims no more than the subsidy and the fees.

```
BTC_RPC_ENDPOINT=https://bitcoin-rpc.publicnode.com go test ./pkg/chain/btc/stream -run TestLiveStream -v
```

streams the last three blocks with full transactions and checks they chain.
