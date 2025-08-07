# Bitfinex API Integration Guidelines for Go Client

We are integrating a new Bitfinex API endpoint into our Go client. Bitfinex’s API responses often return JSON arrays instead of JSON objects, where each array element has a specific meaning and position, corresponding to fields in a struct.

Because of this, we cannot directly unmarshal the response using standard Go structs. Instead, we use a helper function called `parseJsonArray()` to manually map array elements to struct fields by position.

### Example of such response:

```json
[
    "exchange",          // TYPE
    "UST",               // CURRENCY
    19788.6529257,       // BALANCE
    0,                   // UNSETTLED_INTEREST
    19788.6529257        // AVAILABLE_BALANCE
]
```

## Convention for Adding a New Endpoint

1. **Define the request struct** using the naming format `{Verb}{Resource}Request`. Example: `GetTickerRequest`.
2. **File location**: Create a new file in `pkg/exchange/bitfinex`, using snake_case for the filename. Example: `get_ticker_request.go`.
3. **Request Struct**:

```go
type GetTickerRequest struct {
    client requestgen.APIClient
    symbol string `param:"symbol,slug"` // e.g. tBTCUSD
}
```

Parameters needs to be defined in lower case, and the `param` tag specifies how to format the parameter in the URL.

4. **Client method** to create the request:

```go
func (c *Client) NewGetTickerRequest() *GetTickerRequest {
    return &GetTickerRequest{client: c}
}
```

5. **Execute method** using `client.Do` or similar logic. Use:
    - `requestgen.APIClient` for public endpoints
    - `requestgen.AuthenticatedAPIClient` for authenticated endpoints

6. **Define the response struct** matching the array positions:

```go
type TickerResponse struct {
    Bid     fixedpoint.Value
    BidSize fixedpoint.Value
    Ask     fixedpoint.Value
    AskSize fixedpoint.Value
    DailyChange         fixedpoint.Value
    DailyChangeRelative fixedpoint.Value
    LastPrice           fixedpoint.Value
    Volume fixedpoint.Value
    High   fixedpoint.Value
    Low    fixedpoint.Value
}

func (r *TickerResponse) UnmarshalJSON(data []byte) error {
    return parseJsonArray(data, r)
}
```

7. **Add go:generate annotation** above the request definition for requestgen:

```go
//go:generate requestgen -type GetTickerRequest -method GET -url "/v2/ticker/:symbol" -responseType .TickerResponse
```

## Dependencies

- `fixedpoint.Value` is in: `github.com/c9s/bbgo/pkg/fixedpoint`
- `requestgen` package: `github.com/c9s/requestgen`

## Parsing Helpers

For parsing JSON arrays into struct fields, refer to:

- `parser.go` in bfxapi package
- Functions: `parseJsonArray`, `parseRawArray`

---

## Integration Task

Please implement the following API based on the above pattern.

https://docs.bitfinex.com/reference/rest-auth-wallets

### Response Format (Example from Docs):

```json
[
  [
    "exchange",         // TYPE
    "BTC",              // CURRENCY
    0.1,                // BALANCE
    0,                  // UNSETTLED_INTEREST
    0.1,                // BALANCE_AVAILABLE
    null,               // PLACEHOLDER1
    null,               // PLACEHOLDER2
    0                   // LAST_CHANGE
  ]
]
```

### Suggested Request Name

```go
GetWalletsRequest
```

