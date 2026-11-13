package backpackapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// MaxBatchOrders is the maximum number of orders accepted by POST /api/v1/orders.
const MaxBatchOrders = 50

// OrderExecutePayload is a single order of a batch submission.
//
// The batch endpoint takes a JSON array as its request body, which requestgen's
// map[string]interface{} parameter model can not express, so this endpoint is hand written.
// The field names and the omitempty tags are what determine the signing string, so they must
// stay in sync with the JSON body.
type OrderExecutePayload struct {
	Symbol    string    `json:"symbol"`
	Side      Side      `json:"side"`
	OrderType OrderType `json:"orderType"`

	Quantity      string `json:"quantity,omitempty"`
	QuoteQuantity string `json:"quoteQuantity,omitempty"`
	Price         string `json:"price,omitempty"`

	TimeInForce TimeInForce `json:"timeInForce,omitempty"`

	PostOnly   *bool `json:"postOnly,omitempty"`
	ReduceOnly *bool `json:"reduceOnly,omitempty"`

	ClientId *uint32 `json:"clientId,omitempty"`

	SelfTradePrevention SelfTradePrevention `json:"selfTradePrevention,omitempty"`

	AutoLend        *bool `json:"autoLend,omitempty"`
	AutoLendRedeem  *bool `json:"autoLendRedeem,omitempty"`
	AutoBorrow      *bool `json:"autoBorrow,omitempty"`
	AutoBorrowRepay *bool `json:"autoBorrowRepay,omitempty"`

	TriggerPrice    string    `json:"triggerPrice,omitempty"`
	TriggerQuantity string    `json:"triggerQuantity,omitempty"`
	TriggerBy       TriggerBy `json:"triggerBy,omitempty"`
}

// BatchOrderResult is one element of the batch response.
//
// The API returns a discriminated union: Operation is "Ok" for a submitted order and "Err" for
// a rejected one.
type BatchOrderResult struct {
	Operation string `json:"operation"`

	Order *Order    `json:"-"`
	Error *APIError `json:"-"`
}

func (r *BatchOrderResult) UnmarshalJSON(data []byte) error {
	var probe struct {
		Operation string `json:"operation"`
	}

	if err := json.Unmarshal(data, &probe); err != nil {
		return err
	}

	r.Operation = probe.Operation

	switch probe.Operation {
	case "Err":
		var apiErr APIError
		if err := json.Unmarshal(data, &apiErr); err != nil {
			return err
		}

		r.Error = &apiErr
		return nil

	default:
		var order Order
		if err := json.Unmarshal(data, &order); err != nil {
			return err
		}

		r.Order = &order
		return nil
	}
}

func (r BatchOrderResult) IsOk() bool {
	return r.Error == nil
}

// BatchPlaceOrders submits up to MaxBatchOrders orders in a single request.
//
// If any order in the batch fails validation the whole batch is rejected.
func (c *RestClient) BatchPlaceOrders(
	ctx context.Context, orders []OrderExecutePayload,
) ([]BatchOrderResult, error) {
	if len(orders) == 0 {
		return nil, errors.New("batch order request requires at least one order")
	}

	if len(orders) > MaxBatchOrders {
		return nil, fmt.Errorf("batch order request accepts at most %d orders, got %d",
			MaxBatchOrders, len(orders))
	}

	if len(c.key) == 0 {
		return nil, errNoApiKey
	}

	if c.privateKey == nil {
		return nil, errNoApiSecret
	}

	timestamp := time.Now().UnixMilli()
	window := c.getWindow()

	signingString, err := buildBatchSigningString(orders, timestamp, window)
	if err != nil {
		return nil, err
	}

	req, err := c.NewRequest(ctx, http.MethodPost, "/api/v1/orders", nil, orders)
	if err != nil {
		return nil, err
	}

	setAuthHeaders(req, c.key, c.sign(signingString), timestamp, window)

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	var results []BatchOrderResult
	if err := resp.DecodeJSON(&results); err != nil {
		return nil, err
	}

	return results, nil
}

// buildBatchSigningString assembles the signing string of a batch order submission.
//
// Each order contributes its own "instruction=orderExecute&<sorted params>" segment, the
// segments are joined with "&", and the timestamp and window are appended once at the end.
func buildBatchSigningString(orders []OrderExecutePayload, timestamp int64, window uint64) (string, error) {
	segments := make([]string, 0, len(orders))
	for i, order := range orders {
		params, err := orderPayloadParams(order)
		if err != nil {
			return "", errors.Wrapf(err, "unable to build the signing string of order #%d", i)
		}

		segments = append(segments, buildSigningStringPrefix(InstructionOrderExecute, params))
	}

	var sb strings.Builder
	sb.WriteString(strings.Join(segments, "&"))
	sb.WriteString("&timestamp=")
	sb.WriteString(strconv.FormatInt(timestamp, 10))
	sb.WriteString("&window=")
	sb.WriteString(strconv.FormatUint(window, 10))
	return sb.String(), nil
}

// orderPayloadParams returns the alphabetically ordered "key=value" pairs of an order payload.
//
// It round-trips through JSON so that the pairs always match the request body exactly,
// including which optional fields were omitted.
func orderPayloadParams(order OrderExecutePayload) ([]string, error) {
	data, err := json.Marshal(order)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	pairs := make([]string, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, k+"="+formatSigningValue(v))
	}

	sort.Strings(pairs)
	return pairs, nil
}

func buildSigningStringPrefix(instruction Instruction, params []string) string {
	var sb strings.Builder
	sb.WriteString("instruction=")
	sb.WriteString(string(instruction))

	for _, pair := range params {
		sb.WriteString("&")
		sb.WriteString(pair)
	}

	return sb.String()
}
