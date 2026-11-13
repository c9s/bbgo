# Exchange REST API Layer Guide (requestgen)

How bbgo builds the HTTP/REST half of an exchange adapter. Companion to
`doc/development/exchange-stream-guide.md` (the WebSocket half).

Reference implementations:

- `pkg/exchange/okex/okexapi` — canonical: response-wrapper + `-responseDataField`, `go:generate`
  command aliases, `validValues`, query/body split.
- `pkg/exchange/coinbase/api/v1` — bare responses (no wrapper), per-request rate limiters,
  slug path params, `timeFormat`.
- `pkg/exchange/binance/binanceapi` — largest: query-string HMAC/ed25519 signing, prometheus
  instrumentation, type aliases over a third-party SDK.

---

## 1. The two-layer rule

```
pkg/exchange/<name>/            ← ADAPTER layer: speaks types.Exchange, uses global bbgo types
   exchange.go                    QueryMarkets, SubmitOrder, QueryTrades, NewStream …
   convert.go                     toGlobalOrder / toLocalSymbol / toGlobalBalance …
   stream.go                      types.StandardStream implementation
   <name>api/                   ← API layer: speaks the exchange's own wire format ONLY
      client.go                   RestClient: base URL, auth, signing, response wrapper type
      constants.go                exchange-native enums (SideType, OrderType, OrderState …)
      <verb>_<resource>_request.go            hand-written: struct + go:generate directive
      <verb>_<resource>_request_requestgen.go GENERATED — never edit
```

**Hard boundary — this is the rule that matters most:**

| | may import | must NOT import |
|---|---|---|
| `<name>api` | `requestgen`, `fixedpoint`, `types.MillisecondTimestamp`/`types.Time`, `strint` | `pkg/bbgo`, `types.Order`, `types.Trade`, `types.Exchange`, other exchanges |
| `pkg/exchange/<name>` | its own `<name>api`, `pkg/types`, `pkg/fixedpoint`, `pkg/exchange/retry` | another exchange's api package |

The api package models **the exchange's API as documented** — same field names, same enum
values, same nesting. It never knows what a `types.Order` is. All translation happens in
`convert.go` one layer up. This is what makes the api package independently testable against
recorded JSON and what keeps exchange quirks from leaking into strategies.

`fixedpoint.Value` and `types.MillisecondTimestamp` are the sanctioned exceptions — they are
decoding primitives, not domain types.

Naming: the directory is `<name>api` with package name `<name>api`
(`okexapi`, `binanceapi`, `bybitapi`, `bfxapi`, `maxapi`). Coinbase's `api/v1` (package
`coinbase`) is a versioned outlier; for a new exchange prefer `backpackapi`.

---

## 2. What requestgen does

`requestgen` (github.com/c9s/requestgen, pinned at v1.6.0 in `go.mod`) reads a struct whose
unexported fields carry `param:"..."` tags, and generates a fluent request builder:

You write:

```go
//go:generate requestgen -method GET -url "/api/v3/myTrades" -type GetMyTradesRequest -responseType []Trade
type GetMyTradesRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol    string     `param:"symbol,required"`
	startTime *time.Time `param:"startTime,milliseconds"`
	limit     *uint64    `param:"limit"`
}

func (c *RestClient) NewGetMyTradesRequest() *GetMyTradesRequest {
	return &GetMyTradesRequest{client: c}
}
```

You get (in `*_requestgen.go`):

```go
func (g *GetMyTradesRequest) Symbol(symbol string) *GetMyTradesRequest       // chainable setter per field
func (g *GetMyTradesRequest) StartTime(t time.Time) *GetMyTradesRequest
func (g *GetMyTradesRequest) GetParameters() (map[string]interface{}, error) // validates required/validValues
func (g *GetMyTradesRequest) GetParametersQuery() (url.Values, error)
func (g *GetMyTradesRequest) GetQueryParameters() (url.Values, error)
func (g *GetMyTradesRequest) GetParametersJSON() ([]byte, error)
func (g *GetMyTradesRequest) GetSlugParameters() (map[string]interface{}, error)
func (g *GetMyTradesRequest) GetPath() string
func (g *GetMyTradesRequest) Do(ctx context.Context) ([]Trade, error)        // only if `client` field exists
```

Call site:

```go
trades, err := client.NewGetMyTradesRequest().
	Symbol("BTCUSDT").
	StartTime(since).
	Limit(1000).
	Do(ctx)
```

