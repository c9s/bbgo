package httptesting

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

func BuildResponse(code int, payload []byte) *http.Response {
	return &http.Response{
		StatusCode:    code,
		Body:          io.NopCloser(bytes.NewBuffer(payload)),
		ContentLength: int64(len(payload)),
	}
}

func BuildResponseString(code int, payload string) *http.Response {
	b := []byte(payload)
	return &http.Response{
		StatusCode: code,
		Body: io.NopCloser(
			bytes.NewBuffer(b),
		),
		ContentLength: int64(len(b)),
	}
}

func BuildResponseJson(code int, payload interface{}) *http.Response {
	data, err := json.Marshal(payload)
	if err != nil {
		return BuildResponseString(http.StatusInternalServerError, `{error: "httptesting.MockTransport error calling json.Marshal()"}`)
	}

	resp := BuildResponse(code, data)
	resp.Header = http.Header{}
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

func SetHeader(resp *http.Response, name string, value string) *http.Response {
	if resp.Header == nil {
		resp.Header = http.Header{}
	}
	resp.Header.Set(name, value)
	return resp
}

func DeleteHeader(resp *http.Response, name string) *http.Response {
	if resp.Header != nil {
		resp.Header.Del(name)
	}
	return resp
}
