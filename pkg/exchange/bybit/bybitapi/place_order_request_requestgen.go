// Code generated by "requestgen -method POST -responseType .APIResponse -responseDataField Result -url /v5/order/create -type PlaceOrderRequest -responseDataType .PlaceOrderResponse -rateLimiter 5+15/1s"; DO NOT EDIT.

package bybitapi

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/time/rate"
	"net/url"
	"reflect"
	"regexp"
)

var PlaceOrderRequestLimiter = rate.NewLimiter(15.000000150000002, 5)

func (p *PlaceOrderRequest) Category(category Category) *PlaceOrderRequest {
	p.category = category
	return p
}

func (p *PlaceOrderRequest) Symbol(symbol string) *PlaceOrderRequest {
	p.symbol = symbol
	return p
}

func (p *PlaceOrderRequest) Side(side Side) *PlaceOrderRequest {
	p.side = side
	return p
}

func (p *PlaceOrderRequest) OrderType(orderType OrderType) *PlaceOrderRequest {
	p.orderType = orderType
	return p
}

func (p *PlaceOrderRequest) Qty(qty string) *PlaceOrderRequest {
	p.qty = qty
	return p
}

func (p *PlaceOrderRequest) OrderLinkId(orderLinkId string) *PlaceOrderRequest {
	p.orderLinkId = orderLinkId
	return p
}

func (p *PlaceOrderRequest) TimeInForce(timeInForce TimeInForce) *PlaceOrderRequest {
	p.timeInForce = timeInForce
	return p
}

func (p *PlaceOrderRequest) MarketUnit(marketUnit MarketUnit) *PlaceOrderRequest {
	p.marketUnit = &marketUnit
	return p
}

func (p *PlaceOrderRequest) IsLeverage(isLeverage bool) *PlaceOrderRequest {
	p.isLeverage = &isLeverage
	return p
}

func (p *PlaceOrderRequest) Price(price string) *PlaceOrderRequest {
	p.price = &price
	return p
}

func (p *PlaceOrderRequest) TriggerDirection(triggerDirection int) *PlaceOrderRequest {
	p.triggerDirection = &triggerDirection
	return p
}

func (p *PlaceOrderRequest) OrderFilter(orderFilter string) *PlaceOrderRequest {
	p.orderFilter = &orderFilter
	return p
}

func (p *PlaceOrderRequest) TriggerPrice(triggerPrice string) *PlaceOrderRequest {
	p.triggerPrice = &triggerPrice
	return p
}

func (p *PlaceOrderRequest) TriggerBy(triggerBy string) *PlaceOrderRequest {
	p.triggerBy = &triggerBy
	return p
}

func (p *PlaceOrderRequest) OrderIv(orderIv string) *PlaceOrderRequest {
	p.orderIv = &orderIv
	return p
}

func (p *PlaceOrderRequest) PositionIdx(positionIdx string) *PlaceOrderRequest {
	p.positionIdx = &positionIdx
	return p
}

func (p *PlaceOrderRequest) TakeProfit(takeProfit string) *PlaceOrderRequest {
	p.takeProfit = &takeProfit
	return p
}

func (p *PlaceOrderRequest) StopLoss(stopLoss string) *PlaceOrderRequest {
	p.stopLoss = &stopLoss
	return p
}

func (p *PlaceOrderRequest) TpTriggerBy(tpTriggerBy string) *PlaceOrderRequest {
	p.tpTriggerBy = &tpTriggerBy
	return p
}

func (p *PlaceOrderRequest) SlTriggerBy(slTriggerBy string) *PlaceOrderRequest {
	p.slTriggerBy = &slTriggerBy
	return p
}

func (p *PlaceOrderRequest) ReduceOnly(reduceOnly bool) *PlaceOrderRequest {
	p.reduceOnly = &reduceOnly
	return p
}

func (p *PlaceOrderRequest) CloseOnTrigger(closeOnTrigger bool) *PlaceOrderRequest {
	p.closeOnTrigger = &closeOnTrigger
	return p
}

