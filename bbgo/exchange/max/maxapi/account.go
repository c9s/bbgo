package max

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

func (s *AccountService) Account(currency string) (*Account, error) {
	req, err := s.client.newAuthenticatedRequest("GET", "v2/members/accounts/"+currency, nil)
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

func (s *AccountService) Accounts() ([]Account, error) {
	req, err := s.client.newAuthenticatedRequest("GET", "v2/members/accounts", nil)
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
	req, err := s.client.newAuthenticatedRequest("GET", "v2/members/me", nil)
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
