package ftx

type balances struct {
	Success bool `json:"Success"`

	Result []struct {
		Coin  string  `json:"coin"`
		Free  float64 `json:"free"`
		Total float64 `json:"total"`
	} `json:"result"`
}
