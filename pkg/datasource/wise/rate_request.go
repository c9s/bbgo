package wise

import (
	"time"

	"github.com/c9s/requestgen"
)

// https://docs.wise.com/api-docs/api-reference/rate

//go:generate requestgen -method GET -url "/v1/rates" -type RateRequest -responseType []Rate
type RateRequest struct {
	client requestgen.AuthenticatedAPIClient

	source string     `param:"source"`
	target string     `param:"target"`
	time   *time.Time `param:"time" timeFormat:"2006-01-02T15:04:05-0700"`
	from   *time.Time `param:"from" timeFormat:"2006-01-02T15:04:05-0700"`
	to     *time.Time `param:"to" timeFormat:"2006-01-02T15:04:05-0700"`
	group  *Group     `param:"group"`
}

func (c *Client) NewRateRequest() *RateRequest {
	return &RateRequest{
		client: c,
	}
}
