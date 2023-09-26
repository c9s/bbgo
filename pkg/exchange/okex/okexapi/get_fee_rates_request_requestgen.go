// Code generated by "requestgen -method GET -responseType .APIResponse -responseDataField Data -url /api/v5/account/trade-fee -type GetFeeRatesRequest -responseDataType .FeeRateList"; DO NOT EDIT.

package okexapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
)

func (g *GetFeeRatesRequest) InstrumentType(instrumentType InstrumentType) *GetFeeRatesRequest {
	g.instrumentType = instrumentType
	return g
}

func (g *GetFeeRatesRequest) InstrumentID(instrumentID string) *GetFeeRatesRequest {
	g.instrumentID = &instrumentID
	return g
}

func (g *GetFeeRatesRequest) Underlying(underlying string) *GetFeeRatesRequest {
	g.underlying = &underlying
	return g
}

func (g *GetFeeRatesRequest) InstrumentFamily(instrumentFamily string) *GetFeeRatesRequest {
	g.instrumentFamily = &instrumentFamily
	return g
}

// GetQueryParameters builds and checks the query parameters and returns url.Values
func (g *GetFeeRatesRequest) GetQueryParameters() (url.Values, error) {
	var params = map[string]interface{}{}
	// check instrumentType field -> json key instType
	instrumentType := g.instrumentType

	// TEMPLATE check-valid-values
	switch instrumentType {
	case InstrumentTypeSpot, InstrumentTypeSwap, InstrumentTypeFutures, InstrumentTypeOption, InstrumentTypeMARGIN:
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

	query := url.Values{}
	for _k, _v := range params {
		query.Add(_k, fmt.Sprintf("%v", _v))
	}

	return query, nil
}

// GetParameters builds and checks the parameters and return the result in a map object
func (g *GetFeeRatesRequest) GetParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}

	return params, nil
}

// GetParametersQuery converts the parameters from GetParameters into the url.Values format
func (g *GetFeeRatesRequest) GetParametersQuery() (url.Values, error) {
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
func (g *GetFeeRatesRequest) GetParametersJSON() ([]byte, error) {
	params, err := g.GetParameters()
	if err != nil {
		return nil, err
	}

	return json.Marshal(params)
}

// GetSlugParameters builds and checks the slug parameters and return the result in a map object
func (g *GetFeeRatesRequest) GetSlugParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}

	return params, nil
}

func (g *GetFeeRatesRequest) applySlugsToUrl(url string, slugs map[string]string) string {
	for _k, _v := range slugs {
		needleRE := regexp.MustCompile(":" + _k + "\\b")
		url = needleRE.ReplaceAllString(url, _v)
	}

	return url
}

func (g *GetFeeRatesRequest) iterateSlice(slice interface{}, _f func(it interface{})) {
	sliceValue := reflect.ValueOf(slice)
	for _i := 0; _i < sliceValue.Len(); _i++ {
		it := sliceValue.Index(_i).Interface()
		_f(it)
	}
}

func (g *GetFeeRatesRequest) isVarSlice(_v interface{}) bool {
	rt := reflect.TypeOf(_v)
	switch rt.Kind() {
	case reflect.Slice:
		return true
	}
	return false
}

func (g *GetFeeRatesRequest) GetSlugsMap() (map[string]string, error) {
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

func (g *GetFeeRatesRequest) Do(ctx context.Context) (FeeRateList, error) {

	// no body params
	var params interface{}
	query, err := g.GetQueryParameters()
	if err != nil {
		return nil, err
	}

	apiURL := "/api/v5/account/trade-fee"

	req, err := g.client.NewAuthenticatedRequest(ctx, "GET", apiURL, query, params)
	if err != nil {
		return nil, err
	}

	response, err := g.client.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var apiResponse APIResponse
	if err := response.DecodeJSON(&apiResponse); err != nil {
		return nil, err
	}
	var data FeeRateList
	if err := json.Unmarshal(apiResponse.Data, &data); err != nil {
		return nil, err
	}
	return data, nil
}
