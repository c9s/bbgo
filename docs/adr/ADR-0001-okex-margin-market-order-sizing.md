# ADR-0001: OKX margin market order sizing follows the side (no tgtCcy)

Status: accepted

## Context

OKX's `POST /trade/order` defines `sz` differently per instrument/mode:

- SPOT market orders: sz unit is set by the `tgtCcy` parameter (base_ccy or quote_ccy).
- MARGIN market orders: `tgtCcy` is **not applicable** (rejected with
  "does not support the tgtCcy parameter"), and the sz unit is fixed by the
  order side — **MARGIN buy: sz in quote currency; MARGIN sell: sz in base
  currency**.

bbgo's `order.Quantity` is always base-denominated. The pre-existing code sent
`tgtCcy=base_ccy` for every spot market order (correct for cash), but that
same call is made on margin market orders, which OKX rejects — so on a
multi-currency margin (level-3) account, both hedge buys and hedge sells
failed.

## Decision

In `okex.Exchange.SubmitOrder` (pkg/exchange/okex/exchange.go):

1. Margin market orders send **no** `tgtCcy`.
2. Margin **sell** market orders pass `order.Quantity` through unchanged
   (sz in base — matches bbgo's unit).
3. Margin **buy** market orders convert the size to the quote notional at the
   best ask (`QueryTicker` → `PriceTypeAsk` → `Market.FormatPriceCurrency`),
   because OKX expects sz in quote for that side.

Spot (cash) market orders keep `tgtCcy=base_ccy`, unchanged.

The buy conversion is a synchronous REST ticker lookup inside `SubmitOrder`.
The hedge account is a cross-margin account with funded quote, so a buy at the
best ask is within the notional available; a zero ask is rejected explicitly
rather than silently falling back.

## Consequences

- Hedge sells on OKX cross-margin accounts work without size conversion.
- Hedge buys work, with one extra public REST call per order (public rate
  limit, not counted against authenticated rate limits).
- If OKX ever changes the sz unit of margin market orders, the exchange
  adapter is the only place that needs to change — the xmaker strategy is
  unaffected (it always submits base-denominated quantities).
- `debt-quota` uses bbgo's hard-coded MMR table while `MarginLevel` on the
  okex adapter uses OKX's real `mmr` field; the two values are not equivalent
  and this divergence is accepted for now (see strategy-level ADRs / the
  launch checklist).
