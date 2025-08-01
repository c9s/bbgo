// Code generated by "requestgen -method GET -url /api/v3/wallet/:walletType/orders/closed -type GetWalletClosedOrdersRequest -responseType []Order"; DO NOT EDIT.

package v3

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"time"
)

func (g *GetWalletClosedOrdersRequest) Market(market string) *GetWalletClosedOrdersRequest {
	g.market = market
	return g
}

func (g *GetWalletClosedOrdersRequest) Timestamp(timestamp time.Time) *GetWalletClosedOrdersRequest {
	g.timestamp = &timestamp
	return g
}

func (g *GetWalletClosedOrdersRequest) OrderBy(orderBy max.OrderByType) *GetWalletClosedOrdersRequest {
	g.orderBy = &orderBy
	return g
}

func (g *GetWalletClosedOrdersRequest) Limit(limit uint) *GetWalletClosedOrdersRequest {
	g.limit = &limit
	return g
}

func (g *GetWalletClosedOrdersRequest) WalletType(walletType max.WalletType) *GetWalletClosedOrdersRequest {
	g.walletType = walletType
	return g
}

// GetQueryParameters builds and checks the query parameters and returns url.Values
func (g *GetWalletClosedOrdersRequest) GetQueryParameters() (url.Values, error) {
	var params = map[string]interface{}{}

	query := url.Values{}
	for _k, _v := range params {
		query.Add(_k, fmt.Sprintf("%v", _v))
	}

	return query, nil
}

// GetParameters builds and checks the parameters and return the result in a map object
func (g *GetWalletClosedOrdersRequest) GetParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}
	// check market field -> json key market
	market := g.market

	// TEMPLATE check-required
	if len(market) == 0 {
		return nil, fmt.Errorf("market is required, empty string given")
	}
	// END TEMPLATE check-required

	// assign parameter of market
	params["market"] = market
	// check timestamp field -> json key timestamp
	if g.timestamp != nil {
		timestamp := *g.timestamp

		// assign parameter of timestamp
		// convert time.Time to milliseconds time stamp
		params["timestamp"] = strconv.FormatInt(timestamp.UnixNano()/int64(time.Millisecond), 10)
	} else {
	}
	// check orderBy field -> json key order_by
	if g.orderBy != nil {
		orderBy := *g.orderBy

		// assign parameter of orderBy
		params["order_by"] = orderBy
	} else {
	}
	// check limit field -> json key limit
	if g.limit != nil {
		limit := *g.limit

		// assign parameter of limit
		params["limit"] = limit
	} else {
	}

	return params, nil
}

// GetParametersQuery converts the parameters from GetParameters into the url.Values format
func (g *GetWalletClosedOrdersRequest) GetParametersQuery() (url.Values, error) {
	query := url.Values{}

	params, err := g.GetParameters()
	if err != nil {
		return query, err
	}

	for _k, _v := range params {
		if g.isVarSlice(_v) {
			g.iterateSlice(_v, func(it interface{}) {
				query.Add(_k+"[]", fmt.Sprintf("%v", it))
			})
		} else {
			query.Add(_k, fmt.Sprintf("%v", _v))
		}
	}

	return query, nil
}

// GetParametersJSON converts the parameters from GetParameters into the JSON format
func (g *GetWalletClosedOrdersRequest) GetParametersJSON() ([]byte, error) {
	params, err := g.GetParameters()
	if err != nil {
		return nil, err
	}

	return json.Marshal(params)
}

// GetSlugParameters builds and checks the slug parameters and return the result in a map object
func (g *GetWalletClosedOrdersRequest) GetSlugParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}
	// check walletType field -> json key walletType
	walletType := g.walletType

	// TEMPLATE check-required
	if len(walletType) == 0 {
		return nil, fmt.Errorf("walletType is required, empty string given")
	}
	// END TEMPLATE check-required

	// assign parameter of walletType
	params["walletType"] = walletType

	return params, nil
}

func (g *GetWalletClosedOrdersRequest) applySlugsToUrl(url string, slugs map[string]string) string {
	for _k, _v := range slugs {
		needleRE := regexp.MustCompile(":" + _k + "\\b")
		url = needleRE.ReplaceAllString(url, _v)
	}

	return url
}

func (g *GetWalletClosedOrdersRequest) iterateSlice(slice interface{}, _f func(it interface{})) {
	sliceValue := reflect.ValueOf(slice)
	for _i := 0; _i < sliceValue.Len(); _i++ {
		it := sliceValue.Index(_i).Interface()
		_f(it)
	}
}

func (g *GetWalletClosedOrdersRequest) isVarSlice(_v interface{}) bool {
	rt := reflect.TypeOf(_v)
	switch rt.Kind() {
	case reflect.Slice:
		return true
	}
	return false
}

func (g *GetWalletClosedOrdersRequest) GetSlugsMap() (map[string]string, error) {
	slugs := map[string]string{}
	params, err := g.GetSlugParameters()
	if err != nil {
		return slugs, nil
	}

	for _k, _v := range params {
		slugs[_k] = fmt.Sprintf("%v", _v)
	}

	return slugs, nil
}

// GetPath returns the request path of the API
func (g *GetWalletClosedOrdersRequest) GetPath() string {
	return "/api/v3/wallet/:walletType/orders/closed"
}

// Do generates the request object and send the request object to the API endpoint
func (g *GetWalletClosedOrdersRequest) Do(ctx context.Context) ([]max.Order, error) {

	// empty params for GET operation
	var params interface{}
	query, err := g.GetParametersQuery()
	if err != nil {
		return nil, err
	}

	var apiURL string

	apiURL = g.GetPath()
	slugs, err := g.GetSlugsMap()
	if err != nil {
		return nil, err
	}

	apiURL = g.applySlugsToUrl(apiURL, slugs)

	req, err := g.client.NewAuthenticatedRequest(ctx, "GET", apiURL, query, params)
	if err != nil {
		return nil, err
	}

	response, err := g.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse []max.Order

	type responseUnmarshaler interface {
		Unmarshal(data []byte) error
	}

	if unmarshaler, ok := interface{}(&apiResponse).(responseUnmarshaler); ok {
		if err := unmarshaler.Unmarshal(response.Body); err != nil {
			return nil, err
		}
	} else {
		// The line below checks the content type, however, some API server might not send the correct content type header,
		// Hence, this is commented for backward compatibility
		// response.IsJSON()
		if err := response.DecodeJSON(&apiResponse); err != nil {
			return nil, err
		}
	}

	type responseValidator interface {
		Validate() error
	}

	if validator, ok := interface{}(&apiResponse).(responseValidator); ok {
		if err := validator.Validate(); err != nil {
			return nil, err
		}
	}
	return apiResponse, nil
}
