package service

import (
	"go.uber.org/zap"
	"net/http"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RegisterMetrics initializes grpc_prometheus and starts HTTP /metrics server
func RegisterMetrics(listenAddr string) {
	// enable histograms (optional)
	grpc_prometheus.EnableHandlingTimeHistogram()

	// create handler; metrics already registered on import
	http.Handle("/metrics", promhttp.Handler())

	// run metrics HTTP server (non-blocking)
	go func() {
		if err := http.ListenAndServe(listenAddr, nil); err != nil {
			logger.Error("metrics http server stopped", zap.Error(err))
		}
	}()
}
