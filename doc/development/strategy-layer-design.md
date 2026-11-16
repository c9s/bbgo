# Strategy-level design philosophy

This document describes *where* a new mechanism belongs in BBGO, and how it is
expected to move over time. Read it before adding a non-trivial mechanism to a
strategy, and before proposing a change to a core API.

## The two layers

- **Core** — `pkg/bbgo`, `pkg/types`, `pkg/core`, `pkg/indicator`,
  `pkg/exchange`, `pkg/notifier`. Shared, general, depended on by every
  strategy. A change here is expensive: it has to work for cases nobody has
  written yet.
- **Strategy** — `pkg/strategy/<name>`. Owned by one strategy, free to change,
  free to be wrong.

Strategy-level code is deliberately allowed a lot of latitude. Complex
strategies carry real machinery of their own, and that is not a defect — it is
where the machinery is supposed to start.

## Why mechanisms start at the strategy level

Designing a fully general core API up front is hard, and usually wrong. We do
not yet know which parts of a mechanism are essential and which are an accident
of the first strategy that needed it. So the sequence is:

1. **Implement it inside the strategy that needs it.** Concrete, specific,
   no attempt at generality.
2. **Let it be used and reshaped.** Other strategies may copy it; each copy
   teaches us which parts are genuinely shared and which were incidental.
3. **Wait until it stops moving.** The signal to promote is not "two strategies
   use it" — it is *we no longer expect to change its shape*.
4. **Then refactor it into the core API**, and migrate the strategies onto it.

Much of today's core arrived exactly this way:

- **Slack notifications** — `SlackAttachment()` / `slack.Blocks` rendering now
  lives on the domain types (`pkg/types/order.go`, `balance.go`, `kline.go`,
  `pkg/notifier/slacknotifier`), reached through `bbgo.Notify`. It began as
  per-strategy message formatting.
- **Indicators** — `pkg/indicator` and the v2 stream indicators were extracted
  from calculations that strategies first did inline.
- **Order execution** — `bbgo.GeneralOrderExecutor`
  (`pkg/bbgo/order_executor_general.go`), with its position, profit stats, trade
  collection and risk controls, is a generalization of the submit/cancel/track
  loops that individual strategies each hand-rolled.

## What this is not

Strategy-level machinery is a **stage**, not a destination. A strategy that
keeps accumulating subsystems is drifting toward a one-size-fits-all
implementation, which is the thing we are trying to avoid.

The standing preference for strategy code is: as small, as plain, and as direct
as possible. If a strategy's own code is mostly plumbing, the plumbing is a
candidate for promotion, not a feature of the strategy.

**The long-term goal is that a strategy can be implemented in the smallest,
shortest amount of code possible** — the strategy file expresses the trading
idea, and everything mechanical comes from the core.

## Practical guidance

When adding a mechanism:

- **Does the core already provide it?** Use it. Do not re-implement position
  tracking, order execution, notification formatting, indicators or persistence
  inside a strategy.
- **Is it new and unproven?** Put it in the strategy package. Do not widen a
  core interface to accommodate a first guess.
- **Is it proven and stable, and used by more than one strategy?** Propose the
  promotion to core as its own change, separate from strategy work, and migrate
  the existing users in the same series of changes so no duplicate remains.

When reviewing or refactoring:

- Prefer deleting strategy-level code by moving it to core over making the
  strategy-level version more configurable.
- A new core abstraction should be extracted from working code, not invented
  ahead of it. This is the same rule the DeFi protocol adapters follow — see
  `doc/development/defi-aave-v3.md` — where no protocol-agnostic
  `Quoter`/`Swapper` interface exists yet, because the concrete swap shapes
  differ too much to abstract in advance.
