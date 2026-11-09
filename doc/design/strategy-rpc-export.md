# ADR: Strategy RPC Export Middleware

- Status: Accepted
- Scope: `pkg/rpcexport`, `pkg/bbgo` (`Trader`), `pkg/pb` (`StrategyRPCService`), `pkg/grpc`, `pkg/cmd`

## Context

There was no way for an external caller to discover or invoke named actions
on a *running* strategy instance. The existing network surfaces are both
fixed and strategy-agnostic:

- `pkg/pb`'s gRPC services (`MarketDataService`, `TradingService`,
  `UserDataService`) only expose session/exchange-level operations
  (orders, trades, klines, balances) — none of them reference a strategy
  at all.
- `pkg/server`'s HTTP API has exactly one strategy-related endpoint
  (`/api/strategies/single`), which dumps config/state read-only; it is
  not an invocation mechanism.
- Neither has a registry mapping a strategy's `InstanceID()` to a live
  object an API handler could call into.

The closest *existing* mechanism, `pkg/interact` (`bbgo.RegisterCommand`),
solves a related but different problem: it's a global, text-argument,
chat-bot-oriented command system for Telegram/Slack
(`ParseFuncArgsAndCall` shell-tokenizes a string and converts args via
`strconv`). It isn't instance-scoped, and its argument model doesn't fit
structured, possibly-mutating RPC payloads.

We wanted: a strategy instance to explicitly declare which of its own
methods are safe to call remotely and how (read vs. mutating), an external
caller to list running instances and each instance's exported methods
(with enough schema to know what to send), and to invoke one synchronously
— over gRPC, authenticated, and fully audited, since exported methods can
mutate live trading state.

## Decision

### Transport: new gRPC service, same server, separately gated

Add `StrategyRPCService` (`ListInstances`, `ListMethods`, `Invoke`) to the
*existing* `pkg/grpc.Server` (`pkg/grpc/server.go`) rather than a new
listener — that server already exists, is already wired into `bbgo run`
behind `--enable-grpc`, and already holds `Config`/`Environ`/`Trader`.

It is gated by a **separate** flag, `--enable-strategy-rpc`, which
requires `--enable-grpc` to also be set. The base gRPC server only exposes
read-only market/trading data; this feature adds mutating control over
live strategies, so turning one on must not silently turn on the other.

### Registration: a new API, not `pkg/interact`

A strategy opts in by implementing:

```go
// pkg/bbgo/trader.go
type RPCExporter interface {
    ExportRPC(reg *rpcexport.Registry)
}
```

`Trader.Initialize` auto-detects it, exactly like it already does for
`StrategyDefaulter`/`StrategyValidator`/`StrategyInitializer`: on every
strategy that implements `RPCExporter`, it creates a
`rpcexport.NewRegistry(id)` (using `dynamic.CallID(strategy)`, the same
instance-ID resolution `Trader.LoadState` already uses for persistence),
calls `ExportRPC(reg)` once, and keeps the registry in a new
`rpcExports map[string]*rpcExportEntry` field. `Trader.RPCRegistry(id)`
and `Trader.ListRPCInstances()` are the lookups the gRPC layer uses.

This is a new, `pkg/interact`-independent registration surface because
the two systems have genuinely different argument models (see
Alternatives). A strategy that wants both a chat command and an RPC export
simply calls both `bbgo.RegisterCommand` and implements `RPCExporter`;
nothing here changes `pkg/interact`.

### Core registry: `pkg/rpcexport`

A new leaf package (no `bbgo`/`grpc` imports, to avoid import cycles),
used by both the registration side (`pkg/bbgo`) and the transport side
(`pkg/grpc`):

```go
type Registry struct { /* instanceID, methods, inbox chan *Call */ }

func NewRegistry(instanceID string) *Registry
func (r *Registry) Export(name, desc string, mutating bool, fn interface{}, paramNames ...string)
func (r *Registry) Methods() []MethodInfo
func (r *Registry) Requests() <-chan *Call
func (r *Registry) Dispatch(call *Call)
func (r *Registry) Enqueue(ctx context.Context, method string, params json.RawMessage) (<-chan CallResult, error)
```