A slice field additionally gets an `AddXxx(item)` adder alongside the setter.

Run generation from the package directory and **commit the output**:

```bash
go generate ./pkg/exchange/<name>/...
```

`requestgen` must be on `PATH`: `go install github.com/c9s/requestgen/cmd/requestgen@v1.6.0`.

---

## 3. The RestClient

Embed `requestgen.BaseAPIClient` (gives you `NewRequest` + `SendRequest`), add credentials,
and implement `NewAuthenticatedRequest`. That single method is the entire auth contract.

```go
const defaultHTTPTimeout = 15 * time.Second
const RestBaseURL = "https://api.backpack.exchange"

var parsedBaseURL *url.URL

func init() {
	u, err := url.Parse(RestBaseURL)
	if err != nil { panic(err) }
	parsedBaseURL = u
}

type RestClient struct {
	requestgen.BaseAPIClient
	Key, Secret string
}

func NewClient() *RestClient {
	return &RestClient{
		BaseAPIClient: requestgen.BaseAPIClient{
			BaseURL:    parsedBaseURL,
			HttpClient: &http.Client{Timeout: defaultHTTPTimeout},
		},
	}
}

func (c *RestClient) Auth(key, secret string) {
	c.Key = key
	// pragma: allowlist nextline secret
	c.Secret = secret
}

// NewAuthenticatedRequest satisfies requestgen.AuthenticatedAPIClient.
func (c *RestClient) NewAuthenticatedRequest(
	ctx context.Context, method, refURL string, params url.Values, payload interface{},
) (*http.Request, error) {
	if len(c.Key) == 0   { return nil, errors.New("empty api key") }
	if len(c.Secret) == 0 { return nil, errors.New("empty api secret") }
	// ... build, sign, set headers, return
}
```

Public-only clients need no `NewAuthenticatedRequest`; declare the request's `client` field as
`requestgen.APIClient` instead. `Auth()` being separate from `NewClient()` matters: the factory
constructs public clients with empty credentials (`exchange.NewPublic`).

### Signing styles seen in the tree

| Exchange | Signed payload | Transport |
|---|---|---|
| okex | `timestamp + METHOD + path?query + body` , HMAC-SHA256, base64 | `OK-ACCESS-{KEY,SIGN,TIMESTAMP,PASSPHRASE}` headers, JSON body |
| coinbase | `timestamp + METHOD + requestURI + body` , HMAC-SHA256 over **base64-decoded** secret, base64 | `CB-ACCESS-{KEY,SIGN,TIMESTAMP,PASSPHRASE}` headers |
| binance | `rawQuery + body`, HMAC-SHA256 hex **or** ed25519 | `signature` appended to the query string, key in `X-MBX-APIKEY` |

Binance also injects `timestamp` and `recvWindow` into `params` inside
`NewAuthenticatedRequest` — put such universal parameters in the client, not in every request
struct.

### Response wrapper

If the exchange wraps every response in an envelope, model it once in `client.go` and give it
a `Validate() error`. The generated `Do()` calls `Validate()` (and an optional
`Unmarshal([]byte) error`) automatically when present:

```go
type APIResponse struct {
	Code    string          `json:"code"`
	Message string          `json:"msg"`
	Data    json.RawMessage `json:"data"`   // MUST be json.RawMessage for -responseDataField
}

func (a APIResponse) Validate() error {
	if a.Code != "0" {
		return a.Error()
	}
	return nil
}

func (a APIResponse) Error() error {
	return fmt.Errorf("retCode: %s, retMsg: %s, data: %s", a.Code, a.Message, string(a.Data))
}
```

That is how okex turns a `200 OK` carrying `{"code":"51000"}` into a Go error for free, on
every endpoint. HTTP-level errors (status >= 400) are surfaced by `BaseAPIClient.SendRequest`
as `*requestgen.ErrResponse`, which carries `.Response` and `.Body`.

---

## 4. The `go:generate` directive

### Command aliases

When every endpoint shares flags, define aliases at the top of each file (they are
**file-scoped** — repeat them in every file that uses them):

```go
//go:generate -command GetRequest  requestgen -method GET  -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

//go:generate GetRequest -url "/api/v5/trade/orders-history-archive" -type GetOrderHistoryRequest -responseDataType []OrderDetail
```

Without a wrapper, call `requestgen` directly (coinbase / binance style):

