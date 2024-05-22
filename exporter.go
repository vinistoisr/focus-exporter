package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vinistoisr/focus-exporter/internal/windowinfo"
)

// Command-line flags
var (
	inactivityThresholdSec uint64
	listenInterface        string
	listenPort             int
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

func init() {
	// Load Environment Variables or use defaults
	inactivityThresholdSec, _ = strconv.ParseUint(os.Getenv("INACTIVITY_THRESHOLD_SEC"), 10, 64)
	if inactivityThresholdSec == 0 {
		inactivityThresholdSec = 60 // default threshold of 60 seconds
	}
	listenInterface = os.Getenv("LISTEN_INTERFACE")
	privateMode = os.Getenv("PRIVATE_MODE") == "true"
	debugMode = os.Getenv("DEBUG_MODE") == "true"
	listenPort = 9183 // default port

	// Parse command-line flags, will override environment variables if set
	flag.Uint64Var(&inactivityThresholdSec, "inactivityThreshold", inactivityThresholdSec, "The inactivity threshold in seconds")
	flag.StringVar(&listenInterface, "interface", listenInterface, "The interface to listen on (default is all interfaces)")
	flag.IntVar(&listenPort, "port", listenPort, "The port to listen on (default is 9183)")
	flag.BoolVar(&privateMode, "private", privateMode, "When true, the window title will be replaced with the process name for increased privacy")
	flag.BoolVar(&debugMode, "debug", debugMode, "When true, output all values to the console")

	flag.Parse()
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
	fmt.Printf("Listening Port: %d\n", listenPort)
	fmt.Printf("Private Mode: %v\n", privateMode)
	fmt.Printf("Debug Mode: %v\n", debugMode)

	inactivityThreshold := inactivityThresholdSec * 1000
	listenAddress := fmt.Sprintf("%s:%d", listenInterface, listenPort)

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
