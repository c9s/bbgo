package coinbase

import (
	"encoding/json"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/requestgen"
)

type TransferType string

const (
	TransferTypeDeposit          TransferType = "deposit"
	TransferTypeWithdraw         TransferType = "withdraw"
	TransferTypeInternalDeposit  TransferType = "internal_deposit"
	TransferTypeInternalWithdraw TransferType = "internal_withdraw"
)

// Coinbase use a custom time format for transfers
// We handle it with a custom type TransferTime
type TransferTime time.Time

const transferTimeFormat = "2006-01-02 15:04:05.999999Z07"

func (t TransferTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).Format(transferTimeFormat))
}

func (t *TransferTime) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if len(str) == 0 {
		*t = TransferTime(time.Time{})
		return nil
	}
	parsedTime, err := time.Parse(transferTimeFormat, str)
	if err != nil {
		return err
	}

	*t = TransferTime(parsedTime)
	return nil
}

func (t TransferTime) Time() time.Time {
	return time.Time(t)
}

type Transfer struct {
	ID   string       `json:"id"`
	Type TransferType `json:"type"`
	// the timeformat is not RFC3339, so we have to use string
	CreatedAt   TransferTime     `json:"created_at"`
	CompletedAt TransferTime     `json:"completed_at"`
	CanceledAt  TransferTime     `json:"canceled_at"`
	ProcessedAt TransferTime     `json:"processed_at"`
	Amount      fixedpoint.Value `json:"amount"`
	Details     struct {
		Network              string           `json:"network"`
		Fee                  fixedpoint.Value `json:"fee"`
		SendToAddress        string           `json:"sent_to_address"`
		CryptoAddress        string           `json:"crypto_address"`
		CryptoTransctionHash string           `json:"crypto_transaction_hash"`

		Data json.RawMessage `json:"data"`
	} `json:"details"`
	UserNonce string `json:"user_nonce"`
	Currency  string `json:"currency"`
}

type GetTransfersResponse []Transfer

//go:generate requestgen -method GET -url /transfers -rateLimiter 1+20/2s -type GetTransfersRequest -responseType .GetTransfersResponse
type GetTransfersRequest struct {
	client requestgen.AuthenticatedAPIClient

	profileID    *string       `param:"profile_id"`
	before       *time.Time    `param:"before" timeFormat:"2006-01-02"` // date, inclusive
	after        *time.Time    `param:"after" timeFormat:"2006-01-02"`  // date, exclusive
	limit        *int          `param:"limit"`
	transferType *TransferType `param:"type"`
	currency     *string       `param:"currency"`
}

func (client *RestAPIClient) NewGetTransfersRequest() *GetTransfersRequest {
	return &GetTransfersRequest{
		client: client,
	}
}
