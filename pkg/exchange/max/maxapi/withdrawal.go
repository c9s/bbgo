package max

import (
	"context"
	"errors"
)

/*
	example response

	{
	  "uuid": "18022603540001",
	  "currency": "eth",
	  "currency_version": "eth",
	  "amount": "0.019",
	  "fee": "0.0",
	  "fee_currency": "eth",
	  "created_at": 1521726960,
	  "updated_at": 1521726960,
	  "state": "confirmed",
	  "type": "external",
	  "transaction_type": "external send",
	  "notes": "notes",
	  "sender": {
		"email": "max****@maicoin.com"
	  },
	  "recipient": {
		"address": "0x5c7d23d516f120d322fc7b116386b7e491739138"
	  }
	}
*/

type WithdrawalRequest struct {
	client      *RestClient
	addressUUID string
	currency    string
	amount      float64
}

func (r *WithdrawalRequest) Currency(currency string) *WithdrawalRequest {
	r.currency = currency
	return r
}

func (r *WithdrawalRequest) AddressUUID(uuid string) *WithdrawalRequest {
	r.addressUUID = uuid
	return r
}

func (r *WithdrawalRequest) Amount(amount float64) *WithdrawalRequest {
	r.amount = amount
	return r
}

func (r *WithdrawalRequest) Do(ctx context.Context) (*Withdraw, error) {
	if r.currency == "" {
		return nil, errors.New("currency field is required")
	}

	if r.addressUUID == "" {
		return nil, errors.New("withdraw_address_uuid field is required")
	}

	if r.amount <= 0 {
		return nil, errors.New("amount is required")
	}

	payload := map[string]interface{}{
		"currency":              r.currency,
		"withdraw_address_uuid": r.addressUUID,
		"amount":                r.amount,
	}

	req, err := r.client.newAuthenticatedRequest("POST", "v2/withdrawal", payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var resp Withdraw
	if err := response.DecodeJSON(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

type WithdrawalAddress struct {
	UUID            string `json:"uuid"`
	Currency        string `json:"currency"`
	CurrencyVersion string `json:"currency_version"`
	Address         string `json:"address"`
	ExtraLabel      string `json:"extra_label"`
	State           string `json:"state"`
	SygnaVaspCode   string `json:"sygna_vasp_code"`
	SygnaUserType   string `json:"sygna_user_type"`
	SygnaUserCode   string `json:"sygna_user_code"`
	IsInternal      bool   `json:"is_internal"`
}

type GetWithdrawalAddressesRequest struct {
	client   *RestClient
	currency string
}

func (r *GetWithdrawalAddressesRequest) Currency(currency string) *GetWithdrawalAddressesRequest {
	r.currency = currency
	return r
}

func (r *GetWithdrawalAddressesRequest) Do(ctx context.Context) ([]WithdrawalAddress, error) {
	if r.currency == "" {
		return nil, errors.New("currency field is required")
	}

	payload := map[string]interface{}{
		"currency": r.currency,
	}

	req, err := r.client.newAuthenticatedRequest("GET", "v2/withdraw_addresses", payload)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var addresses []WithdrawalAddress
	if err := response.DecodeJSON(&addresses); err != nil {
		return nil, err
	}

	return addresses, nil
}

type WithdrawalService struct {
	client *RestClient
}

func (s *WithdrawalService) NewGetWithdrawalAddressesRequest() *GetWithdrawalAddressesRequest {
	return &GetWithdrawalAddressesRequest{
		client: s.client,
	}
}

func (s *WithdrawalService) NewWithdrawalRequest() *WithdrawalRequest {
	return &WithdrawalRequest{client: s.client}
}

func (s *WithdrawalService) NewGetWithdrawalHistoryRequest() *GetWithdrawHistoryRequest {
	return &GetWithdrawHistoryRequest{
		client: s.client,
	}
}
