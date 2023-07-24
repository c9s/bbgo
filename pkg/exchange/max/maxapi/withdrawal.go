package max

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST

import (
	"github.com/c9s/requestgen"
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

//go:generate PostRequest -url "v2/withdrawal" -type WithdrawalRequest -responseType .Withdraw
type WithdrawalRequest struct {
	client requestgen.AuthenticatedAPIClient

	addressUUID string  `param:"withdraw_address_uuid,required"`
	currency    string  `param:"currency,required"`
	amount      float64 `param:"amount"`
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

//go:generate GetRequest -url "v2/withdraw_addresses" -type GetWithdrawalAddressesRequest -responseType []WithdrawalAddress
type GetWithdrawalAddressesRequest struct {
	client   requestgen.AuthenticatedAPIClient
	currency string `param:"currency,required"`
}

type WithdrawalService struct {
	client requestgen.AuthenticatedAPIClient
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
