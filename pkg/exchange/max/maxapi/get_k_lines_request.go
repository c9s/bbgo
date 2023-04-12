package max

import (
	"time"

	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

type KLineData []float64

//go:generate GetRequest -url "/api/v2/k" -type GetKLinesRequest -responseType []KLineData
type GetKLinesRequest struct {
	client requestgen.APIClient

	market    string    `param:"market,required"`
	limit     *int      `param:"limit"`
	period    *int      `param:"period"`
	timestamp time.Time `param:"timestamp,seconds"`
}

func (c *RestClient) NewGetKLinesRequest() *GetKLinesRequest {
	return &GetKLinesRequest{client: c}
}
