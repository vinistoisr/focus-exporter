package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/VinistoisR/focus-exporter/internal/inactivity"
	"github.com/VinistoisR/focus-exporter/internal/windowinfo"
)

// Global variables to track the last focused window and start time of focus
var (
	lastWindowInfo      windowinfo.ActiveWindowInfo
	lastWindowFocusTime time.Time
)

// inactivityThresholdSec is the threshold in seconds for inactivity
var inactivityThresholdSec uint64

// init is called before main
func init() {
	flag.Uint64Var(&inactivityThresholdSec, "inactivityThreshold", 10, "The inactivity threshold in seconds")
}

func main() {
	flag.Parse()
	inactivityThreshold := inactivityThresholdSec * 1000
	fmt.Printf("Inactivity Threshold: %d milliseconds\n", inactivityThreshold)

	// Prometheus Metrics Setup
	reg := prometheus.NewRegistry()

	// Standard Go application metrics
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
			Name: "focus_inactivity_seconds_total",
			Help: "Total seconds of user inactivity.",
		},
		[]string{"hostname", "username"},
	)
	reg.MustRegister(inactivityMetric)

	// Window focus change counter metric
	focusChangeCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "focused_Window_changes_total",
			Help: "Total number of times the focused window has changed.",
		},
		[]string{"hostname", "username"},
	)
	reg.MustRegister(focusChangeCounter)

	// Focused window duration counter metric
	focusedWindowDuration := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "focused_window_duration_seconds",
			Help: "Duration in seconds the window has been focused.",
		},
		[]string{"hostname", "username", "process_name"},
	)
	reg.MustRegister(focusedWindowDuration)

	// Expose metrics endpoint
	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	go http.ListenAndServe(":9183", nil)

	// Initialize the last window info and focus time
	lastWindowInfo, _ = windowinfo.GetActiveWindowInfo(*focusChangeCounter)
	lastWindowFocusTime = time.Now()

	// Main loop
	for {
		// Get Active Window Information
		windowInfo, err := windowinfo.GetActiveWindowInfo(*focusChangeCounter)
		if err != nil {
			fmt.Println("Error getting window information:", err)
		} else {
			fmt.Println("Window Title:", windowInfo.Title)
			fmt.Println("Process ID:", windowInfo.ProcessID)
			fmt.Println("Process Name:", windowInfo.ProcessName)
			// Reset the gauge
			windowPidGauge.Reset()

			// Update Prometheus gauge metric
			windowPidGauge.WithLabelValues(windowInfo.Hostname, windowInfo.Username, windowInfo.Title, windowInfo.ProcessName).Set(float64(windowInfo.ProcessID))

			// Check if the focused window has changed
			if windowInfo != lastWindowInfo {
				// Calculate the duration the previous window was focused
				duration := time.Since(lastWindowFocusTime).Seconds()
				focusedWindowDuration.WithLabelValues(lastWindowInfo.Hostname, lastWindowInfo.Username, lastWindowInfo.ProcessName).Add(duration)

				// Update the last window info and focus time
				lastWindowInfo = windowInfo
				lastWindowFocusTime = time.Now()
			}
		}

		// Get inactivity time
		inactivityTime, shouldIncrementCounter := inactivity.GetInactivityTime(inactivityThreshold)
		fmt.Println("Inactivity:", inactivityTime)

		// Update Prometheus counter metric ONLY if inactive for the threshold duration
		if shouldIncrementCounter {
			inactivityMetric.WithLabelValues(windowInfo.Hostname, windowInfo.Username).Inc()
		}

		time.Sleep(1 * time.Second)
		fmt.Println("------------")
	}
}
