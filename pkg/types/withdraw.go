package types

import "time"

type Withdraw struct {
	ID         string  `json:"id"`
	Asset      string  `json:"asset"`
	Amount     float64 `json:"amount"`
	Address    string  `json:"address"`
	AddressTag string  `json:"addressTag"`
	Status     string  `json:"status"`

	TransactionID   string    `json:"txId"`
	TransactionFee  float64   `json:"transactionFee"`
	WithdrawOrderID string    `json:"withdrawOrderId"`
	ApplyTime       time.Time `json:"applyTime"`
	Network         string    `json:"network"`
}

func (w Withdraw) EffectiveTime() time.Time {
	return w.ApplyTime
}
