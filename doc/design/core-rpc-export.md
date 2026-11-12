# ADR: Core RPC Export Middleware

- Status: Accepted
- Scope: `pkg/rpcexport`, `pkg/bbgo` (`Environment`, `Trader`), `pkg/pb`
  (`CoreRPCService`), `pkg/grpc`, `pkg/cmd`

## Context

bbgo had no way for an external caller to discover or invoke named
actions against the running framework. The existing gRPC services
(`MarketDataService`, `TradingService`, `UserDataService`) are fixed,
session/exchange-level operations — none of them expose anything about
bbgo itself (strategies, sessions, trading state as a whole).

An earlier iteration of this work (PR #2765, never merged) solved a
narrower problem: it let individual strategy instances opt into exporting
their own methods via a per-instance `bbgo.RPCExporter` interface, with
calls handed off through an inbox channel the strategy had to explicitly
drain from its own `Run()`/`CrossRun()` loop. That design was deliberately
set aside after review, for three reasons that reshaped this feature:

1. **gRPC is the only transport this needs to serve.** The earlier design
   kept hedging toward "transport-agnostic"; that hedge bought nothing
   real and cost extra abstraction.
2. **Control should be framework-owned, not strategy-owned.** This
   codebase has a known evolution pattern — Slack notifier/attachments,
   `Indicator`, `GeneralOrderExecutor` all started as ad hoc strategy-level
   mechanisms, matured, then got promoted into core framework APIs.
   Per-strategy flexibility is the right trade-off for things not yet
   mature; it isn't something to keep accumulating. The long-term goal is
   for a strategy to be implementable in the shortest, simplest code
   possible — asking every strategy to implement an RPC interface and run
   its own dispatch loop pulled in the opposite direction.
3. **Strategy code shouldn't have to think about RPC concurrency at all.**
   The per-strategy design required an explicit inbox-draining loop so a
   strategy could control ordering against its own callbacks. In practice
   that's not a guarantee bbgo can cheaply give anyway (a strategy's own
   `MarketDataStream`/`UserDataStream` callbacks already run on separate
   goroutines with no cross-stream exclusivity), so the complexity bought
   little. This version drops it: `Invoke` just calls the exported
   function directly.

## Decision

### The registry lives on `Environment`, components export directly

`pkg/bbgo/interact.go`'s `CoreInteraction` is the existing precedent for
"framework-owned, not-per-strategy" control: built once per `Trader`
(not per strategy), registering commands like `/sessions`/`/balances`
onto a shared registry. The RPC registry follows the same spirit, but
without even that wrapper type: `Environment` owns the one
`rpcexport.Registry` directly, built eagerly in `NewEnvironment`:

```go
type Environment struct {
    // ... existing fields ...
    rpcRegistry *rpcexport.Registry
}

func NewEnvironment() *Environment {
    return &Environment{
        // ... existing fields ...
        rpcRegistry: rpcexport.NewRegistry(),
    }
}

func (environ *Environment) RPCRegistry() *rpcexport.Registry {
    return environ.rpcRegistry
}
```

`Environment` is the more fundamental, earlier-constructed object
(`Trader` wraps `*Environment`, not the other way around), so it's the
natural place for a registry that any component — not just `Trader` —
might want to export onto. There's no `Trader.RPCRegistry()` delegate:
it would have been a pure pass-through to
`trader.environment.RPCRegistry()`, and every caller that needs the
registry — e.g. `pkg/grpc`'s `CoreRPCService` — already holds its own
`*bbgo.Environment` reference independently of `Trader` (every service
in `pkg/grpc` carries `Config`/`Environ`/`Trader` alongside each other),
so the delegate had no caller that couldn't just reach `Environment`
directly.

There's no dedicated wrapper type to route exports through (an earlier
iteration had one, `CoreRPC`; it added indirection around a single
`Export` call and was removed). A component exports by calling
`environ.RPCRegistry().Export(...)` directly, wherever it's naturally
initialized. `Trader.Run` does exactly this for its own demo `ping`
method, right next to the existing `NewCoreInteraction` wiring:

```go
if err := trader.environment.RPCRegistry().Export("ping", "demo core RPC handler: returns \"pong\"", false, func() (string, error) {
    return "pong", nil
}); err != nil {
    return err
}
```

Deciding what else should be exported (listing sessions, querying
positions, pausing/resuming, etc.) is deliberately left as later,
incremental work, the same way `CoreInteraction`'s commands grew one at a
time rather than arriving all at once.

### `pkg/rpcexport`: one registry, one flat namespace, direct `Invoke`

No per-instance addressing — there's exactly one registry, so there's
nothing to list or address by ID. The whole contract is:

```go
func (r *Registry) Export(name, desc string, mutating bool, fn interface{}, paramNames ...string) error
func (r *Registry) Methods() []MethodInfo
func (r *Registry) Invoke(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error)
```

`Export` returns `ErrDuplicateMethod` if `name` is already registered,
rather than silently overwriting the earlier registration — two
independently-written components can legitimately pick the same name
(e.g. two packages both trying to export `"ping"`), and that's worth
surfacing as an ordinary error rather than one silently shadowing the
other or crashing the process. (`fn` not being a function is still a
panic: that's a caller bug, not something two components could
legitimately collide on.)