func (p *PlaceOrderRequest) SmpType(smpType string) *PlaceOrderRequest {
	p.smpType = &smpType
	return p
}

func (p *PlaceOrderRequest) Mmp(mmp bool) *PlaceOrderRequest {
	p.mmp = &mmp
	return p
}

func (p *PlaceOrderRequest) TpslMode(tpslMode string) *PlaceOrderRequest {
	p.tpslMode = &tpslMode
	return p
}

func (p *PlaceOrderRequest) TpLimitPrice(tpLimitPrice string) *PlaceOrderRequest {
	p.tpLimitPrice = &tpLimitPrice
	return p
}

func (p *PlaceOrderRequest) SlLimitPrice(slLimitPrice string) *PlaceOrderRequest {
	p.slLimitPrice = &slLimitPrice
	return p
}

func (p *PlaceOrderRequest) TpOrderType(tpOrderType string) *PlaceOrderRequest {
	p.tpOrderType = &tpOrderType
	return p
}

func (p *PlaceOrderRequest) SlOrderType(slOrderType string) *PlaceOrderRequest {
	p.slOrderType = &slOrderType
	return p
}

// GetQueryParameters builds and checks the query parameters and returns url.Values
func (p *PlaceOrderRequest) GetQueryParameters() (url.Values, error) {
	var params = map[string]interface{}{}

	query := url.Values{}
	for _k, _v := range params {
		query.Add(_k, fmt.Sprintf("%v", _v))
	}

	return query, nil
}

