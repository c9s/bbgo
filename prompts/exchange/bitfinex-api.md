# Bitfinex API Integration Guidelines for Go Client

We are integrating a new Bitfinex API endpoint into our Go client. Bitfinex’s
API responses often return JSON arrays instead of JSON objects, where each
array element has a specific meaning and position, corresponding to fields in a
struct.

Because of this, we cannot directly unmarshal the response using standard Go
structs. Instead, we use a helper function called `parseJsonArray()` to
manually map array elements to struct fields by position.

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

Please be aware that some JSON messages may have different structures, and you need to change the parsing logic according
to the specific message element.

For example, the second element is a "fiu" string, which indicates that the message is a funding info update message,
and the third element is an array that contains the funding info update array.

And in the second array, the "sym" string indicates that the update type is a symbol update, and the second element is
the symbol name, and the third element is an array that contains the funding info update array.

```json
[
  0, //CHAN_ID
  "fiu", //MSG_TYPE
  [
    "sym", //UPDATE_TYPE
    "fUSD",  //SYMBOL
    [
      0.0008595462068208099, //YIELD_LOAN
      0, //YIELD_LEND
      1.8444560185185186, //DURATION_LOAN
      0 //DURATION_LEND
    ] //FUNDING_INFO_UPDATE_ARRAY
  ] //UPDATE_ARRAY
]
```

For such case, you need to implement UnmarshalJSON method for the struct to handle the parsing logic:

- parse the message into []json.RawMessage
- check the element to determine the message type
- pass the rest json.RawMessage to the parseJsonArray function to parse the array elements into the struct fields.


## Array Message Parsing Helpers

For parsing JSON arrays into struct fields, refer to:

- `parser.go` in bfxapi package
- Functions: `parseJsonArray`, `parseRawArray`

If the JSON array is in the json.RawMessage format, you can use `parseJsonArray` to parse the array elements one by one into the struct fields.
If the JSON array is already parsed into a slice of json.RawMessage (meaning []json.RawMessage), you can use `parseRawArray` to parse the array elements one by one into the struct fields.

## Conventions

When returning error, please wrap the error with %w and use fmt.Errorf:

    return fmt.Errorf("failed to parse stream name: %w", err)

When designing the struct for each message, please respect the placeholder fields,
parseJsonArray, parseRawArray can map the array element one by one, so the placeholder fields matters.

When parsing numbers, please use fixedpoint.Value for precision:
`fixedpoint.Value` is defined in: `github.com/c9s/bbgo/pkg/fixedpoint`

When parsing millisecond time, please use `types.MillisecondTimestamp`
Use `types.MillisecondTimestamp` to convert from Unix timestamps, which can be found in `github.com/c9s/bbgo/pkg/types`.

When parsing websocket messages, please follow the below pattern, function name should start with "parse":

```go
// parseWalletSnapshot parses Bitfinex wallet snapshot message into []WalletResponse.
func parseWalletSnapshot(arrJson json.RawMessage) ([]WalletResponse, error) {
	var walletArrays []json.RawMessage

	if err := json.Unmarshal(arrJson, &walletArrays); err != nil {
		return nil, fmt.Errorf("failed to unmarshal wallet snapshot array: %w", err)
	}

	wallets := make([]WalletResponse, 0, len(walletArrays))
	for _, fields := range walletArrays {
		wallet, err := parseWalletUpdate(fields)
		if err != nil {
			log.WithError(err).Warnf("failed to parse wallet fields: %s", fields)
			continue
		}

		wallets = append(wallets, wallet)
	}
	return wallets, nil
}

// parseWalletUpdate parses Bitfinex wallet update message into WalletResponse.
func parseWalletUpdate(arrJson json.RawMessage) (WalletResponse, error) {
	var wallet WalletResponse
	if err := parseJsonArray(arrJson, &wallet, 0); err != nil {
		return WalletResponse{}, fmt.Errorf("failed to parse wallet update fields: %w", err)
	}

	return wallet, nil
}
```


## Convention for Adding a New Endpoint

1. **Define the request struct** using the naming format `{Verb}{Resource}Request`. Example: `GetTickerRequest`.
2. **File location**: Create a new file in `pkg/exchange/bitfinex/bfxapi`, using snake_case for the filename. Example: `get_ticker_request.go`.
3. **Request Struct**:

```go
type GetTickerRequest struct {
    client requestgen.APIClient
    symbol string `param:"symbol,slug"` // e.g. tBTCUSD
}
```

Parameters needs to be defined in lower case, and the `param` tag specifies how to format the parameter in the URL.

"slug" means the parameter will be formatted as a slug, which is typically used for identifiers in URLs (e.g., tBTCUSD).


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
    return parseJsonArray(data, r, 0)
}
```

- For the response struct field, please start with upper case letter to export the fields.
- For floating value please use fixedpoint.Value from `github.com/c9s/bbgo/pkg/fixedpoint` to handle precision correctly
- For optional fields or nullable fields, use pointer types. e.g., *int, *string, *fixedpoint.Value... etc.

7. **Add go:generate annotation** above the request definition for requestgen:

```go
//go:generate requestgen -type GetTickerRequest -method GET -url "/v2/ticker/:symbol" -responseType .TickerResponse
```

## Dependencies

- `fixedpoint.Value` is in: `github.com/c9s/bbgo/pkg/fixedpoint`
- `requestgen` package: `github.com/c9s/requestgen`
- For millisecond timestamps, use `types.MillisecondTimestamp` to convert from Unix timestamps, which can be found in `github.com/c9s/bbgo/pkg/types`.

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

