package amberdataapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// APIError is an error response.
//
// It decodes both shapes the service produces. The documented one is the same
// envelope as a success:
//
//	{"status":404,"title":"Not Found","description":"..."}
//
// but an unauthenticated or unauthorized request never reaches the application
// and comes back from AWS API Gateway instead:
//
//	{"message":"Forbidden"}
//
// Tolerating both is not defensive padding: the second is what every request
// without a valid key returns, so it is the shape a developer without
// credentials sees exclusively.
type APIError struct {
	HTTPStatus int

	Status      int    `json:"status"`
	Title       string `json:"title"`
	Description string `json:"description"`

	// Message is the API Gateway shape.
	Message string `json:"message"`

	// Body is kept when neither shape parses, so an unexpected response is
	// still diagnosable.
	Body string `json:"-"`
}

// ParseAPIError builds an APIError from a response.
func ParseAPIError(status int, body []byte) *APIError {
	e := &APIError{HTTPStatus: status, Body: strings.TrimSpace(string(body))}

	// Both shapes are objects with string or int fields, so one pass suffices.
	_ = json.Unmarshal(body, e)

	return e
}

func (e *APIError) Error() string {
	var detail string
	switch {
	case e.Description != "":
		detail = e.Description
	case e.Title != "":
		detail = e.Title
	case e.Message != "":
		detail = e.Message
	default:
		detail = e.Body
	}

	msg := fmt.Sprintf("amberdata: http %d: %s", e.HTTPStatus, detail)

	if e.IsForbidden() {
		// A 403 is genuinely ambiguous, and the two causes lead to very
		// different fixes, so say both rather than letting the reader assume.
		msg += " (a 403 means either the API key is missing or invalid, or the" +
			" requested exchange and asset class are not included in the plan" +
			" this key is scoped to)"
	}

	return msg
}

// IsForbidden reports a 403.
func (e *APIError) IsForbidden() bool { return e.HTTPStatus == http.StatusForbidden }

// IsUnauthorized reports a 401, which the documentation promises for a missing
// key but which is not what the service actually returns.
func (e *APIError) IsUnauthorized() bool { return e.HTTPStatus == http.StatusUnauthorized }

// IsTooLarge reports the 400 the service returns when a single query's result
// would exceed its 10 MB response cap.
//
// This, rather than the time range, is usually the binding constraint on trades
// and order book pulls, and the response to it is to halve the window rather
// than to retry the same request.
func (e *APIError) IsTooLarge() bool {
	if e.HTTPStatus != http.StatusBadRequest {
		return false
	}

	haystack := strings.ToLower(e.Description + " " + e.Title + " " + e.Message + " " + e.Body)
	for _, needle := range []string{"10 mb", "10mb", "too large", "exceeds", "payload size"} {
		if strings.Contains(haystack, needle) {
			return true
		}
	}
	return false
}

// IsTimeout reports the 504 the service returns after 28 seconds. Like
// IsTooLarge, the fix is a smaller window.
func (e *APIError) IsTimeout() bool { return e.HTTPStatus == http.StatusGatewayTimeout }

// IsRateLimited reports a 429.
func (e *APIError) IsRateLimited() bool { return e.HTTPStatus == http.StatusTooManyRequests }

// ShouldShrinkWindow reports whether the request should be retried over a
// smaller time range rather than repeated unchanged.
func (e *APIError) ShouldShrinkWindow() bool { return e.IsTooLarge() || e.IsTimeout() }
