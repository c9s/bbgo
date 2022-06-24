package glassnodeapi

import (
	"time"

	"github.com/c9s/requestgen"
)

//go:generate requestgen -method GET -type Request -url "/v1/metrics/:category/:metric" -responseType DataSlice
type Request struct {
	Client requestgen.AuthenticatedAPIClient

	Asset           string     `param:"a,required,query"`
	Since           *time.Time `param:"s,query,seconds"`
	Until           *time.Time `param:"u,query,seconds"`
	Interval        *Interval  `param:"i,query"`
	Format          *Format    `param:"f,query" default:"JSON"`
	Currency        *string    `param:"c,query"`
	TimestampFormat *string    `param:"timestamp_format,query"`

	Category string `param:"category,slug"`
	Metric   string `param:"metric,slug"`
}
