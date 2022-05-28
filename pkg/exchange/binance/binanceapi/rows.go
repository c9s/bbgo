package binanceapi

import "encoding/json"

type RowsResponse struct {
	Rows  json.RawMessage `json:"rows"`
	Total int             `json:"total"`
}
