package types

type Balance struct {
	Currency string `json:"currency"`
	Available float64 `json:"available"`
	Locked float64 `json:"locked"`
}


type Account struct {
	MakerCommission int64
	TakerCommission int64
	AccountType string
	Balances map[string]Balance
}
