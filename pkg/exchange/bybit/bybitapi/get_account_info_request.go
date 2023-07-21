package bybitapi

import "github.com/c9s/requestgen"

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type AccountInfo struct {
	MarginMode          string `json:"marginMode"`
	UpdatedTime         string `json:"updatedTime"`
	UnifiedMarginStatus int    `json:"unifiedMarginStatus"`
	DcpStatus           string `json:"dcpStatus"`
	TimeWindow          int    `json:"timeWindow"`
	SmpGroup            int    `json:"smpGroup"`
}

//go:generate GetRequest -url "/v5/account/info" -type GetAccountInfoRequest -responseDataType .AccountInfo
type GetAccountInfoRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetAccountRequest() *GetAccountInfoRequest {
	return &GetAccountInfoRequest{client: c}
}
