package ftx

import "time"

type balances struct {
	Success bool `json:"success"`

	Result []struct {
		Coin  string  `json:"coin"`
		Free  float64 `json:"free"`
		Total float64 `json:"total"`
	} `json:"result"`
}

type ordersHistoryResponse struct {
	Success     bool    `json:"success"`
	Result      []order `json:"result"`
	HasMoreData bool    `json:"hasMoreData"`
}

type ordersResponse struct {
	Success bool `json:"success"`

	Result []order `json:"result"`
}

type cancelOrderResponse struct {
	Success bool   `json:"success"`
	Result  string `json:"result"`
}

type order struct {
	CreatedAt  time.Time `json:"createdAt"`
	FilledSize float64   `json:"filledSize"`
	// Future field is not defined in the response format table but in the response example.
	Future        string  `json:"future"`
	ID            int64   `json:"id"`
	Market        string  `json:"market"`
	Price         float64 `json:"price"`
	AvgFillPrice  float64 `json:"avgFillPrice"`
	RemainingSize float64 `json:"remainingSize"`
	Side          string  `json:"side"`
	Size          float64 `json:"size"`
	Status        string  `json:"status"`
	Type          string  `json:"type"`
	ReduceOnly    bool    `json:"reduceOnly"`
	Ioc           bool    `json:"ioc"`
	PostOnly      bool    `json:"postOnly"`
	ClientId      string  `json:"clientId"`
}

type orderResponse struct {
	Success bool `json:"success"`

	Result order `json:"result"`
}
