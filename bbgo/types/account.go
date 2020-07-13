package types

type Balance struct {
	Currency string
	Available float64
	Locked float64
}


type Account struct {
	MakerCommission int64
	TakerCommission int64
	AccountType string
	Balances map[string]Balance
}
