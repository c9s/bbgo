package amberdataapi

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseAPIError_GatewayShape covers the shape a developer without a key sees
// exclusively. It does not match the documented envelope, which is why the
// decoder accepts both.
func TestParseAPIError_GatewayShape(t *testing.T) {
	body, err := os.ReadFile("testdata/error_gateway_403.json")
	require.NoError(t, err)

	e := ParseAPIError(http.StatusForbidden, body)

	assert.Equal(t, "Forbidden", e.Message)
	assert.Zero(t, e.Status, "the gateway shape has no status field")
	assert.True(t, e.IsForbidden())

	// A 403 has two very different causes, so the message must name both rather
	// than letting the reader assume it is a bad key.
	assert.Contains(t, e.Error(), "missing or invalid")
	assert.Contains(t, e.Error(), "not included in the plan")
}

func TestParseAPIError_DocumentedShape(t *testing.T) {
	body, err := os.ReadFile("testdata/error_documented_404.json")
	require.NoError(t, err)

	e := ParseAPIError(http.StatusNotFound, body)

	assert.Equal(t, 404, e.Status)
	assert.Equal(t, "Not Found", e.Title)
	assert.Contains(t, e.Error(), "The requested instrument was not found")
	assert.False(t, e.IsForbidden())
}

func TestParseAPIError_UnknownShapeKeepsBody(t *testing.T) {
	e := ParseAPIError(http.StatusBadGateway, []byte("<html>upstream is down</html>"))

	assert.Contains(t, e.Error(), "upstream is down",
		"an unrecognized body must stay diagnosable")
}

// TestAPIError_ShouldShrinkWindow covers the two failures whose correct response
// is a smaller request rather than a retry. The 10 MB response cap, not the time
// range, is usually what binds on trades and order book pulls.
func TestAPIError_ShouldShrinkWindow(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{
			name:   "400 mentioning the size cap",
			status: http.StatusBadRequest,
			body:   `{"status":400,"title":"Bad Request","description":"The response payload size exceeds 10 MB"}`,
			want:   true,
		},
		{
			name:   "400 about something else",
			status: http.StatusBadRequest,
			body:   `{"status":400,"title":"Bad Request","description":"sortDirection is not allowed for this range"}`,
			want:   false,
		},
		{
			name:   "504 after the server's 28 second limit",
			status: http.StatusGatewayTimeout,
			body:   `{}`,
			want:   true,
		},
		{
			name:   "403 is not a window problem",
			status: http.StatusForbidden,
			body:   `{"message":"Forbidden"}`,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := ParseAPIError(tt.status, []byte(tt.body))
			assert.Equal(t, tt.want, e.ShouldShrinkWindow())
		})
	}
}

func TestAPIError_Classification(t *testing.T) {
	assert.True(t, ParseAPIError(429, []byte(`{}`)).IsRateLimited())
	assert.True(t, ParseAPIError(401, []byte(`{}`)).IsUnauthorized())
	assert.True(t, ParseAPIError(504, []byte(`{}`)).IsTimeout())
}

func TestTimestamp_Encodings(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"milliseconds", `1717518651671`, "2024-06-04T16:30:51.671Z"},
		{"quoted milliseconds", `"1717518651671"`, "2024-06-04T16:30:51.671Z"},
		{"human readable", `"2024-06-04 16:23:12 414"`, "2024-06-04T16:23:12.414Z"},
		{"human readable without millis", `"2024-06-04 16:23:12"`, "2024-06-04T16:23:12.000Z"},
		{"rfc3339", `"2024-06-04T16:23:12.414Z"`, "2024-06-04T16:23:12.414Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts Timestamp
			require.NoError(t, ts.UnmarshalJSON([]byte(tt.json)))
			assert.Equal(t, tt.want, ts.Format("2006-01-02T15:04:05.000Z"))
		})
	}

	t.Run("null stays zero", func(t *testing.T) {
		var ts Timestamp
		require.NoError(t, ts.UnmarshalJSON([]byte(`null`)))
		assert.True(t, ts.IsZero())
	})

	t.Run("garbage errors", func(t *testing.T) {
		var ts Timestamp
		assert.Error(t, ts.UnmarshalJSON([]byte(`"not a time"`)))
	})
}

// TestNumber_Encodings covers the inconsistency that made a lenient type
// necessary: the same logical field arrives bare in one endpoint and quoted in
// another, and can be null in both.
func TestNumber_Encodings(t *testing.T) {
	for _, tt := range []struct {
		json  string
		want  string
		valid bool
	}{
		{`70845.5`, "70845.5", true},
		{`"70845.5"`, "70845.5", true},
		{`0`, "0", true},
		{`null`, "0", false},
		{`""`, "0", false},
	} {
		var n Number
		require.NoError(t, n.UnmarshalJSON([]byte(tt.json)), tt.json)
		assert.Equal(t, tt.valid, n.Valid, tt.json)
		if tt.valid {
			assert.Equal(t, tt.want, n.String(), tt.json)
		}
	}
}

func TestUint64_Encodings(t *testing.T) {
	for _, tt := range []struct {
		json  string
		want  uint64
		valid bool
	}{
		{`960251880359`, 960251880359, true},
		{`"960251880359"`, 960251880359, true},
		{`null`, 0, false},
		{`"not-numeric"`, 0, false},
	} {
		var u Uint64
		require.NoError(t, u.UnmarshalJSON([]byte(tt.json)), tt.json)
		assert.Equal(t, tt.valid, u.Valid, tt.json)
		assert.Equal(t, tt.want, u.Value, tt.json)
	}
}
