# ADR: Bitcoin Chain Layer

- Status: Accepted (RPC client, block stream, address watcher, balances);
  the rest is listed under "Not supported yet"
- Scope: `pkg/chain/btc`, `pkg/chain/btc/stream`, `pkg/chain/btc/watch`,
  `pkg/chain/btc/balance`, `pkg/chain/btc/esplora`, `pkg/cmd/btc*.go`
- Usage guide: `doc/development/bitcoin-chain-layer.md` (how to use the
  packages). This record is the why, and what is deliberately missing.

## Context

bbgo can read EVM chains (`pkg/chain/evm`, `ChainSession`) but had nothing for
Bitcoin. The motivating use case was following large on-chain movements
(whales, exchange and ETF wallets) in real time. Detecting those is an
application-level problem — it needs address labels, clustering and
thresholds — so it was split off and deferred. What has to exist first is the
layer underneath it: reading the chain as it moves, block by block, with
correct values, senders and fees.

Bitcoin is not an EVM chain with a different RPC. The differences decide most
of this design:

| | EVM | Bitcoin |
|---|---|---|
| State | accounts with balances | unspent transaction outputs (UTXOs); no balances anywhere |
| Sender | `from` field | none; it is the script of the output each input spends |
| Fee | `gasUsed × price`, in the receipt | `Σ inputs − Σ outputs`, written nowhere |
| Amounts | wei, needs `*big.Int` | satoshis, 21M BTC fits in an int64 |
| Finality | `finalized` tag | none; confirmations only, and the heaviest chain wins, not the longest |
| Block time | seconds | ~10 minutes, high variance |
| Address lookup | `eth_getBalance` on any node | not possible on a node; needs an external indexer |

## Decision

### Shape the lowest layer like the API underneath it

The bottom layer mirrors Bitcoin Core's JSON-RPC field for field and adds no
model of its own. No indexer, no `ChainSession` integration, no shared
abstraction with the EVM layer. Higher layers (stream, watcher, balances) are
separate packages built only on that client, so each can be replaced without
touching the others.

### RPC client (`pkg/chain/btc`)

- **Own types, no btcd dependency.** The types decode the RPC JSON directly.
  Script and address decoding is left to the node: every `ScriptPubKey`
  already carries `address` and `type`.
- **Senders and fees come from the node.** `GetBlock` asks for verbosity 3
  (Core 23+), `GetRawTransaction` for verbosity 2 (Core 25+), so each input
  carries its `prevout`. That turns "who paid" and "what fee" from one extra
  lookup per input into a field read. On older nodes `Prevout` is nil.
- **`Hash` is stored in wire byte order** and printed reversed, as the RPC and
  explorers do. Mixing the two orders is the classic Bitcoin bug; the type
  makes it impossible by never exposing the raw hex.
- **`Amount` is int64 satoshis parsed from the JSON number literal**, never
  through float64 (`0.1 + 0.2` problems on 8-decimal values).
- **Both error dialects.** Core 28+ answers JSON-RPC 2.0; older nodes answer
  HTTP 500/404 with an error body. Both become `*RPCError`.
- **Auth** by URL credentials, explicit basic auth, or the node's cookie file,
  re-read after a 401 because the cookie changes on every node restart.

### Block stream (`pkg/chain/btc/stream`)

Same design as `pkg/chain/evm/stream`: a signal that the chain moved, then
every block from the cursor to the head is read and parent-checked.

- **Polling `getbestblockhash`** (5s) is the signal. One cheap call against a
  ten-minute block time; ZMQ would need a reachable node socket, which no
  hosted provider offers.
- **Reorgs** are detected by `previousblockhash` and walked back within a
  window (`MaxReorgDepth`, 64), reported newest first.
- **Same-height reorgs.** A competing block can replace the tip without the
  height changing, and the block that would reveal it is ten minutes away. On
  every best-hash change the stream checks its cursor is still on the chain.
- **Heads are not monotonic.** Most work wins, not most blocks, so the node's
  height can drop below the cursor. The stream waits instead of failing.
- **Lagging backends.** Providers load-balance across nodes; "height out of
  range" (-8) or "not found" (-5) for the next block means a backend is behind,
  not that the block is gone. The stream retries on the next poll.
