package kucoinapi

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"

	"github.com/c9s/bbgo/pkg/util"
)

// ApiClient defines the request builder method and request method for the API service
type ApiClient interface {
	// NewAuthenticatedRequest builds up the http request for authentication-required endpoints
	NewAuthenticatedRequest(method, refURL string, params url.Values, payload interface{}) (*http.Request, error)

	// NewRequest builds up the http request for public endpoints
	NewRequest(method, refURL string, params url.Values, payload []byte) (*http.Request, error)

	// SendRequest sends the request object to the api gateway
	SendRequest(req *http.Request) (*util.Response, error)
}

type BulletService struct {
	client *RestClient
}

func (s *BulletService) NewGetPublicBulletRequest() *GetPublicBulletRequest {
	return &GetPublicBulletRequest{client: s.client}
}

func (s *BulletService) NewGetPrivateBulletRequest() *GetPrivateBulletRequest {
	return &GetPrivateBulletRequest{client: s.client}
}

//go:generate requestgen -type GetPublicBulletRequest
type GetPublicBulletRequest struct {
	client ApiClient
}

type Bullet struct {
	InstanceServers []struct {
		Endpoint     string `json:"endpoint"`
		Protocol     string `json:"protocol"`
		Encrypt      bool   `json:"encrypt"`
		PingInterval int    `json:"pingInterval"`
		PingTimeout  int    `json:"pingTimeout"`
	} `json:"instanceServers"`
	Token string `json:"token"`
}

func (b *Bullet) PingInterval() time.Duration {
	return time.Duration(b.InstanceServers[0].PingInterval) * time.Millisecond
}

func (b *Bullet) PingTimeout() time.Duration {
	return time.Duration(b.InstanceServers[0].PingTimeout) * time.Millisecond
}

func (b *Bullet) URL() (*url.URL, error) {
	if len(b.InstanceServers) == 0 {
		return nil, errors.New("InstanceServers is empty")
	}

	u, err := url.Parse(b.InstanceServers[0].Endpoint)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Add("token", b.Token)

	u.RawQuery = params.Encode()
	return u, nil
}

func (r *GetPublicBulletRequest) Do(ctx context.Context) (*Bullet, error) {
	req, err := r.client.NewRequest("POST", "/api/v1/bullet-public", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string  `json:"code"`
		Message string  `json:"msg"`
		Data    *Bullet `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}

//go:generate requestgen -type GetPrivateBulletRequest
type GetPrivateBulletRequest struct {
	client ApiClient
}

func (r *GetPrivateBulletRequest) Do(ctx context.Context) (*Bullet, error) {
	req, err := r.client.NewAuthenticatedRequest("POST", "/api/v1/bullet-private", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse struct {
		Code    string  `json:"code"`
		Message string  `json:"msg"`
		Data    *Bullet `json:"data"`
	}

	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}

	return apiResponse.Data, nil
}
