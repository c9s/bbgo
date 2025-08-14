package bfxapi

import (
	"github.com/c9s/requestgen"
)

//go:generate requestgen -type GetDepositAddressRequest -method POST -url "/v2/auth/w/deposit/address" -responseType .DepositAddressResponse

// GetDepositAddressRequest represents a request for deposit address.
type GetDepositAddressRequest struct {
	client requestgen.AuthenticatedAPIClient

	// wallet - Select the wallet from which to transfer (exchange, margin, funding (can also use the old labels which are exchange, trading and deposit respectively))
	wallet string `param:"wallet"`

	// method - method of deposit (methods accepted: “bitcoin”, “litecoin”, “ethereum”, ...) For an up-to-date listing of supported currencies see: https://api-pub.bitfinex.com//v2/conf/pub:map:tx:method
	method string `param:"method"`

	// opRenew - Input 1 to generate a new deposit address (old addresses remain valid). Defaults to 0 if omitted, which will return the existing deposit address
	opRenew *int `param:"op_renew"` // optional, 1 to renew
}

// NewGetDepositAddressRequest creates a new deposit address request.
func (c *Client) NewGetDepositAddressRequest() *GetDepositAddressRequest {
	return &GetDepositAddressRequest{client: c}
}

// DepositAddress represents the nested address array in the response.
type DepositAddress struct {
	_ any

	Method       string
	CurrencyCode string

	_ any

	Address     string
	PoolAddress *string
}

func (r *DepositAddress) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}

// DepositAddressResponse represents the response for deposit address.
type DepositAddressResponse struct {
	MTS          int64
	Type         string
	MessageID    *string
	Placeholder  *string
	AddressArray DepositAddress
	Code         *string
	Status       string
	Text         string
}

// UnmarshalJSON parses the JSON array response into DepositAddressResponse.
func (r *DepositAddressResponse) UnmarshalJSON(data []byte) error {
	return parseJsonArray(data, r, 0)
}