// GetParameters builds and checks the parameters and return the result in a map object
func (p *PlaceOrderRequest) GetParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}
	// check category field -> json key category
	category := p.category

	// TEMPLATE check-valid-values
	switch category {
	case "spot":
		params["category"] = category

	default:
		return nil, fmt.Errorf("category value %v is invalid", category)

	}
	// END TEMPLATE check-valid-values

	// assign parameter of category
	params["category"] = category
	// check symbol field -> json key symbol
	symbol := p.symbol

	// assign parameter of symbol
	params["symbol"] = symbol
	// check side field -> json key side
	side := p.side

	// TEMPLATE check-valid-values
	switch side {
	case "Buy", "Sell":
		params["side"] = side

	default:
		return nil, fmt.Errorf("side value %v is invalid", side)

	}
	// END TEMPLATE check-valid-values

	// assign parameter of side
	params["side"] = side
	// check orderType field -> json key orderType
	orderType := p.orderType

	// TEMPLATE check-valid-values
	switch orderType {
	case "Market", "Limit":
		params["orderType"] = orderType

	default:
		return nil, fmt.Errorf("orderType value %v is invalid", orderType)

	}
	// END TEMPLATE check-valid-values

	// assign parameter of orderType
	params["orderType"] = orderType
	// check qty field -> json key qty
	qty := p.qty

	// assign parameter of qty
	params["qty"] = qty
	// check orderLinkId field -> json key orderLinkId
	orderLinkId := p.orderLinkId

	// assign parameter of orderLinkId
	params["orderLinkId"] = orderLinkId
	// check timeInForce field -> json key timeInForce
	timeInForce := p.timeInForce

	// TEMPLATE check-valid-values
	switch timeInForce {
	case TimeInForceGTC, TimeInForceIOC, TimeInForceFOK:
		params["timeInForce"] = timeInForce

	default:
		return nil, fmt.Errorf("timeInForce value %v is invalid", timeInForce)

	}
	// END TEMPLATE check-valid-values

	// assign parameter of timeInForce
	params["timeInForce"] = timeInForce
	// check marketUnit field -> json key marketUnit
	if p.marketUnit != nil {
		marketUnit := *p.marketUnit

		// TEMPLATE check-valid-values
		switch marketUnit {
		case MarketUnitBase, MarketUnitQuote:
			params["marketUnit"] = marketUnit

		default:
			return nil, fmt.Errorf("marketUnit value %v is invalid", marketUnit)

		}
		// END TEMPLATE check-valid-values

		// assign parameter of marketUnit
		params["marketUnit"] = marketUnit
	} else {
	}
	// check isLeverage field -> json key isLeverage
	if p.isLeverage != nil {
		isLeverage := *p.isLeverage

		// assign parameter of isLeverage
		params["isLeverage"] = isLeverage
	} else {
	}
	// check price field -> json key price
	if p.price != nil {
		price := *p.price

		// assign parameter of price
		params["price"] = price
	} else {
	}
	// check triggerDirection field -> json key triggerDirection
	if p.triggerDirection != nil {
		triggerDirection := *p.triggerDirection

		// assign parameter of triggerDirection
		params["triggerDirection"] = triggerDirection
	} else {
	}
	// check orderFilter field -> json key orderFilter
	if p.orderFilter != nil {
		orderFilter := *p.orderFilter

		// assign parameter of orderFilter
		params["orderFilter"] = orderFilter
	} else {
	}
	// check triggerPrice field -> json key triggerPrice
	if p.triggerPrice != nil {
		triggerPrice := *p.triggerPrice

		// assign parameter of triggerPrice
		params["triggerPrice"] = triggerPrice
	} else {
	}
	// check triggerBy field -> json key triggerBy
	if p.triggerBy != nil {
		triggerBy := *p.triggerBy

		// assign parameter of triggerBy
		params["triggerBy"] = triggerBy
	} else {
	}
	// check orderIv field -> json key orderIv
	if p.orderIv != nil {
		orderIv := *p.orderIv

		// assign parameter of orderIv
		params["orderIv"] = orderIv
	} else {
	}
	// check positionIdx field -> json key positionIdx
	if p.positionIdx != nil {
		positionIdx := *p.positionIdx

		// assign parameter of positionIdx
		params["positionIdx"] = positionIdx
	} else {
	}
	// check takeProfit field -> json key takeProfit
	if p.takeProfit != nil {
		takeProfit := *p.takeProfit

		// assign parameter of takeProfit
		params["takeProfit"] = takeProfit
	} else {
	}
	// check stopLoss field -> json key stopLoss
	if p.stopLoss != nil {
		stopLoss := *p.stopLoss

		// assign parameter of stopLoss
		params["stopLoss"] = stopLoss
	} else {
	}
	// check tpTriggerBy field -> json key tpTriggerBy
	if p.tpTriggerBy != nil {
		tpTriggerBy := *p.tpTriggerBy

		// assign parameter of tpTriggerBy
		params["tpTriggerBy"] = tpTriggerBy
	} else {
	}
	// check slTriggerBy field -> json key slTriggerBy
	if p.slTriggerBy != nil {
		slTriggerBy := *p.slTriggerBy

		// assign parameter of slTriggerBy
		params["slTriggerBy"] = slTriggerBy
	} else {
	}
	// check reduceOnly field -> json key reduceOnly
	if p.reduceOnly != nil {
		reduceOnly := *p.reduceOnly

		// assign parameter of reduceOnly
		params["reduceOnly"] = reduceOnly
	} else {
	}
	// check closeOnTrigger field -> json key closeOnTrigger
	if p.closeOnTrigger != nil {
		closeOnTrigger := *p.closeOnTrigger

		// assign parameter of closeOnTrigger
		params["closeOnTrigger"] = closeOnTrigger
	} else {
	}
	// check smpType field -> json key smpType
	if p.smpType != nil {
		smpType := *p.smpType

		// assign parameter of smpType
		params["smpType"] = smpType
	} else {
	}
	// check mmp field -> json key mmp
	if p.mmp != nil {
		mmp := *p.mmp

		// assign parameter of mmp
		params["mmp"] = mmp
	} else {
	}
	// check tpslMode field -> json key tpslMode
	if p.tpslMode != nil {
		tpslMode := *p.tpslMode

		// assign parameter of tpslMode
		params["tpslMode"] = tpslMode
	} else {
	}
	// check tpLimitPrice field -> json key tpLimitPrice
	if p.tpLimitPrice != nil {
		tpLimitPrice := *p.tpLimitPrice

		// assign parameter of tpLimitPrice
		params["tpLimitPrice"] = tpLimitPrice
	} else {
	}
	// check slLimitPrice field -> json key slLimitPrice
	if p.slLimitPrice != nil {
		slLimitPrice := *p.slLimitPrice

		// assign parameter of slLimitPrice
		params["slLimitPrice"] = slLimitPrice
	} else {
	}
	// check tpOrderType field -> json key tpOrderType
	if p.tpOrderType != nil {
		tpOrderType := *p.tpOrderType

		// assign parameter of tpOrderType
		params["tpOrderType"] = tpOrderType
	} else {
	}
	// check slOrderType field -> json key slOrderType
	if p.slOrderType != nil {
		slOrderType := *p.slOrderType

		// assign parameter of slOrderType
		params["slOrderType"] = slOrderType
	} else {
	}

	return params, nil
}