```go
//go:generate requestgen -method GET -url /orders -rateLimiter 1+20/2s -type GetOrdersRequest -responseType .OrderSnapshot
```

### Flags

| Flag | Meaning |
|---|---|
| `-type T` | request struct name (required) |
| `-method` | `GET` (default) / `POST` / `PUT` / `DELETE` |
| `-url` | endpoint path; `:name` segments are filled from `slug` params |
| `-responseType` | type `Do()` decodes into. Default `interface{}`. `.Foo` = `Foo` in this package; `[]Foo` and bare `Foo` also work |
| `-responseDataField F` | field of the wrapper holding the payload; **must be `json.RawMessage`** |
| `-responseDataType T` | type the raw data field decodes into; becomes `Do()`'s return type |
| `-rateLimiter L+N/D` | generates `var <Type>Limiter = rate.NewLimiter(...)` and `Limiter.Wait(ctx)` at the top of `Do()`. `1+20/2s` → burst 1, 20 events / 2s → `rate.NewLimiter(10, 1)` |
| `-sharedRateLimiterTypeName N` | use `NLimiter` (declared by another request) instead of a private one |
| `-parameterType map\|url` | how the body is built for non-GET; `map` (default) → JSON object |
| `-dynamicPath` | `Do()` calls your `GetDynamicPath() (string, error)` instead of `GetPath()` |
| `-debug`, `-stdout`, `-output`, `-tags` | development aids |

Return-type decision table:

