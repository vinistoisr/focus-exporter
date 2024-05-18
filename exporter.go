package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/VinistoisR/focus-exporter/internal/windowinfo"
)

// Command-line flags
var (
	inactivityThresholdSec uint64
	listenInterface        string
	listenPort             string
	privateMode            bool
	debugMode              bool
)

// Prometheus metrics
var (
	windowPidGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "focused_window_pid",
			Help: "Process ID of the currently focused window.",
		},
		[]string{"hostname", "username", "window_title", "process_name"},
	)

	inactivityMetric = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "focus_inactivity_seconds_total",
			Help: "Total seconds of user inactivity.",
		},
		[]string{"hostname", "username"},
	)

	focusChangeCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "focused_window_changes_total",
			Help: "Total number of times the focused window has changed.",
		},
		[]string{"hostname", "username"},
	)

	focusedWindowDuration = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "focused_window_duration_seconds",
			Help: "Duration in seconds the window has been focused.",
		},
		[]string{"hostname", "username", "process_name"},
	)

	meetingDuration = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "meeting_duration_seconds",
			Help: "Duration in seconds spent in a meeting.",
		},
		[]string{"hostname", "username", "meeting_subject"},
	)
)

// read the inactivityThresholdSec flag value or set it to 60 seconds by default
func init() {
	flag.Uint64Var(&inactivityThresholdSec, "inactivityThreshold", 60, "The inactivity threshold in seconds")
	flag.StringVar(&listenInterface, "interface", "", "The interface to listen on (default is all interfaces)")
	flag.StringVar(&listenPort, "port", "9183", "The port to listen on (default is 9183)")
	flag.BoolVar(&privateMode, "private", false, "When true, the window title will be replaced with the process name for increased privacy")
	flag.BoolVar(&debugMode, "debug", false, "When true, output all values to the console")
}

// setupMetrics initializes the Prometheus metrics and returns the registry
func setupMetrics() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(windowPidGauge)
	reg.MustRegister(inactivityMetric)
	reg.MustRegister(focusChangeCounter)
	reg.MustRegister(focusedWindowDuration)
	reg.MustRegister(meetingDuration)
	return reg
}

// startHTTPServer starts the HTTP server to expose the Prometheus metrics
func startHTTPServer(reg *prometheus.Registry, listenAddress string) {
	go func() {
		if err := http.ListenAndServe(listenAddress, promhttp.HandlerFor(reg, promhttp.HandlerOpts{})); err != nil {
			fmt.Printf("Error starting HTTP server: %v\n", err)
		}
	}()
}

func main() {
	flag.Parse()

	// Always output the initial flag values
	fmt.Printf("Inactivity Threshold: %d seconds\n", inactivityThresholdSec)
	fmt.Printf("Listening Interface: %s\n", listenInterface)
	fmt.Printf("Listening Port: %s\n", listenPort)
	fmt.Printf("Private Mode: %v\n", privateMode)
	fmt.Printf("Debug Mode: %v\n", debugMode)

	inactivityThreshold := inactivityThresholdSec * 1000
	listenAddress := fmt.Sprintf("%s:%s", listenInterface, listenPort)

	reg := setupMetrics()
	startHTTPServer(reg, listenAddress)

	windowinfo.LastWindowInfo, _ = windowinfo.GetActiveWindowInfo(*focusChangeCounter)
	windowinfo.LastWindowFocusTime = time.Now()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			windowinfo.ProcessWindowInfo(inactivityThreshold, privateMode, debugMode, focusChangeCounter, focusedWindowDuration, meetingDuration, inactivityMetric, windowPidGauge)
		}
	}
}
