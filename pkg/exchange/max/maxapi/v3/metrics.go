package v3

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/c9s/requestgen"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var latencyMetrics = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "max_api_latency_ms",
		Help:    "The histogram of latency returned by MAX API",
		Buckets: prometheus.ExponentialBuckets(20, 2, 9), // 20ms to 5120ms
	},
	[]string{"path", "status_code"},
)

func recordLatencyMetrics(req *http.Request, latencyMs float64, err error) {
	path := req.URL.Path
	statusCode := 200
	if err != nil {
		var requestErr *requestgen.ErrResponse
		if errors.As(err, &requestErr) {
			statusCode = requestErr.StatusCode
		} else {
			// Use 500 for errors that can't be casted
			statusCode = 500
			log.WithError(err).Warnf("failed to cast request error to record status code: %T", err)
		}
	}
	latencyMetrics.With(
		prometheus.Labels{
			"path":        path,
			"status_code": strconv.Itoa(statusCode),
		},
	).Observe(latencyMs)
}