| Exchange shape | Flags | `Do()` returns |
|---|---|---|
| `[{...},{...}]` | `-responseType []Trade` | `[]Trade` |
| `{...}` | `-responseType .Account` | `Account` |
| `{"code":..,"data":[...]}` | `-responseType .APIResponse -responseDataField Data -responseDataType []Order` | `[]Order` |
| `{"code":..,"rows":[...]}` (want the envelope's other fields too) | `-responseType .RowsResponse` | `RowsResponse` |

---

## 5. Field tags — exact semantics

```go
field <type> `param:"<wireName>[,opt...]" validValues:"a,b,c" default:"x" defaultValuer:"now()" timeFormat:"RFC3339Nano"`
```

Fields must be **unexported**; the setter is the exported name. The `client` field is special
and carries no tag.

### `param` options (the complete set in v1.6.0)

| Option | Effect |
|---|---|
| *(none)* | body param for non-GET; query param for GET |
| `required` | `GetParameters()` errors if unset; also drops the nil-check for pointers |
| `query` | force into the URL query string, even on POST/DELETE |
| `slug` | substituted into `:name` in the `-url` path, not sent as a parameter |
| `milliseconds` | `time.Time` → Unix ms integer |
| `seconds` | `time.Time` → Unix seconds integer |

`milliseconds`/`seconds` on a non-`time.Time` field is a generation error.
`omitempty` appears in upstream README examples but is **not** an option requestgen reads —
optionality comes from the pointer type.

### Optionality

**A pointer type means optional.** `*string` produces `Symbol(v string)` (still takes a value)
and is simply omitted from the parameter map when nil. A non-pointer field is always sent,
with its zero value if never set — so `limit int` sends `limit=0`. Use `*uint64` unless the
exchange requires the parameter.

### Other tags

| Tag | Effect |
|---|---|
| `validValues:"cross,isolated,cash"` | generates a `switch` that errors on anything else. For a named string/int type with declared constants, requestgen infers the valid set automatically — no tag needed |
| `default:"100"` | literal default applied when unset |
| `defaultValuer:"now()"` | computed default; only `now()`, `uuid()`, `method()` are supported |
| `timeFormat:"RFC3339Nano"` | format a `time.Time` as a string instead of an epoch number |

### The query/body split — read this before writing a GET

`Do()` picks its parameter source by template rules, and the interaction is a genuine footgun:

- **Any** field tagged `query` ⇒ the query string comes from `GetQueryParameters()`, which
  contains **only the `query`-tagged fields**.
- Otherwise, for `GET`, the query comes from `GetParametersQuery()` — every untagged field.
- For non-GET, untagged fields become the JSON body (`GetParameters()`); `query`-tagged fields
  go to the URL.

**Consequence: on a GET, tag either all params with `query` or none of them.** A mix silently
drops the untagged ones, because `GetParameters()` is not consulted for GET requests.
okex tags every field (`param:"instId,query"`); coinbase tags none. Both are correct;
half-and-half is a bug that compiles and generates cleanly.

---

## 6. Request file template

One resource per file, request struct + `New...Request` constructor + response structs
together:

```go
package backpackapi

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest  requestgen -method GET  -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

// Order mirrors the exchange's documented order object, field for field.
type Order struct {
	OrderID     string                     `json:"orderId"`
	Symbol      string                     `json:"symbol"`
	Side        SideType                   `json:"side"`
	OrderType   OrderType                  `json:"orderType"`
	Status      OrderStatus                `json:"status"`
	Price       fixedpoint.Value           `json:"price"`
	Quantity    fixedpoint.Value           `json:"quantity"`
	ExecutedQty fixedpoint.Value           `json:"executedQuantity"`
	CreatedAt   types.MillisecondTimestamp `json:"createdAt"`
}

//go:generate GetRequest -url "/api/v1/orders" -type GetOpenOrdersRequest -responseDataType []Order -rateLimiter 1+20/2s
type GetOpenOrdersRequest struct {
	client requestgen.AuthenticatedAPIClient

	symbol    *string    `param:"symbol,query"`
	startTime *time.Time `param:"from,query,milliseconds"`
	limit     *uint64    `param:"limit,query"`
}

func (c *RestClient) NewGetOpenOrdersRequest() *GetOpenOrdersRequest {
	return &GetOpenOrdersRequest{client: c}
}
```

Path parameter (`slug`):

```go
//go:generate requestgen -method DELETE -url "/api/v1/orders/:orderID" -type CancelOrderRequest -responseType .CancelOrderResponse
type CancelOrderRequest struct {
	client  requestgen.AuthenticatedAPIClient
	orderID string `param:"orderID,required,slug"`
}
```

Naming conventions, all load-bearing for consistency:

| Thing | Convention |
|---|---|
| Request type | `{Verb}{Resource}Request` — `GetOpenOrdersRequest`, `PlaceOrderRequest`, `CancelOrderRequest` |
| Constructor | `(c *RestClient) New{Verb}{Resource}Request(...) *{...}Request` |
| File | `snake_case` of the type minus `Request` → `get_open_orders_request.go` |
| Generated file | same + `_requestgen.go` |
| Enums | `constants.go`, named `SideType`/`OrderType`/`OrderState` with typed constants |

Give constructors the **required** parameters as arguments and leave optional ones to setters;
seed non-obvious defaults there too (okex's `NewPlaceOrderRequest` presets
`tradeMode: TradeModeCash`).

---

## 7. Decoding types

| Wire shape | Use |
|---|---|
| any number, incl. `"0.0012"` strings | `fixedpoint.Value` — never `float64` |
| epoch milliseconds (number or string) | `types.MillisecondTimestamp` |
| RFC3339 / mixed timestamp | `types.Time` |
| integer sent as a JSON string (`"12345"`) | `strint.Int64` (`pkg/types/strint`) |
| enum string | a named type in `constants.go`, so `validValues` is inferred |
| payload of a wrapper | `json.RawMessage` |

`fixedpoint.Value` parses ints, floats and numeric strings, and respects the `dnum` build tag.
A `float64` in an api struct is a review defect.

---

## 8. Rate limiting

Two layers, both in use:

1. **Per-request, generated** — `-rateLimiter 1+20/2s` emits a package-level
   `GetOrdersRequestLimiter` and a `Wait(ctx)` at the top of `Do()`. Prefer this: it is
   declarative and cannot be forgotten at a call site. Use `-sharedRateLimiterTypeName` when
   several endpoints share one exchange-side bucket.
2. **Hand-rolled in the adapter layer** — `pkg/exchange/okex/exchange.go` keeps
   `placeOrderLimiter`, `queryMarketLimiter` etc. and waits before calling `Do()`. Use this
   when the limit depends on adapter-level context (symbol counts, batch sizes):

```go
if err := placeOrderLimiter.Wait(ctx); err != nil {
	return nil, fmt.Errorf("place order rate limiter wait error: %w", err)
}
```

Always wrap the wait error with context — a bare `context deadline exceeded` from a limiter is
undebuggable.

---

## 9. Using the api layer from the adapter

`exchange.go` holds the client, translates global → native on the way in and native → global on
the way out, and never leaks a native type outward:

```go
func (e *Exchange) SubmitOrder(ctx context.Context, order types.SubmitOrder) (*types.Order, error) {
	req := e.client.NewPlaceOrderRequest()
	req.InstrumentID(e.getInstrumentId(order.Symbol))
	req.Side(toLocalSideType(order.Side))
	req.Size(order.Market.FormatQuantity(order.Quantity))   // market precision, always

	switch order.Type {
	case types.OrderTypeLimit, types.OrderTypeLimitMaker, types.OrderTypeStopLimit:
		req.Price(order.Market.FormatPrice(order.Price))
	}

	if err := placeOrderLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("place order rate limiter wait error: %w", err)
	}

	orderResponses, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}
	...
	return toGlobalOrder(&orderResponses[0])
}
```

Rules:

- Format quantities and prices with `order.Market.FormatQuantity` / `FormatPrice` — the api
  layer must receive already-rounded strings.
- Validate exchange-specific constraints (client-order-id charset, symbol format) in the
  adapter and return a descriptive error, rather than letting the exchange reject it.
- Retries and backoff belong in the adapter via `pkg/exchange/retry`, not in the api layer.
- Expose `GetClient() *<name>api.RestClient` only when strategies genuinely need raw access
  (okex does); it is an escape hatch, not the interface.
- Pagination loops (cursor / `after` / `before`) live in the adapter — see okex's `getTrades`
  helper — so the api layer stays a thin one-call-per-endpoint mapping.

---

## 10. Testing the api layer

**Integration tests** (`client_test.go`), skipped without credentials and in CI:

```go
func getTestClientOrSkip(t *testing.T) *RestClient {
	if b, _ := strconv.ParseBool(os.Getenv("CI")); b {
		t.Skip("skip test for CI")
	}
	key, secret, ok := testutil.IntegrationTestConfigured(t, "BACKPACK")   // env BACKPACK_API_KEY/SECRET
	if !ok {
		t.Skip("Please configure all credentials about BACKPACK")
		return nil
	}
	client := NewClient()
	client.Auth(key, secret)
	return client
}
```

Use `testutil.IntegrationTestWithPassphraseConfigured` for three-part credentials.

**Decode tests** — capture a real response into `<name>api/testdata/*.json` and assert the
struct decodes it (okexapi does this for its two history endpoints). These are the tests that
actually catch API drift, and they run in CI.

**HTTP mocking** — `pkg/testing/httptesting` provides `MockTransport` (per method/path
handlers) and a `Recorder` that captures live traffic to JSON, strips credential headers, and
replays it later under `TEST_HTTP_RECORD=1`. Prefer it over hand-written JSON when an endpoint
needs many cases.

---

## 11. Checklist for a new api package

1. `go install github.com/c9s/requestgen/cmd/requestgen@v1.6.0`.
2. `client.go`: base URL const(s), `RestClient` embedding `requestgen.BaseAPIClient`,
   `NewClient()`, `Auth()`, `NewAuthenticatedRequest()`, signing helper, response wrapper with
   `Validate()`.
3. `constants.go`: `SideType`, `OrderType`, `OrderStatus`, `TimeInForce`… as named types with
   constants, so `validValues` checks generate themselves.
4. One `*_request.go` per endpoint, in this order (the adapter needs them in roughly this
   sequence): markets/instruments → tickers → klines → balances → place order → cancel order →
   open orders → order history → trades.
5. `go generate ./pkg/exchange/<name>/...`, then `go build ./...`. Commit generated files.
6. `client_test.go` with skip-guards + `testdata/` decode tests.
7. Only then write `convert.go` and `exchange.go` on top.

## 12. Common mistakes

| Mistake | Symptom |
|---|---|
| Editing a `*_requestgen.go` file | silently reverted on the next `go generate` |
| `float64` instead of `fixedpoint.Value` | precision loss; breaks the `dnum` build |
| Exported field in a request struct | requestgen skips it — no setter, parameter never sent |
| Non-pointer optional field | parameter always sent with its zero value |
| Mixing `query`-tagged and untagged params on a GET | untagged params silently dropped |
| `-responseDataField` on a non-`json.RawMessage` field | generated code fails to compile |
| Importing `pkg/types.Order` in the api package | layering violation; makes the package untestable in isolation |
| Forgetting the `//go:generate -command` aliases in a new file | `GetRequest: command not found` during generate |
| Scattering endpoint constants | pick one home and stay there: okex keeps REST **and** ws URLs in `okexapi`; binance and coinbase keep ws URLs in the adapter and the REST base URL in the api package |
