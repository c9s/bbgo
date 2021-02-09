package ftx

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/util"
)

func Test_toErrorResponse(t *testing.T) {
	r, err := util.NewResponse(&http.Response{
		Header:     http.Header{},
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewReader([]byte(`{"Success": true}`))),
	})
	assert.NoError(t, err)

	_, err = toErrorResponse(r)
	assert.EqualError(t, err, "unexpected response content type ")
	r.Header.Set("content-type", "text/json")

	_, err = toErrorResponse(r)
	assert.EqualError(t, err, "response.Success should be false")

	r.Body = []byte(`{"error":"Not logged in","Success":false}`)
	errResp, err := toErrorResponse(r)
	assert.NoError(t, err)
	assert.False(t, errResp.IsSuccess)
	assert.Equal(t, "Not logged in", errResp.ErrorString)
}
