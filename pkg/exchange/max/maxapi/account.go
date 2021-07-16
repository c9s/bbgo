package max

import "context"

type AccountService struct {
	client *RestClient
}

// Account is for max rest api v2, Balance and Type will be conflict with types.PrivateBalanceUpdate
type Account struct {
	Currency string `json:"currency"`
	Balance  string `json:"balance"`
	Locked   string `json:"locked"`
	Type     string `json:"type"`
}

// Balance is for kingfisher
type Balance struct {
	Currency  string
	Available int64
	Locked    int64
	Total     int64
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

func (s *AccountService) VipLevel() (*VipLevel, error) {
	req, err := s.client.newAuthenticatedRequest("GET", "v2/members/vip_level", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var vipLevel VipLevel
	err = response.DecodeJSON(&vipLevel)
	if err != nil {
		return nil, err
	}

	return &vipLevel, nil
}

func (s *AccountService) Account(currency string) (*Account, error) {
	req, err := s.client.newAuthenticatedRequest("GET", "v2/members/accounts/"+currency, nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var account Account
	err = response.DecodeJSON(&account)
	if err != nil {
		return nil, err
	}

	return &account, nil
}

func (s *AccountService) NewGetWithdrawalHistoryRequest() *GetWithdrawHistoryRequest {
	return &GetWithdrawHistoryRequest{
		client: s.client,
	}
}

func (s *AccountService) Accounts() ([]Account, error) {
	req, err := s.client.newAuthenticatedRequest("GET", "v2/members/accounts", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var accounts []Account
	err = response.DecodeJSON(&accounts)
	if err != nil {
		return nil, err
	}

	return accounts, nil
}

// Me returns the current user info by the current used MAX key and secret
func (s *AccountService) Me() (*UserInfo, error) {
	req, err := s.client.newAuthenticatedRequest("GET", "v2/members/me", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var m = UserInfo{}
	err = response.DecodeJSON(&m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

type Deposit struct {
	Currency        string `json:"currency"`
	CurrencyVersion string `json:"currency_version"` // "eth"
	Amount          string `json:"amount"`
	Fee             string `json:"fee"`
	TxID            string `json:"txid"`
	State           string `json:"state"`
	Confirmations   int64  `json:"confirmations"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

type GetDepositHistoryRequestParams struct {
	*PrivateRequestParams

	Currency string `json:"currency,omitempty"`
	From     int64  `json:"from,omitempty"`  // seconds
	To       int64  `json:"to,omitempty"`    // seconds
	State    string `json:"state,omitempty"` // submitting, submitted, rejected, accepted, checking, refunded, canceled, suspect
	Limit    int    `json:"limit,omitempty"`
}

type GetDepositHistoryRequest struct {
	client *RestClient
	params GetDepositHistoryRequestParams
}

func (r *GetDepositHistoryRequest) State(state string) *GetDepositHistoryRequest {
	r.params.State = state
	return r
}

func (r *GetDepositHistoryRequest) Currency(currency string) *GetDepositHistoryRequest {
	r.params.Currency = currency
	return r
}

func (r *GetDepositHistoryRequest) Limit(limit int) *GetDepositHistoryRequest {
	r.params.Limit = limit
	return r
}

func (r *GetDepositHistoryRequest) From(from int64) *GetDepositHistoryRequest {
	r.params.From = from
	return r
}

func (r *GetDepositHistoryRequest) To(to int64) *GetDepositHistoryRequest {
	r.params.To = to
	return r
}

func (r *GetDepositHistoryRequest) Do(ctx context.Context) (deposits []Deposit, err error) {
	req, err := r.client.newAuthenticatedRequest("GET", "v2/deposits", &r.params, nil)
	if err != nil {
		return deposits, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return deposits, err
	}

	if err := response.DecodeJSON(&deposits); err != nil {
		return deposits, err
	}

	return deposits, err
}

func (s *AccountService) NewGetDepositHistoryRequest() *GetDepositHistoryRequest {
	return &GetDepositHistoryRequest{
		client: s.client,
	}
}

type Withdraw struct {
	UUID            string `json:"uuid"`
	Currency        string `json:"currency"`
	CurrencyVersion string `json:"currency_version"` // "eth"
	Amount          string `json:"amount"`
	Fee             string `json:"fee"`
	FeeCurrency     string `json:"fee_currency"`
	TxID            string `json:"txid"`

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

type GetWithdrawHistoryRequestParams struct {
	*PrivateRequestParams

	Currency string `json:"currency,omitempty"`
	From     int64  `json:"from,omitempty"`  // seconds
	To       int64  `json:"to,omitempty"`    // seconds
	State    string `json:"state,omitempty"` // submitting, submitted, rejected, accepted, checking, refunded, canceled, suspect
	Limit    int    `json:"limit,omitempty"`
}

type GetWithdrawHistoryRequest struct {
	client *RestClient
	params GetWithdrawHistoryRequestParams
}

func (r *GetWithdrawHistoryRequest) State(state string) *GetWithdrawHistoryRequest {
	r.params.State = state
	return r
}

func (r *GetWithdrawHistoryRequest) Currency(currency string) *GetWithdrawHistoryRequest {
	r.params.Currency = currency
	return r
}

func (r *GetWithdrawHistoryRequest) Limit(limit int) *GetWithdrawHistoryRequest {
	r.params.Limit = limit
	return r
}

func (r *GetWithdrawHistoryRequest) From(from int64) *GetWithdrawHistoryRequest {
	r.params.From = from
	return r
}

func (r *GetWithdrawHistoryRequest) To(to int64) *GetWithdrawHistoryRequest {
	r.params.To = to
	return r
}

func (r *GetWithdrawHistoryRequest) Do(ctx context.Context) (withdraws []Withdraw, err error) {
	req, err := r.client.newAuthenticatedRequest("GET", "v2/withdrawals", &r.params, nil)
	if err != nil {
		return withdraws, err
	}

	response, err := r.client.sendRequest(req)
	if err != nil {
		return withdraws, err
	}

	if err := response.DecodeJSON(&withdraws); err != nil {
		return withdraws, err
	}

	return withdraws, err
}
