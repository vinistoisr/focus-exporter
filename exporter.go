package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/VinistoisR/focus-exporter/internal/windowinfo"

	"github.com/VinistoisR/focus-exporter/internal/inactivity"
)

func main() {
	// Request administrator privileges (uncomment when needed)
	// if !amAdmin() {
	// 	runMeElevated()
	// 	return
	// }

	const inactivityThreshold = 5000 // 5 seconds in milliseconds

	// Prometheus Metrics Setup
	reg := prometheus.NewRegistry()

	// Standard Go metrics
	goCollector := collectors.NewGoCollector()
	reg.MustRegister(goCollector)

	// Window focused PID Gauge metric
	windowPidGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "focused_window_pid",
			Help: "Process ID of the currently focused window.",
		},
		[]string{"hostname", "username", "window_title", "process_name"},
	)
	reg.MustRegister(windowPidGauge)

	// Inactivity counter metric
	inactivityMetric := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "user_inactivity_seconds_total",
			Help: "Total seconds of user inactivity.",
		},
		[]string{"hostname", "username"},
	)
	reg.MustRegister(inactivityMetric)

	// Expose metrics endpoint
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	go http.ListenAndServe(":9183", nil)

	lastInputTime := inactivity.GetTickCount()

	// Main loop
	for {
		// Get Active Window Information
		windowInfo, err := windowinfo.GetActiveWindowInfo()
		if err != nil {
			fmt.Println("Error getting window information:", err)
		} else {
			fmt.Println("Window Title:", windowInfo.Title)
			fmt.Println("Process ID:", windowInfo.ProcessID)
			fmt.Println("Process Name:", windowInfo.ProcessName)

			// Update Prometheus gauge metric
			windowPidGauge.WithLabelValues(windowInfo.Hostname, windowInfo.Username, windowInfo.Title, windowInfo.ProcessName).Set(float64(windowInfo.ProcessID))
		}

		// Get inactivity time
		inactivityTime, shouldIncrementCounter := inactivity.GetInactivityTime(inactivityThreshold, lastInputTime)
		fmt.Println("Inactivity:", inactivityTime)

		// Update Prometheus counter metric ONLY if inactive for a whole second
		if shouldIncrementCounter {
			inactivityMetric.WithLabelValues(windowInfo.Hostname, windowInfo.Username).Inc()
			lastInputTime = inactivity.GetTickCount() // Reset lastInputTime if counter was incremented
		}

		time.Sleep(1 * time.Second)
		fmt.Println("------------")
	}

}
