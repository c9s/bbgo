package binanceapi

import (
	"fmt"
	"time"

	"github.com/c9s/requestgen"
)

type AdlRiskLevel string

const (
	AdlRiskLevelLow           AdlRiskLevel = "LOW"
	AdlRiskLevelMiddle        AdlRiskLevel = "MIDDLE"
	AdlRiskLevelHigh          AdlRiskLevel = "HIGH"
	AdlRiskLevelExtremelyHigh AdlRiskLevel = "EXTREMELY_HIGH"
)

type AdlRisk struct {
	Symbol     string
	RiskLevel  AdlRiskLevel
	UpdateTime time.Time
}

func (r *AdlRisk) String() string {
	return fmt.Sprintf(
		"AdlRisk{Symbol: %s, AdlRisk: %s, UpdateTime: %s}",
		r.Symbol, r.RiskLevel, r.UpdateTime.Format(time.RFC3339),
	)
}

type RawAdlRisk struct {
	Symbol     string       `json:"symbol"`
	AdlRisk    AdlRiskLevel `json:"adlRisk"`
	UpdateTime int64        `json:"updateTime"`
}

func (r *RawAdlRisk) ToAdlRisk() *AdlRisk {
	return &AdlRisk{
		Symbol:     r.Symbol,
		RiskLevel:  r.AdlRisk,
		UpdateTime: time.UnixMilli(r.UpdateTime),
	}
}

func (r *RawAdlRisk) String() string {
	return fmt.Sprintf(
		"AdlRisk{Symbol: %s, AdlRisk: %s, UpdateTime: %d}",
		r.Symbol, r.AdlRisk, r.UpdateTime,
	)
}

type RawFuturesAdlRiskResponse []*RawAdlRisk

//go:generate requestgen -method GET -url "/fapi/v1/symbolAdlRisk" -type FuturesAdlRiskQuery -responseType RawFuturesAdlRiskResponse
type FuturesAdlRiskQuery struct {
	client requestgen.APIClient
}

//go:generate requestgen -method GET -url "/fapi/v1/symbolAdlRisk" -type FuturesSymbolAdlRiskQuery -responseType RawAdlRisk
type FuturesSymbolAdlRiskQuery struct {
	client requestgen.APIClient

	symbol string `param:"symbol,required"`
}

func (c *FuturesRestClient) NewFuturesAdlRiskQuery() *FuturesAdlRiskQuery {
	return &FuturesAdlRiskQuery{client: c}
}

func (c *FuturesRestClient) NewFuturesSymbolAdlRiskQuery(symbol string) *FuturesSymbolAdlRiskQuery {
	return &FuturesSymbolAdlRiskQuery{
		client: c,
		symbol: symbol,
	}
}