// GetParametersQuery converts the parameters from GetParameters into the url.Values format
func (p *PlaceOrderRequest) GetParametersQuery() (url.Values, error) {
	query := url.Values{}

	params, err := p.GetParameters()
	if err != nil {
		return query, err
	}

	for _k, _v := range params {
		if p.isVarSlice(_v) {
			p.iterateSlice(_v, func(it interface{}) {
				query.Add(_k+"[]", fmt.Sprintf("%v", it))
			})
		} else {
			query.Add(_k, fmt.Sprintf("%v", _v))
		}
	}

	return query, nil
}

// GetParametersJSON converts the parameters from GetParameters into the JSON format
func (p *PlaceOrderRequest) GetParametersJSON() ([]byte, error) {
	params, err := p.GetParameters()
	if err != nil {
		return nil, err
	}

	return json.Marshal(params)
}

// GetSlugParameters builds and checks the slug parameters and return the result in a map object
func (p *PlaceOrderRequest) GetSlugParameters() (map[string]interface{}, error) {
	var params = map[string]interface{}{}

	return params, nil
}

func (p *PlaceOrderRequest) applySlugsToUrl(url string, slugs map[string]string) string {
	for _k, _v := range slugs {
		needleRE := regexp.MustCompile(":" + _k + "\\b")
		url = needleRE.ReplaceAllString(url, _v)
	}

	return url
}

func (p *PlaceOrderRequest) iterateSlice(slice interface{}, _f func(it interface{})) {
	sliceValue := reflect.ValueOf(slice)
	for _i := 0; _i < sliceValue.Len(); _i++ {
		it := sliceValue.Index(_i).Interface()
		_f(it)
	}
}

func (p *PlaceOrderRequest) isVarSlice(_v interface{}) bool {
	rt := reflect.TypeOf(_v)
	switch rt.Kind() {
	case reflect.Slice:
		return true
	}
	return false
}

func (p *PlaceOrderRequest) GetSlugsMap() (map[string]string, error) {
	slugs := map[string]string{}
	params, err := p.GetSlugParameters()
	if err != nil {
		return slugs, nil
	}

	for _k, _v := range params {
		slugs[_k] = fmt.Sprintf("%v", _v)
	}

	return slugs, nil
}

// GetPath returns the request path of the API
func (p *PlaceOrderRequest) GetPath() string {
	return "/v5/order/create"
}

// Do generates the request object and send the request object to the API endpoint
func (p *PlaceOrderRequest) Do(ctx context.Context) (*PlaceOrderResponse, error) {
	if err := PlaceOrderRequestLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	params, err := p.GetParameters()
	if err != nil {
		return nil, err
	}
	query := url.Values{}

	var apiURL string

	apiURL = p.GetPath()

	req, err := p.client.NewAuthenticatedRequest(ctx, "POST", apiURL, query, params)
	if err != nil {
		return nil, err
	}

	response, err := p.client.SendRequest(req)
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
	var data PlaceOrderResponse
	if err := json.Unmarshal(apiResponse.Result, &data); err != nil {
		return nil, err
	}
	return &data, nil
}
