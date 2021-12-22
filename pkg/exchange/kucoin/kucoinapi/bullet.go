package kucoinapi

import (
	"context"
	"net/http"
	"net/url"

	"github.com/c9s/bbgo/pkg/util"
)

// ApiClient defines the request builder method and request method for the API service
type ApiClient interface {
	// newAuthenticatedRequest builds up the http request for authentication-required endpoints
	newAuthenticatedRequest(method, refURL string, params url.Values, payload interface{}) (*http.Request, error)

	// newRequest builds up the http request for public endpoints
	newRequest(method, refURL string, params url.Values, body []byte) (*http.Request, error)

	// sendRequest sends the request object to the api gateway
	sendRequest(req *http.Request) (*util.Response, error)
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

func (r *GetPublicBulletRequest) Do(ctx context.Context) (*Bullet, error) {
	req, err := r.client.newRequest("POST", "/api/v1/bullet-public", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
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
	req, err := r.client.newAuthenticatedRequest("POST", "/api/v1/bullet-private", nil, nil)
	if err != nil {
		return nil, err
	}

	response, err := r.client.sendRequest(req)
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