`Export` accepts an **arbitrary-signature Go function** plus a
`mutating bool` the strategy author sets explicitly — there is no
reflection-over-all-methods auto-exposure. `Dispatch`'s reflection core is
modeled on `pkg/interact/parse.go`'s `ParseFuncArgsAndCall` (reflect over
the function's parameter types, build `[]reflect.Value`, `fv.Call(...)`,
inspect outputs for an `error`), but swaps its `strconv` string-arg
conversion for `json.Unmarshal` into each reflected parameter type, and
marshals the first non-error return value back to JSON. Because Go
reflection cannot recover a compiled function's real parameter names,
`Export`'s `paramNames` are optional hints used only for the discovery
schema; omitted ones fall back to `"arg0"`, `"arg1"`, etc.

### Concurrency: the strategy owns it, not the middleware

A strategy instance's own callback delivery is **not** single-threaded
today — `MarketDataStream` and `UserDataStream` each run on their own
goroutine, so e.g. `OnKLineClosed` and `OnOrderUpdate` can already fire
concurrently for one instance (more so for `CrossExchangeStrategy` with
multiple sessions). There is no existing "the strategy's own goroutine" to
transparently piggyback RPC calls onto.

Rather than retrofit a framework-wide lock around every existing callback
dispatch site (`pkg/bbgo/session.go`'s `OnKLineClosed`/`OnOrderUpdate`/etc.
registrations) — a much larger, separate change — `Registry` delivers a
pending call onto a buffered inbox channel (`Requests()`), and the
strategy drains it explicitly from inside its own `Run()`/`CrossRun()`
loop, at a time of its choosing, by calling `Dispatch(call)`. This only
guarantees RPC-vs-RPC ordering on that one channel; it does not promise
exclusivity with the strategy's own normal event processing. That's a
pre-existing property of the framework (strategies binding multiple
streams already need their own locking discipline), not a new gap this
feature introduces.

`Dispatch` recovers panics from the exported function and turns them into
a `CallResult.Err` instead of propagating — since the call now executes
inline inside the strategy's own event loop, a bug in one export must not
be able to take down the whole strategy.

### Reliability: bounded wait, bounded queue

- `Enqueue` never blocks: if the inbox is full (the strategy isn't
  draining fast enough), it returns `ErrInboxFull` immediately, which the
  gRPC layer maps to `codes.ResourceExhausted`.