`Invoke` reflects over `fn`'s parameter types, `json.Unmarshal`s a
JSON-array `params` into them positionally, calls `fn` in its own
goroutine (recovering any panic into an error instead of propagating),
and races the result against `ctx`'s deadline. There's no inbox channel,
no `Requests()`/`Dispatch()` split, and nothing for an exporter to drain
from its own event loop — `Export` once at startup, `Invoke` is called
directly whenever a gRPC request arrives.

Because `Export` only runs during one-time setup and `Invoke` only reads
the resulting map, concurrent `Invoke` calls are safe without a mutex, as
long as no `Export` call races with them (it doesn't, in practice — every
component's `Export` calls finish before `Trader.Run` starts anything
that could reach the gRPC server).

### Wire shape: `CoreRPCService`, no instance addressing

```proto
service CoreRPCService {
  rpc ListMethods(ListMethodsRequest) returns (ListMethodsResponse) {}
  rpc Invoke(InvokeRequest) returns (InvokeResponse) {}
}
```

Same conventions as the rest of `bbgo.proto`: parameters/results travel
as JSON strings (not per-method protobuf messages, since exports are
decided at runtime, not at `.proto` compile time), and business-logic
failures (unknown method) go through the response's embedded `Error`
field; only transport-level concerns (bad token, deadline exceeded) use
gRPC status codes.

### Auth, audit, and gating — unchanged in spirit

A single shared bearer token gates every `CoreRPCService` call, including
`ListMethods`. It's resolved in order — `--core-grpc-token`, then the
`BBGO_CORE_GRPC_TOKEN` env var, then a randomly generated token — and
only the generated case is logged (at startup, once); an
operator-provided token, from either the flag or the env var, is never
written to the log. `CoreRPCInterceptor`
audit-logs every call that reaches the service, successful or not, and is
scoped by method-prefix so `MarketDataService`/`TradingService`/
`UserDataService` stay unauthenticated and unaffected. `CoreRPCService` is
registered on the existing `pkg/grpc.Server` only when
`--enable-core-grpc` is set — deliberately a separate flag from
`--enable-grpc`, since this adds mutating control over bbgo, not just
read-only market/trading queries, and requires `--enable-grpc` to also be
set.

## Alternatives considered

- **Per-strategy `RPCExporter`** (PR #2765). Superseded for the reasons in
  Context: it pushed RPC-concurrency concerns onto every strategy author,
  and optimized for a flexibility (per-instance addressing) nothing
  actually needed yet.
- **Inbox channel + strategy-drained `Dispatch`.** Dropped along with the
  above — without per-strategy exports there's no one left to "drain" an
  inbox from their own loop, and a direct call is simpler to reason about
  than a channel hand-off with no real serialization guarantee behind it.
- **Per-method generated protobuf messages**, instead of a generic JSON
  envelope. Rejected for the same reason as before: would mean a `.proto`
  edit and `make grpc-go` run every time a component gained a new export.
- **Keeping instance/component addressing** in case multiple addressable
  "components" show up later. Rejected for now: one flat registry is all
  this needs today; reintroducing addressing is a future proto change if
  a real need for it appears, not speculative work now.
- **A `CoreRPC` wrapper type** holding `*Environment`/`*Trader` and
  centralizing every export inside its own `ExportRPC` method. Removed:
  it added a layer of indirection around what amounted to a single
  `Export` call, and every future component wanting to export something
  would have had to either route through it or bypass it — simpler to
  let any component call `environ.RPCRegistry().Export(...)` directly.
- **Silently overwriting a duplicate `Export` name.** Rejected: two
  independently-written components can legitimately pick the same name;
  silently letting the second one win would be a confusing, hard-to-debug
  failure mode. `Export` returns `ErrDuplicateMethod` instead.

## Consequences

- Positive: a strategy needs zero RPC-specific code to benefit from this
  — framework-level control is additive, not something every strategy
  author has to opt into and maintain.
- Positive: `rpcexport` has no dependency on `bbgo`/`grpc`, so the same
  `Registry` could back a second framework-owned surface later (though
  none is planned) without rework.
- Negative/accepted risk: because `Invoke` calls straight into the
  exported function, a mutating export can run concurrently
  with whatever else bbgo is doing at that moment. There's no attempt to
  serialize RPC calls against the rest of the framework's own event
  processing — exports are expected to be written with that in mind (e.g.
  taking whatever locks the underlying state already requires), the same
  as any other concurrent caller into `Trader`/`Environment` would need
  to.

## Example

See `pkg/bbgo/trader.go`'s `Run` method for the real, current `ping`
export — the snippet above is kept in sync with it. A mutating export
(e.g. forcing a graceful shutdown) would follow the same shape, just with
`mutating: true` and real logic in place of `return "pong", nil`, and
would check the returned error from `Export` the same way.

## References

- `pkg/rpcexport/registry.go` — the registry.
- `pkg/bbgo/environment.go` — `Environment.RPCRegistry()`, where the
  registry is constructed.
- `pkg/bbgo/trader.go` — the `ping` export inside `Run`, reached via
  `trader.environment.RPCRegistry()`.
- `pkg/pb/bbgo.proto` — `CoreRPCService` and its messages.
- `pkg/grpc/corerpc.go`, `pkg/grpc/interceptor.go`, `pkg/grpc/server.go` —
  service implementation, auth/audit interceptor, wiring.
- `pkg/cmd/run.go` — `--enable-core-grpc`, `--core-grpc-token`.
- `doc/topics/grpc.md` — existing guide for connecting to this gRPC
  server with `evans`; the same approach works for `CoreRPCService` once
  requests include `authorization: Bearer <token>` metadata.
