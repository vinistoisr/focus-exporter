package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Create a new instance of a metrics registry.
	reg := prometheus.NewRegistry()

	// Create some standard server metrics.
	goCollector := prometheus.NewGoCollector()

	// Register our metrics with our registry.
	reg.MustRegister(goCollector)

	// Create a HTTP handler for our metrics registry.
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	// Start the HTTP server for our metrics endpoint.
	http.ListenAndServe(":9183", nil)
}
