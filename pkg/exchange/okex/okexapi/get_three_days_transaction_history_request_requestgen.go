// Code generated by "requestgen -method GET -responseType .APIResponse -responseDataField Data -url /api/v5/trade/fills -type GetThreeDaysTransactionHistoryRequest -responseDataType []Trade -rateLimiter 1+30/2s"; DO NOT EDIT.

package okexapi

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/time/rate"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"time"
)

var GetThreeDaysTransactionHistoryRequestLimiter = rate.NewLimiter(15.000000150000002, 1)

func (g *GetThreeDaysTransactionHistoryRequest) InstrumentType(instrumentType InstrumentType) *GetThreeDaysTransactionHistoryRequest {
	g.instrumentType = instrumentType
	return g
}

func (g *GetThreeDaysTransactionHistoryRequest) InstrumentID(instrumentID string) *GetThreeDaysTransactionHistoryRequest {
	g.instrumentID = &instrumentID
	return g
}

func (g *GetThreeDaysTransactionHistoryRequest) OrderID(orderID string) *GetThreeDaysTransactionHistoryRequest {
	g.orderID = &orderID
	return g
}

func (g *GetThreeDaysTransactionHistoryRequest) Underlying(underlying string) *GetThreeDaysTransactionHistoryRequest {
	g.underlying = &underlying
	return g
}

func (g *GetThreeDaysTransactionHistoryRequest) InstrumentFamily(instrumentFamily string) *GetThreeDaysTransactionHistoryRequest {
	g.instrumentFamily = &instrumentFamily
	return g
}

func (g *GetThreeDaysTransactionHistoryRequest) After(after string) *GetThreeDaysTransactionHistoryRequest {
	g.after = &after
	return g
}

func (g *GetThreeDaysTransactionHistoryRequest) Before(before string) *GetThreeDaysTransactionHistoryRequest {
	g.before = &before
	return g
}

func (g *GetThreeDaysTransactionHistoryRequest) StartTime(startTime time.Time) *GetThreeDaysTransactionHistoryRequest {
	g.startTime = &startTime
	return g
}

func (g *GetThreeDaysTransactionHistoryRequest) EndTime(endTime time.Time) *GetThreeDaysTransactionHistoryRequest {
	g.endTime = &endTime
	return g
}

func (g *GetThreeDaysTransactionHistoryRequest) Limit(limit uint64) *GetThreeDaysTransactionHistoryRequest {
	g.limit = &limit
	return g
}

// GetQueryParameters builds and checks the query parameters and returns url.Values
func (g *GetThreeDaysTransactionHistoryRequest) GetQueryParameters() (url.Values, error) {
	var params = map[string]interface{}{}
	// check instrumentType field -> json key instType
	instrumentType := g.instrumentType

	// TEMPLATE check-valid-values
	switch instrumentType {
	case InstrumentTypeSpot, InstrumentTypeSwap, InstrumentTypeFutures, InstrumentTypeOption, InstrumentTypeMargin:
		params["instType"] = instrumentType

	default:
		return nil, fmt.Errorf("instType value %v is invalid", instrumentType)

	}
	// END TEMPLATE check-valid-values

	// assign parameter of instrumentType
	params["instType"] = instrumentType
	// check instrumentID field -> json key instId
	if g.instrumentID != nil {
		instrumentID := *g.instrumentID

		// assign parameter of instrumentID
		params["instId"] = instrumentID
	} else {
	}
	// check orderID field -> json key ordId
	if g.orderID != nil {
		orderID := *g.orderID

		// assign parameter of orderID
		params["ordId"] = orderID
	} else {
	}
	// check underlying field -> json key uly
	if g.underlying != nil {
		underlying := *g.underlying

		// assign parameter of underlying
		params["uly"] = underlying
	} else {
	}
	// check instrumentFamily field -> json key instFamily
	if g.instrumentFamily != nil {
		instrumentFamily := *g.instrumentFamily

		// assign parameter of instrumentFamily
		params["instFamily"] = instrumentFamily
	} else {
	}
	// check after field -> json key after
	if g.after != nil {
		after := *g.after

		// assign parameter of after
		params["after"] = after
	} else {
	}
	// check before field -> json key before
	if g.before != nil {
		before := *g.before

		// assign parameter of before
		params["before"] = before
	} else {
	}
	// check startTime field -> json key begin
	if g.startTime != nil {
		startTime := *g.startTime

		// assign parameter of startTime
		// convert time.Time to milliseconds time stamp
		params["begin"] = strconv.FormatInt(startTime.UnixNano()/int64(time.Millisecond), 10)
	} else {
	}
	// check endTime field -> json key end
	if g.endTime != nil {
		endTime := *g.endTime

		// assign parameter of endTime
		// convert time.Time to milliseconds time stamp
		params["end"] = strconv.FormatInt(endTime.UnixNano()/int64(time.Millisecond), 10)
	} else {
	}
	// check limit field -> json key limit
	if g.limit != nil {
		limit := *g.limit

		// assign parameter of limit
		params["limit"] = limit
	} else {
	}

	query := url.Values{}
	for _k, _v := range params {
		query.Add(_k, fmt.Sprintf("%v", _v))
	}

	return query, nil
}

// GetParameters builds and checks the parameters and return the result in a map object
func (g *GetThreeDaysTransactionHistoryRequest) GetParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}

	return params, nil
}

// GetParametersQuery converts the parameters from GetParameters into the url.Values format
func (g *GetThreeDaysTransactionHistoryRequest) GetParametersQuery() (url.Values, error) {
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
func (g *GetThreeDaysTransactionHistoryRequest) GetParametersJSON() ([]byte, error) {
	params, err := g.GetParameters()
	if err != nil {
		return nil, err
	}

	return json.Marshal(params)
}

// GetSlugParameters builds and checks the slug parameters and return the result in a map object
func (g *GetThreeDaysTransactionHistoryRequest) GetSlugParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}

	return params, nil
}

func (g *GetThreeDaysTransactionHistoryRequest) applySlugsToUrl(url string, slugs map[string]string) string {
	for _k, _v := range slugs {
		needleRE := regexp.MustCompile(":" + _k + "\\b")
		url = needleRE.ReplaceAllString(url, _v)
	}

	return url
}

func (g *GetThreeDaysTransactionHistoryRequest) iterateSlice(slice interface{}, _f func(it interface{})) {
	sliceValue := reflect.ValueOf(slice)
	for _i := 0; _i < sliceValue.Len(); _i++ {
		it := sliceValue.Index(_i).Interface()
		_f(it)
	}
}

func (g *GetThreeDaysTransactionHistoryRequest) isVarSlice(_v interface{}) bool {
	rt := reflect.TypeOf(_v)
	switch rt.Kind() {
	case reflect.Slice:
		return true
	}
	return false
}

func (g *GetThreeDaysTransactionHistoryRequest) GetSlugsMap() (map[string]string, error) {
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
func (g *GetThreeDaysTransactionHistoryRequest) GetPath() string {
	return "/api/v5/trade/fills"
}

// Do generates the request object and send the request object to the API endpoint
func (g *GetThreeDaysTransactionHistoryRequest) Do(ctx context.Context) ([]Trade, error) {
	if err := GetThreeDaysTransactionHistoryRequestLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	// no body params
	var params interface{}
	query, err := g.GetQueryParameters()
	if err != nil {
		return nil, err
	}

	var apiURL string

	apiURL = g.GetPath()

	req, err := g.client.NewAuthenticatedRequest(ctx, "GET", apiURL, query, params)
	if err != nil {
		return nil, err
	}

	response, err := g.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse

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
	var data []Trade
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}
	return data, nil
}
