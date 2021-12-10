package kucoinapi


type AccountService struct {
	client *RestClient
}

type SubAccount struct {
	UserID string `json:"userId"`
	Name string `json:"subName"`
	Type string `json:"type"`
	Remark string `json:"remarks"`
}

func (s *AccountService) QuerySubAccounts() ([]SubAccount, error) {
	req, err := s.client.newAuthenticatedRequest("GET", "/api/v1/sub/user", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := s.client.sendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string       `json:"code"`
		Message string       `json:"msg"`
		Data    []SubAccount `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}