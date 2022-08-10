package max

//go:generate -command GetRequest requestgen -method GET
//go:generate -command PostRequest requestgen -method POST
//go:generate -command DeleteRequest requestgen -method DELETE

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
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
	Debt      fixedpoint.Value `json:"debt"`
	Principal fixedpoint.Value `json:"principal"`
	Borrowed  fixedpoint.Value `json:"borrowed"`
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
	Sn              string    `json:"sn"`
	Name            string    `json:"name"`
	Type            string    `json:"member_type"`
	Level           int       `json:"level"`
	VipLevel        int       `json:"vip_level"`
	Email           string    `json:"email"`
	Accounts        []Account `json:"accounts"`
	Bank            *UserBank `json:"bank,omitempty"`
	IsFrozen        bool      `json:"is_frozen"`
	IsActivated     bool      `json:"is_activated"`
	KycApproved     bool      `json:"kyc_approved"`
	KycState        string    `json:"kyc_state"`
	PhoneSet        bool      `json:"phone_set"`
	PhoneNumber     string    `json:"phone_number"`
	ProfileVerified bool      `json:"profile_verified"`
	CountryCode     string    `json:"country_code"`
	IdentityNumber  string    `json:"identity_number"`
	WithDrawable    bool      `json:"withdrawable"`
	ReferralCode    string    `json:"referral_code"`
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

func (s *AccountService) NewGetVipLevelRequest() *GetVipLevelRequest {
	return &GetVipLevelRequest{client: s.client}
}

//go:generate GetRequest -url "v2/members/accounts/:currency" -type GetAccountRequest -responseType .Account
type GetAccountRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency string `param:"currency,slug"`
}

func (s *AccountService) NewGetAccountRequest() *GetAccountRequest {
	return &GetAccountRequest{client: s.client}
}

//go:generate GetRequest -url "v2/members/accounts" -type GetAccountsRequest -responseType []Account
type GetAccountsRequest struct {
	client requestgen.AuthenticatedAPIClient
}

func (s *AccountService) NewGetAccountsRequest() *GetAccountsRequest {
	return &GetAccountsRequest{client: s.client}
}

type Deposit struct {
	Currency        string           `json:"currency"`
	CurrencyVersion string           `json:"currency_version"` // "eth"
	Amount          fixedpoint.Value `json:"amount"`
	Fee             fixedpoint.Value `json:"fee"`
	TxID            string           `json:"txid"`
	State           string           `json:"state"`
	Confirmations   int64            `json:"confirmations"`
	CreatedAt       int64            `json:"created_at"`
	UpdatedAt       int64            `json:"updated_at"`
}

//go:generate GetRequest -url "v2/deposits" -type GetDepositHistoryRequest -responseType []Deposit
type GetDepositHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency *string `param:"currency"`
	from     *int64  `param:"from"`  // seconds
	to       *int64  `param:"to"`    // seconds
	state    *string `param:"state"` // submitting, submitted, rejected, accepted, checking, refunded, canceled, suspect
	limit    *int    `param:"limit"`
}

func (s *AccountService) NewGetDepositHistoryRequest() *GetDepositHistoryRequest {
	return &GetDepositHistoryRequest{
		client: s.client,
	}
}

type Withdraw struct {
	UUID            string           `json:"uuid"`
	Currency        string           `json:"currency"`
	CurrencyVersion string           `json:"currency_version"` // "eth"
	Amount          fixedpoint.Value `json:"amount"`
	Fee             fixedpoint.Value `json:"fee"`
	FeeCurrency     string           `json:"fee_currency"`
	TxID            string           `json:"txid"`

	// State can be "submitting", "submitted",
	//     "rejected", "accepted", "suspect", "approved", "delisted_processing",
	//     "processing", "retryable", "sent", "canceled",
	//     "failed", "pending", "confirmed",
	//     "kgi_manually_processing", "kgi_manually_confirmed", "kgi_possible_failed",
	//     "sygna_verifying"
	State         string `json:"state"`
	Confirmations int    `json:"confirmations"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
	Notes         string `json:"notes"`
}

//go:generate GetRequest -url "v2/withdrawals" -type GetWithdrawHistoryRequest -responseType []Withdraw
type GetWithdrawHistoryRequest struct {
	client requestgen.AuthenticatedAPIClient

	currency string  `param:"currency"`
	from     *int64  `param:"from"`  // seconds
	to       *int64  `param:"to"`    // seconds
	state    *string `param:"state"` // submitting, submitted, rejected, accepted, checking, refunded, canceled, suspect
	limit    *int    `param:"limit"`
}

func (s *AccountService) NewGetWithdrawalHistoryRequest() *GetWithdrawHistoryRequest {
	return &GetWithdrawHistoryRequest{
		client: s.client,
	}
}
