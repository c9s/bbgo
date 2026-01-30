package maxapi

import (
	"fmt"
	"regexp"

	"github.com/c9s/requestgen"
	"github.com/pkg/errors"
)

var htmlTagPattern = regexp.MustCompile("<[/]?[a-zA-Z-]+.*?>")

type ErrorField struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse is the custom error type that is returned if the API returns an
// error.
type ErrorResponse struct {
	*requestgen.Response
	Err ErrorField `json:"error"`
}

func (r *ErrorResponse) Error() string {
	return fmt.Sprintf("%s %s: %d %d %s",
		r.Response.Response.Request.Method,
		r.Response.Response.Request.URL.String(),
		r.Response.Response.StatusCode,
		r.Err.Code,
		r.Err.Message,
	)
}

// ToErrorResponse tries to convert/parse the server response to the standard Error interface object
func ToErrorResponse(response *requestgen.Response) (errorResponse *ErrorResponse, err error) {
	errorResponse = &ErrorResponse{Response: response}

	contentType := response.Header.Get("content-type")
	switch contentType {
	case "text/json", "application/json", "application/json; charset=utf-8":
		var err = response.DecodeJSON(errorResponse)
		if err != nil {
			return errorResponse, errors.Wrapf(err, "failed to decode json for response: %d %s", response.StatusCode, string(response.Body))
		}
		return errorResponse, nil
	case "text/html":
		// convert 5xx error from the HTML page to the ErrorResponse
		errorResponse.Err.Message = htmlTagPattern.ReplaceAllLiteralString(string(response.Body), "")
		return errorResponse, nil
	case "text/plain":
		errorResponse.Err.Message = string(response.Body)
		return errorResponse, nil
	}

	return errorResponse, fmt.Errorf("unexpected response content type %s", contentType)
}
