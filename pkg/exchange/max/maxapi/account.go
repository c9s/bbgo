package maxapi

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type AccountService struct {
	client requestgen.AuthenticatedAPIClient
}

// Account is for max rest api v2, Balance and Type will be conflict with types.PrivateBalanceUpdate
type Account struct {
	Type     string           `json:"type"`
	Currency string           `json:"currency"`
	Balance  fixedpoint.Value `json:"balance"`
	Locked   fixedpoint.Value `json:"locked"`

	// v3 fields for M wallet
	Principal fixedpoint.Value `json:"principal"`
	Interest  fixedpoint.Value `json:"interest"`

	// v2 fields
	FiatCurrency string           `json:"fiat_currency"`
	FiatBalance  fixedpoint.Value `json:"fiat_balance"`
}

type UserBank struct {
	Branch  string `json:"branch"`
	Name    string `json:"name"`
	Account string `json:"account"`
	State   string `json:"state"`
}

type UserInfo struct {
	Email          string           `json:"email"`
	Level          int              `json:"level"`
	MWalletEnabled bool             `json:"m_wallet_enabled"`
	Current        VipLevelSettings `json:"current_vip_level"`
	Next           VipLevelSettings `json:"next_vip_level"`
}

type VipLevelSettings struct {
	Level                int     `json:"level"`
	MinimumTradingVolume float64 `json:"minimum_trading_volume"`
	MinimumStakingVolume float64 `json:"minimum_staking_volume"`
	MakerFee             float64 `json:"maker_fee"`
	TakerFee             float64 `json:"taker_fee"`
}

type VipLevel struct {
	Current VipLevelSettings `json:"current_vip_level"`
	Next    VipLevelSettings `json:"next_vip_level"`
}

//go:generate GetRequest -url "v2/members/vip_level" -type GetVipLevelRequest -responseType .VipLevel
type GetVipLevelRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetVipLevelRequest() *GetVipLevelRequest {
	return &GetVipLevelRequest{client: c}
}

//go:generate GetRequest -url "v2/members/accounts/:currency" -type GetAccountRequest -responseType .Account
type GetAccountRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency string `param:"currency,slug"`
}

func (c *RestClient) NewGetAccountRequest() *GetAccountRequest {
	return &GetAccountRequest{client: c}
}

//go:generate GetRequest -url "v2/members/accounts" -type GetAccountsRequest -responseType []Account
type GetAccountsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (c *RestClient) NewGetAccountsRequest() *GetAccountsRequest {
	return &GetAccountsRequest{client: c}
}

// submitted -> accepted -> processing -> sent -> confirmed
type WithdrawState string

const (
	WithdrawStateSubmitting WithdrawState = "submitting"
	WithdrawStateSubmitted  WithdrawState = "submitted"
	WithdrawStateConfirmed  WithdrawState = "confirmed"
	WithdrawStatePending    WithdrawState = "pending"
	WithdrawStateProcessing WithdrawState = "processing"
	WithdrawStateCanceled   WithdrawState = "canceled"
	WithdrawStateFailed     WithdrawState = "failed"
	WithdrawStateSent       WithdrawState = "sent"
	WithdrawStateRejected   WithdrawState = "rejected"
)

type WithdrawStatus string

const (
	WithdrawStatusPending   WithdrawStatus = "pending"
	WithdrawStatusCancelled WithdrawStatus = "cancelled"
	WithdrawStatusFailed    WithdrawStatus = "failed"
	WithdrawStatusOK        WithdrawStatus = "ok"
)

type Withdraw struct {
	UUID            string           `json:"uuid"`
	Currency        string           `json:"currency"`
	CurrencyVersion string           `json:"currency_version"` // "eth"
	Amount          fixedpoint.Value `json:"amount"`
	Fee             fixedpoint.Value `json:"fee"`
	FeeCurrency     string           `json:"fee_currency"`
	TxID            string           `json:"txid"`

	NetworkProtocol string `json:"network_protocol"`
	Address         string `json:"to_address"`

	// State can be "submitting", "submitted",
	//     "rejected", "accepted", "suspect", "approved", "delisted_processing",
	//     "processing", "retryable", "sent", "canceled",
	//     "failed", "pending", "confirmed",
	//     "kgi_manually_processing", "kgi_manually_confirmed", "kgi_possible_failed",
	//     "sygna_verifying"
	State WithdrawState `json:"state"`

	// Status WithdrawStatus `json:"status,omitempty"`

	CreatedAt types.MillisecondTimestamp `json:"created_at"`
	UpdatedAt types.MillisecondTimestamp `json:"updated_at"`
	Notes     string                     `json:"notes"`
}

//go:generate GetRequest -url "v3/withdrawals" -type GetWithdrawHistoryRequest -responseType []Withdraw
type GetWithdrawHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency  *string    `param:"currency"`
	state     *string    `param:"state"`                  // submitting, submitted, rejected, accepted, checking, refunded, canceled, suspect
	timestamp *time.Time `param:"timestamp,milliseconds"` // milli-seconds

	// order could be desc or asc
	order *string `param:"order"`

	// limit's default = 50
	limit *int `param:"limit"`
}

func (c *RestClient) NewGetWithdrawalHistoryRequest() *GetWithdrawHistoryRequest {
	return &GetWithdrawHistoryRequest{
		client: c,
	}
}
