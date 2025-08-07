We are integrating a new Bitfinex API endpoint into our Go client. Bitfinex’s API responses often return JSON arrays
instead of JSON objects, where each array element has a specific meaning and position, corresponding to fields in a
struct.

Because of this, we cannot directly unmarshal the response using standard Go structs. Instead, we use a helper function
called parseJsonArray() to manually map array elements to struct fields by position.

Example of such response:

```json
[
    "exchange",          // TYPE
    "UST",               // CURRENCY
    19788.6529257,       // BALANCE
    0,                   // UNSETTLED_INTEREST
    19788.6529257        // AVAILABLE_BALANCE
]
```

To add a new API endpoint, we follow these conventions:

* Define the request name in the format: `{Verb}{Resource}Request`, using the API name as a prefix, like: `GetTickerRequest`, `PlaceOrderRequest`, etc.
* Open a new file in the `pkg/exchange/bitfinex` directory, named after the request with snake case (e.g., `get_ticker_request.go`).
* Define a request struct with the request name (e.g. `GetTickerRequest`).

```go
type GetTickerRequest struct {
    client requestgen.APIClient
    symbol string `param:"symbol,slug"` // e.g. tBTCUSD
}

```

* Add a new method to the Client struct to create a new instance of the request, with the request name as a method name:

```go
func (c *Client) NewGetTickerRequest() *GetTickerRequest {
    return &GetTickerRequest{client: c}
}
```

* Implement a method on the request struct to execute the request, using the `client` field to make the API call:

For public API endpoint, please use:

        client requestgen.APIClient

For private API endpoint that needs authentication, please use:

        client requestgen.AuthenticatedAPIClient

* Define a response struct, with the corresponding fields in the order they appear in the JSON array:

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

* Annotate the request with go:generate, so the requestgen tool can generate the necessary client logic:

```
//go:generate requestgen -type GetTickerRequest -method GET -url "/v2/ticker/:symbol" -responseType .TickerResponse
```

You can find fixedpoint.Value in the following import path:

    "github.com/c9s/bbgo/pkg/fixedpoint"

And here is the requestgen package path:

    "github.com/c9s/requestgen"

---
Now, please help me integrate the following API endpoint using the above pattern:

Bitfinex Authenticated Wallets Endpoint Doc:
https://docs.bitfinex.com/reference/rest-auth-wallets

Please use the response format defined in the API doc to implement the fields of response struct.