- `Invoke` applies a server-side deadline (`context.WithTimeout` with a
  default if the caller didn't set one) and returns
  `codes.DeadlineExceeded` if the target instance never services the call
  in time, rather than hanging the caller forever.

### Wire format: generic JSON envelope, not per-method protobuf

```proto
service StrategyRPCService {
  rpc ListInstances(ListInstancesRequest) returns (ListInstancesResponse) {}
  rpc ListMethods(ListMethodsRequest) returns (ListMethodsResponse) {}
  rpc Invoke(InvokeRequest) returns (InvokeResponse) {}
}

message InvokeRequest {
  string instance_id = 1;
  string method = 2;
  string params_json = 3; // JSON array of positional parameters
}

message InvokeResponse {
  string result_json = 1;
  Error error = 2;
}
```

Parameters and results travel as a JSON-encoded string, not a per-method
generated protobuf message — exports are arbitrary strategy-author
functions, decided at runtime by `Export()` calls, not a fixed schema
known at `.proto` compile time. Lookup failures (unknown instance/method)
are reported through the response's `Error` field, matching every other
response in `bbgo.proto`; only transport-level concerns that don't fit
that shape (bad token, full inbox, timeout) use gRPC status codes.

### Auth and audit

A single shared bearer token (from `--strategy-rpc-token` / its
`STRATEGY_RPC_TOKEN` env equivalent via the existing viper
`AutomaticEnv` binding, or generated with `uuid.New()` and logged once at
startup if omitted) gates **every** RPC on `StrategyRPCService`, including
`ListInstances`/`ListMethods` — not just `Invoke`. `StrategyRPCInterceptor`
enforces this and audit-logs every call (instance, method, duration,
error) that reaches the service, successful or not. It's scoped by
`info.FullMethod` prefix, so `MarketDataService`/`TradingService`/
`UserDataService` remain unauthenticated and unaffected.

## Alternatives considered

- **Extend `pkg/interact` instead of a new API.** Rejected: its
  `ParseFuncArgsAndCall` converts shell-tokenized strings via `strconv`
  into `string`/`bool`/`int64`/`float64` positional args — built for a
  human typing into a chat window, not a structured RPC payload. Forcing
  gRPC calls through that shape would have been worse than a small amount
  of duplication.
- **Per-method generated protobuf messages**, instead of a generic JSON
  envelope. Rejected: would mean a `.proto` edit and `make grpc-go` run
  every time a strategy added or changed an export, which defeats the
  point of a strategy controlling its own exports dynamically.
- **Full mutual exclusion** between RPC calls and a strategy's normal
  callback dispatch (a shared lock wrapping every `OnKLineClosed`/
  `OnOrderUpdate`/etc. registration). Rejected for this change's scope:
  it's a legitimate but separate concurrency-safety gap that already
  exists independent of RPC export, and fixing it means touching
  `pkg/bbgo/session.go`'s dispatch plumbing for every stream type.
- **Per-instance YAML allowlist** on top of interface + flag + token.
  Rejected for v1: three independent gates (strategy code opts in, the
  operator passes `--enable-strategy-rpc`, the caller has the token) is
  already deliberate; a config-level allowlist is defense-in-depth that
  can be added later if it turns out to be needed.
- **Streaming/subscription-style exports.** Rejected: nothing in the
  motivating use case needed push updates, and it's a materially larger
  design surface (backpressure, connection lifecycle) to take on
  speculatively.

## Consequences

- Positive: strategies gain a self-describing, typed-enough-for-discovery
  way to expose both read and mutating actions without any protobuf
  codegen per export; the existing three gRPC services are completely
  unaffected (same auth posture, same behavior) when this feature is off.
- Positive: `rpcexport` has no dependency on `bbgo`/`grpc`, so it's
  reusable if a second transport (e.g. HTTP, or a `pkg/interact` adapter)
  is ever added on top of the same registry.
- Negative/accepted risk: a strategy that implements `RPCExporter` but
  never drains `Requests()` (e.g. forgets to add the worker) will cause
  every `Invoke` against it to fail with `DeadlineExceeded` rather than
  erroring immediately — there's no static check that a registered method
  is actually being serviced.
- Negative/accepted risk: because dispatch runs inline in the strategy's
  own loop and there is no cross-callback exclusivity, a mutating export
  can still race with the strategy's normal order/kline processing on
  strategies that don't already synchronize across their own streams.
  This is pre-existing framework behavior, not introduced by this
  feature, but RPC export is a new way to trigger it.

## Example: wiring a strategy for RPC export

This mirrors a demo that was prototyped against `pkg/strategy/xfundingv2`
and then reverted out of that strategy's real code, since a dummy `ping`
handler has no business living in a production strategy file. It's kept
here as the reference shape for the next strategy that wants this.

```go
// Strategy struct gains one field:
type Strategy struct {
    // ...
    rpcRegistry *rpcexport.Registry
}

// ExportRPC implements bbgo.RPCExporter.
func (s *Strategy) ExportRPC(reg *rpcexport.Registry) {
    s.rpcRegistry = reg

    reg.Export("ping", "demo/dummy RPC handler: returns \"pong\"", false, func() (string, error) {
        return "pong", nil
    })
}

// runRPCWorker drains the registry's inbox from the strategy's own
// goroutine, following the same ctx.Done()-select shape as this
// strategy's other background workers (e.g. runFundingIncomeWorker).
func (s *Strategy) runRPCWorker(ctx context.Context) {
    if s.rpcRegistry == nil {
        return
    }

    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            case call := <-s.rpcRegistry.Requests():
                s.rpcRegistry.Dispatch(call)
            }
        }
    }()
}

// Started alongside the strategy's other workers in CrossRun/Run:
//   s.runRPCWorker(s.ctx)
```

A method that actually mutates state (e.g. forcing a round to close early)
would follow the same shape, just with `mutating: true` and real logic in
place of `return "pong", nil`.

## References

- `pkg/rpcexport/registry.go` — the core registry.
- `pkg/bbgo/trader.go` — `RPCExporter`, `RPCRegistry`, `ListRPCInstances`.
- `pkg/pb/bbgo.proto` — `StrategyRPCService` and its messages.
- `pkg/grpc/strategyrpc.go`, `pkg/grpc/interceptor.go`, `pkg/grpc/server.go`
  — service implementation, auth/audit interceptor, wiring.
- `pkg/cmd/run.go` — `--enable-strategy-rpc`, `--strategy-rpc-token`.
- `doc/topics/grpc.md` — existing guide for connecting to this gRPC
  server with `evans`; the same approach works for `StrategyRPCService`
  once an instance's gRPC metadata includes `authorization: Bearer <token>`.
