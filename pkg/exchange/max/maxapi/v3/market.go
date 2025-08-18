package v3

import "github.com/c9s/bbgo/pkg/fixedpoint"

type Market struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Status             string           `json:"market_status"` // active
	BaseUnit           string           `json:"base_unit"`
	BaseUnitPrecision  int              `json:"base_unit_precision"`
	QuoteUnit          string           `json:"quote_unit"`
	QuoteUnitPrecision int              `json:"quote_unit_precision"`
	MinBaseAmount      fixedpoint.Value `json:"min_base_amount"`
	MinQuoteAmount     fixedpoint.Value `json:"min_quote_amount"`
	SupportMargin      bool             `json:"m_wallet_supported"`
}
