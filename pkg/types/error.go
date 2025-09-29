package types

import (
	"errors"
	"fmt"
	"net/http/httputil"

	"github.com/c9s/requestgen"
	log "github.com/sirupsen/logrus"
)

var ErrOrderNotFound = errors.New("order not found")

type RecoverOrderError struct {
	Exchange    ExchangeName
	SubmitOrder SubmitOrder
	OriginalErr error
	RecoverErr  error

	ResponseErr *requestgen.ErrResponse
}

func NewRecoverOrderError(exchange ExchangeName, submitOrder SubmitOrder, originalErr, err error) *RecoverOrderError {
	var responseErr *requestgen.ErrResponse
	errors.As(originalErr, &responseErr)
	return &RecoverOrderError{
		Exchange:    exchange,
		SubmitOrder: submitOrder,
		OriginalErr: originalErr,
		RecoverErr:  err,
		ResponseErr: responseErr,
	}
}

func (e *RecoverOrderError) IsNotFound() bool {
	return errors.Is(e.RecoverErr, ErrOrderNotFound)
}

// Debug prints the debug information of the recover order error.
// If body is true, it will also print the request and response body.
func (e *RecoverOrderError) Debug(body bool) {
	log.Warnf("recover order error: exchange=%s, order=%+v, original error: %s, error: %s",
		e.Exchange, e.SubmitOrder, e.OriginalErr.Error(), e.RecoverErr.Error())

	if e.ResponseErr != nil {
		resp := e.ResponseErr.Response
		req := e.ResponseErr.Request
		resTxt, _ := httputil.DumpResponse(resp.Response, body)
		log.Warnf("response dump: status code %d %s\n%s", resp.StatusCode, resp.Status, resTxt)

		if req != nil {
			reqTxt, _ := httputil.DumpRequest(req, body)
			log.Warnf("request dump: %s %s\n%s", resp.Request.Method, resp.Request.URL.String(), string(reqTxt))
		}
	}
}

func (e *RecoverOrderError) Error() string {
	return fmt.Sprintf("recover order not found: %+v, original error: %s, error: %s", e.SubmitOrder, e.OriginalErr.Error(), e.RecoverErr.Error())
}

type OrderError struct {
	error error
	order Order
}

func (e *OrderError) Error() string {
	return fmt.Sprintf("%s exchange: %s orderID:%d", e.error.Error(), e.order.Exchange, e.order.OrderID)
}

func (e *OrderError) Order() Order {
	return e.order
}

func NewOrderError(e error, o Order) error {
	return &OrderError{
		error: e,
		order: o,
	}
}

type ZeroAssetError struct {
	error
}

func NewZeroAssetError(e error) ZeroAssetError {
	return ZeroAssetError{
		error: e,
	}
}
