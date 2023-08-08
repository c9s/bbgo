package v3

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func (s *MarginService) NewMarginTransferRequest() *MarginTransferRequest {
	return &MarginTransferRequest{client: s.Client}
}

type MarginTransferSide string

const (
	MarginTransferSideIn  MarginTransferSide = "in"
	MarginTransferSideOut MarginTransferSide = "out"
)

type MarginTransferResponse struct {
	Sn        string                     `json:"sn"`
	Side      MarginTransferSide         `json:"side"`
	Currency  string                     `json:"currency"`
	Amount    fixedpoint.Value           `json:"amount"`
	CreatedAt types.MillisecondTimestamp `json:"created_at"`
	State     string                     `json:"state"`
}

//go:generate PostRequest -url "/api/v3/wallet/m/transfer" -type MarginTransferRequest -responseType .MarginTransferResponse
type MarginTransferRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency string             `param:"currency,required"`
	amount   string             `param:"amount"`
	side     MarginTransferSide `param:"side"`
}
