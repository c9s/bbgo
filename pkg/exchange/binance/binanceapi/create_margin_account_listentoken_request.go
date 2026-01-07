package binanceapi

import (
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

// CreateMarginAccountListenTokenResponse represents the response from creating a margin account listen token
type CreateMarginAccountListenTokenResponse struct {
	Token          string                     `json:"token"`
	ExpirationTime types.MillisecondTimestamp `json:"expirationTime"`
}

//go:generate requestgen -method POST -url "/sapi/v1/userListenToken" -type CreateMarginAccountListenTokenRequest -responseType .CreateMarginAccountListenTokenResponse
type CreateMarginAccountListenTokenRequest struct {
	client requestgen.AuthenticatedAPIClient

	// Trading pair symbol; required when isIsolated is true, e.g., BNBUSDT
	symbol *string `param:"symbol"`

	// Whether it is isolated margin; true means isolated; default is cross margin
	isIsolated *bool `param:"isIsolated"`

	// Validity in milliseconds; default 24 hours, maximum 24 hours
	validity *int64 `param:"validity"`
}

func (c *RestClient) NewCreateMarginAccountListenTokenRequest() *CreateMarginAccountListenTokenRequest {
	return &CreateMarginAccountListenTokenRequest{client: c}
}