- **`FullTransactions` is opt-in.** A full mainnet block is ~15 MB of JSON at
  verbosity 3; header plus txids is enough for many consumers.

### Address watcher (`pkg/chain/btc/watch`)

Tracks the UTXOs of a set of addresses or raw scripts over a full-transaction
stream, emitting `Received` and `Spent` events.

- **Spends are matched by outpoint**, so a spend of a tracked output is seen
  even without a prevout; a spend of an untracked older output is still
  reported from the prevout, with `Known` false.
- **Reorgs are undone from a per-block journal** kept for the stream's reorg
  window, and reported as `OnRevert` in reverse order, so its state always
  matches the stream's chain.
- **It only knows what it saw.** There is no address index on the node, so
  the starting state must be seeded. `scantxoutset` reports the height its
  UTXO set is from; seeding from it and starting the stream at height + 1 is
  exact.

### Balances (`pkg/chain/btc/balance`)

There are no balances on chain, so a balance is derived: the sum of the
unspent outputs locked to an address, and how much of it is spendable depends
on the tip. The two concerns are separate:

- **`Snapshot{Height, UTXOs}`** — UTXOs as of a tip height, in the shared
  `btc.UTXO` shape (moved up from the watcher so every source uses it).
- **`Compute(snapshot, minConfirmations)`** splits each address into
  `Confirmed`, `Pending` (fewer confirmations, mempool included) and
  `Immature` (coinbase outputs younger than `CoinbaseMaturity`, 100 blocks,
  whatever `minConfirmations` is). Immature coinbase cannot be spent no matter
  how many confirmations the caller asks for, so it is its own bucket rather
  than a confirmation threshold.
- **`Source`** produces the snapshot. Three exist, and none is complete:

  | Source | Sees | Gives up |
  |---|---|---|
  | `CoreSource` (`scantxoutset`) | confirmed outputs, coinbase flag, exact height, descriptors | needs your own node; tens of seconds per scan; no mempool |
  | `EsploraSource` | confirmed and mempool outputs, minus mempool spends | operator learns the addresses; no coinbase flag; busy addresses refused; not atomic across addresses |
  | `watch.Watcher.Snapshot()` | outputs since the watch started, plus seeded ones | no history; no mempool |

## Findings

- **Fees and the subsidy check out exactly.** On block 975765, all 7730
  transactions had `fee = Σ prevout values − Σ output values`, and the
  coinbase claimed exactly subsidy + fees. That is the end-to-end test for
  `Amount` parsing and verbosity-3 prevouts.
- **`https://bitcoin-rpc.publicnode.com`** (Core 32, txindex) answers
  `getblock` verbosity 3, `gettxout`, the mempool calls and batches, but
  `scantxoutset` fails with `-32701 broker: node error`. `CoreSource` and
  `bbgo btc watch --seed` therefore need a self-hosted node, and have only
  been tested against a fake RPC.
- **Esplora has no coinbase flag.** F2Pool's payout address
  `1AfCc4F9c4VTYSE31PUe2kUEKs6ZxiDjxm` holds coinbase outputs from the last
  100 blocks that cannot be spent yet; `EsploraSource` counts them as
  confirmed. Telling them apart would need one extra transaction lookup per
  recent UTXO.
- **Public Esplora instances cap busy addresses.** Exchange hot wallets get
  HTTP 400 "Too many unspent outputs" from public instances such as mempool.space.
  The addresses that matter most for the motivating use case are exactly the
  ones a public indexer refuses.
- **The Esplora tip is read after the UTXOs.** Reading it first can produce a
  snapshot whose height is below one of its UTXOs; the snapshot height is
  raised to the highest UTXO height for the same reason.
- **Address reuse is normal.** Exchanges, ETF custodians and mining pools keep
  fixed addresses for years, which is what makes watching by address useful at
  all. Retail HD wallets do the opposite (fresh receive and change address per
  payment), so one owner is many addresses.

## Rejected alternatives

- **Running an indexer (electrs, Fulcrum, Esplora) as part of bbgo.** It is a
  separate service with its own storage (hundreds of GB) and reorg handling.
  bbgo talks to one through `pkg/chain/btc/esplora` instead.
