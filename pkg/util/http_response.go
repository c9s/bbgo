package util

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// Response is wrapper for standard http.Response and provides
// more methods.
type Response struct {
	*http.Response

	// Body overrides the composited Body field.
	Body []byte
}

// newResponse is a wrapper of the http.Response instance, it reads the response body and close the file.
func NewResponse(r *http.Response) (response *Response, err error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	err = r.Body.Close()
	response = &Response{Response: r, Body: body}
	return response, err
}

// String converts response body to string.
// An empty string will be returned if error.
func (r *Response) String() string {
	return string(r.Body)
}

func (r *Response) DecodeJSON(o interface{}) error {
	return json.Unmarshal(r.Body, o)
}

func (r *Response) IsError() bool {
	return r.StatusCode >= 400
}
