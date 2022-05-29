package binanceapi

import "encoding/json"

type PagedDataResponse struct {
	Status string `json:"status"`
	Type   string `json:"type"`
	Code   string `json:"code"`
	Data   struct {
		Page         int             `json:"page"`
		TotalRecords int             `json:"totalRecords"`
		TotalPageNum int             `json:"totalPageNum"`
		Data         json.RawMessage `json:"data"`
	} `json:"data"`
}