- **Building on btcd / btcutil.** It would bring script decoding and address
  encoding, but the node already returns both, and the RPC JSON is what the
  layer must mirror.
- **ZMQ `hashblock` as the stream signal.** Not offered by hosted providers;
  polling costs one call per five seconds against a ten-minute block time.
- **A balance field on the watcher only.** A balance is a function of UTXOs
  and a tip, and the UTXOs can come from three places; tying it to the
  watcher would have duplicated the confirmed / pending / immature rules per
  source.
- **Treating immature coinbase as "pending".** It has confirmations, often
  many; `minConfirmations` cannot express "not spendable before block N".

## Not supported yet

Recorded so the next change knows where the edges are.

### Mempool and 0-conf

- **No mempool stream.** `GetRawMempool` / `GetMempoolEntry` exist but nothing
  follows the mempool. Real-time alerts before a block (the original use case)
  need one, and it brings its own problems: replace-by-fee (a pending
  transaction replaced by a conflicting one, full RBF is the default since
  Core 28), eviction and expiry, CPFP packages, and transactions that never
  confirm. A mempool event is a hint, not a fact.
- **The watcher ignores the mempool.** Its events are confirmed only, and
  `Watcher.Snapshot()` has no pending outputs.
- **No fee estimation** (`estimatesmartfee`, mempool fee histograms).

### Transport

- **No ZMQ** (`hashblock`, `rawtx`, `sequence`). Polling is enough for blocks;
  a mempool stream on a self-hosted node is where ZMQ starts to pay off.
- **No endpoint failover** across providers, and no rate limiting; one
  endpoint per client.
- **No Electrum protocol** source (electrs / Fulcrum speak it, and they
  index by script hash with subscriptions).

### State and history

- **No address history.** Neither the node nor the watcher can list the past
  transactions of an address; Esplora can (`/address/:addr/txs`, paginated)
  but the client only reads UTXOs.
- **Watcher state is not persisted.** The stream cursor can be resumed
  (`FromHeight`), but the UTXO set and the reorg journal live in memory; a
  restart needs a new seed.
- **Seeding is not atomic with the stream** across sources other than
  `scantxoutset`; seeding from Esplora can miss or double-count a block
  mined between the two reads.
- **`EsploraSource` cannot tell immature coinbase apart** (see Findings), and
  fails on capped addresses instead of paging.

### Addresses and scripts

- **No local script / address decoding.** Matching relies on the address the
  node writes; outputs without one (bare multisig, `OP_RETURN`, non-standard)
  are matched by script hex only.
- **Descriptors are only understood by `scantxoutset`.** The watcher cannot
  watch an xpub range (`wpkh(xpub.../0/*)`), and there is no gap-limit
  derivation.
- **No address validation** or network check (mainnet vs testnet prefixes)
  before a watch starts.

### Writing to the chain

- **Read-only.** No transaction building, coin selection, signing, PSBT,
  `sendrawtransaction` or fee bumping. The EVM `Sender` has no Bitcoin
  counterpart.

### Integration and applications

- **No `ChainSession`.** The Bitcoin layer is reachable only from Go code and
  `bbgo btc`; strategies cannot declare a Bitcoin session in YAML.
- **No large-transfer detection, address labels or clustering.** The
  application the layer was built for is deferred. When it comes it needs:
  labels for known entities (exchanges, ETF custodians, pools), the
  common-input-ownership heuristic and change detection to group addresses
  (union-find), CoinJoin detection to stop false merges, and thresholds that
  exclude change (`--min-value` today counts change as moved value).
- **No Taproot-era protocols:** Ordinals, inscriptions, BRC-20 and Runes are
  not parsed; their outputs show up as ordinary (often dust) outputs.
- **No Lightning, sidechains or wrapped BTC** on other chains.

## Consequences

- Every consumer gets exact amounts, senders and fees without extra lookups,
  provided the node is Core 23+ (25+ for single transactions).
- Watching by address works for long-lived addresses (exchanges, custodians,
  pools) and is exact when seeded from your own node; with public
  infrastructure only, balances of busy addresses are not available at all.
- The packages are independent layers: a mempool stream, a ZMQ signal or an
  Electrum source can be added beside the existing ones without changing them.
