package maxapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/c9s/requestgen"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	log "github.com/sirupsen/logrus"
)

var latencyMetrics = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "max_api_latency_seconds",
		Help:    "The histogram of latency returned by MAX API",
		Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to ~30s
	},
	[]string{"path", "status_code"},
)

func recordLatencyMetrics(req *http.Request, latency float64, err error) {
	path := req.RequestURI
	statusCode := 200
	if err != nil {
		var requestErr *requestgen.ErrResponse
		if errors.As(err, &requestErr) {
			statusCode = requestErr.StatusCode
		} else {
			log.WithError(err).Warn("fail to cast request error to record status code")
		}
	}
	latencyMetrics.With(
		prometheus.Labels{
			"path":        path,
			"status_code": strconv.Itoa(statusCode),
		},
	).Observe(latency)
}
