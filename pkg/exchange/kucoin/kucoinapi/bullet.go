package kucoinapi

import (
	"net/url"
	"time"

	"github.com/c9s/requestgen"
	"github.com/pkg/errors"
)

type BulletService struct {
	client *RestClient
}

func (s *BulletService) NewGetPublicBulletRequest() *GetPublicBulletRequest {
	return &GetPublicBulletRequest{client: s.client}
}

func (s *BulletService) NewGetPrivateBulletRequest() *GetPrivateBulletRequest {
	return &GetPrivateBulletRequest{client: s.client}
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

//go:generate requestgen -type GetPublicBulletRequest -method "POST" -url "/api/v1/bullet-public" -responseType .APIResponse -responseDataField Data -responseDataType .Bullet
type GetPublicBulletRequest struct {
	client requestgen.APIClient
}

//go:generate requestgen -type GetPrivateBulletRequest -method "POST" -url "/api/v1/bullet-private" -responseType .APIResponse -responseDataField Data -responseDataType .Bullet
type GetPrivateBulletRequest struct {
	client requestgen.AuthenticatedAPIClient
}
