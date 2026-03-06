package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sys/windows"

	"github.com/vinistoisr/timewarp/internal/db"
	"github.com/vinistoisr/timewarp/internal/mcp"
	"github.com/vinistoisr/timewarp/internal/tray"
	"github.com/vinistoisr/timewarp/internal/windowinfo"
)

// Command-line flags
var (
	inactivityThresholdSec uint64
	listenInterface        string
	listenPort             int
	privateMode            bool
	debugMode              bool
	mcpMode                bool
	silentMode             bool
	installMode            bool
	uninstallMode          bool
	dbpath                 string
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

var (
	trackerMu        sync.Mutex
	tracker          *db.Tracker
	paused           atomic.Bool
	inactThresholdMs atomic.Uint64

	promMu     sync.Mutex
	promServer *http.Server
	promReg    *prometheus.Registry
)

func getTracker() *db.Tracker {
	trackerMu.Lock()
	defer trackerMu.Unlock()
	return tracker
}

func setTracker(t *db.Tracker) {
	trackerMu.Lock()
	defer trackerMu.Unlock()
	tracker = t
}

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

	// Register command-line flags (parsed in main)
	flag.Uint64Var(&inactivityThresholdSec, "inactivityThreshold", inactivityThresholdSec, "The inactivity threshold in seconds")
	flag.StringVar(&listenInterface, "interface", listenInterface, "The interface to listen on (default is all interfaces)")
	flag.IntVar(&listenPort, "port", listenPort, "The port to listen on (default is 9183)")
	flag.BoolVar(&privateMode, "private", privateMode, "When true, the window title will be replaced with the process name for increased privacy")
	flag.BoolVar(&debugMode, "debug", debugMode, "When true, output all values to the console")
	flag.BoolVar(&mcpMode, "mcp", false, "Run as MCP stdio server instead of Prometheus exporter")
	flag.BoolVar(&silentMode, "silent", false, "Run without system tray icon")
	flag.BoolVar(&installMode, "install", false, "Install as a startup task (runs at logon)")
	flag.BoolVar(&uninstallMode, "uninstall", false, "Remove the startup task")
	flag.StringVar(&dbpath, "dbpath", "", "Directory for DB file(s) (default: same directory as the executable)")
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

func startPrometheus() error {
	promMu.Lock()
	defer promMu.Unlock()

	if promServer != nil {
		return nil // already running
	}

	if promReg == nil {
		promReg = setupMetrics()
	}

	addr := fmt.Sprintf("%s:%d", listenInterface, listenPort)
	promServer = &http.Server{
		Addr:    addr,
		Handler: promhttp.HandlerFor(promReg, promhttp.HandlerOpts{}),
	}

	go func() {
		log.Printf("Prometheus endpoint started on %s", addr)
		if err := promServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Prometheus server error: %v", err)
		}
	}()
	return nil
}

func stopPrometheus() {
	promMu.Lock()
	defer promMu.Unlock()

	if promServer == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	promServer.Shutdown(ctx)
	promServer = nil
	log.Printf("Prometheus endpoint stopped")
}

func runExporter(ctx context.Context) {
	path := dbpath
	if path == "" {
		path = db.ExeDir()
	}

	t, err := db.Open(path)
	if err != nil {
		log.Printf("Warning: DB tracking disabled: %v", err)
	} else {
		setTracker(t)
		log.Printf("DB path: %s", path)
	}

	// Store initial threshold in atomic for tray menu to update
	inactThresholdMs.Store(inactivityThresholdSec * 1000)

	log.Printf("Inactivity Threshold: %d seconds", inactivityThresholdSec)
	log.Printf("Private Mode: %v", privateMode)
	log.Printf("Debug Mode: %v", debugMode)

	// Initialize metrics registry (but don't start the server — user toggles it via tray)
	promMu.Lock()
	if promReg == nil {
		promReg = setupMetrics()
	}
	promMu.Unlock()

	windowinfo.LastWindowInfo, _ = windowinfo.GetActiveWindowInfo(*focusChangeCounter)
	windowinfo.LastWindowFocusTime = time.Now()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if paused.Load() {
				continue
			}
			// Avoid nil-interface trap: only pass tracker as interface when non-nil
			cur := getTracker()
			var ti windowinfo.FocusTracker
			if cur != nil {
				ti = cur
			}
			windowinfo.ProcessWindowInfo(inactThresholdMs.Load(), privateMode, debugMode, focusChangeCounter, focusedWindowDuration, meetingDuration, inactivityMetric, windowPidGauge, ti)
		}
	}
}

// elevateAndRun re-launches the current exe with admin privileges via UAC prompt.
func elevateAndRun() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := windows.UTF16PtrFromString("runas")
	exePtr, _ := windows.UTF16PtrFromString(exe)
	argsPtr, _ := windows.UTF16PtrFromString(args)

	err = windows.ShellExecute(0, verbPtr, exePtr, argsPtr, nil, windows.SW_NORMAL)
	if err != nil {
		return fmt.Errorf("UAC elevation failed: %w", err)
	}
	return nil
}

// isElevated checks if the current process has admin privileges.
func isElevated() bool {
	token := windows.GetCurrentProcessToken()
	elevated := false
	var elevation struct{ TokenIsElevated uint32 }
	var size uint32
	err := windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&elevation)), uint32(unsafe.Sizeof(elevation)), &size)
	if err == nil && elevation.TokenIsElevated != 0 {
		elevated = true
	}
	return elevated
}

func doInstall() error {
	if !isElevated() {
		fmt.Println("Requesting administrator privileges...")
		return elevateAndRun()
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not determine executable path: %w", err)
	}

	tr := `"` + exePath + `"`
	if dbpath != "" {
		tr += ` -dbpath "` + dbpath + `"`
	}

	cmd := exec.Command("schtasks", "/create",
		"/tn", "Timewarp",
		"/tr", tr,
		"/sc", "onlogon",
		"/rl", "limited",
		"/delay", "0000:30",
		"/f",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("schtasks create failed: %w", err)
	}
	fmt.Println("Timewarp installed as a startup task.")
	return nil
}

func doUninstall() error {
	if !isElevated() {
		fmt.Println("Requesting administrator privileges...")
		return elevateAndRun()
	}

	cmd := exec.Command("schtasks", "/delete", "/tn", "Timewarp", "/f")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("schtasks delete failed: %w", err)
	}
	fmt.Println("Timewarp startup task removed.")
	return nil
}

func main() {
	flag.Parse()

	if installMode {
		if err := doInstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Install error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if uninstallMode {
		if err := doUninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Uninstall error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if mcpMode {
		path := dbpath
		if path == "" {
			path = db.ExeDir()
		}
		if err := mcp.Run(path); err != nil {
			fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Onboarding: first-run DB folder selection
	if dbpath == "" && !silentMode {
		defaultDir := db.ExeDir()
		if !tray.HasExistingDB(defaultDir) {
			picked := tray.RunOnboarding()
			if picked == "" {
				fmt.Fprintln(os.Stderr, "No folder selected. Exiting.")
				os.Exit(0)
			}
			dbpath = picked
		}
	}

	// Onboarding: offer to create scheduled task
	if !silentMode && !tray.HasScheduledTask() {
		if tray.OfferScheduledTask() {
			if err := doInstall(); err != nil {
				log.Printf("Failed to create scheduled task: %v", err)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	if silentMode {
		runExporter(ctx)
		cancel()
		return
	}

	// Default: run with system tray icon
	path := dbpath
	if path == "" {
		path = db.ExeDir()
	}
	tray.Run(path, tray.Callbacks{
		OnReady: func() {
			go runExporter(ctx)
		},
		OnQuit: func() {
			cancel()
			stopPrometheus()
			time.Sleep(100 * time.Millisecond)
			if t := getTracker(); t != nil {
				t.Close()
			}
		},
		OnPause: func() {
			paused.Store(true)
		},
		OnResume: func() {
			paused.Store(false)
		},
		SetInactivityThreshold: func(seconds uint64) {
			inactThresholdMs.Store(seconds * 1000)
			log.Printf("Inactivity threshold changed to %d seconds", seconds)
		},
		OnPrometheusToggle: func(enable bool) {
			if enable {
				startPrometheus()
			} else {
				stopPrometheus()
			}
		},
		GetMCPConfig: func() string {
			exePath, err := os.Executable()
			if err != nil {
				return `{"error": "could not determine executable path"}`
			}
			p := dbpath
			if p == "" {
				p = db.ExeDir()
			}
			// Escape backslashes for JSON
			exeEsc := strings.ReplaceAll(exePath, `\`, `\\`)
			pathEsc := strings.ReplaceAll(p, `\`, `\\`)
			return fmt.Sprintf(`{
  "mcpServers": {
    "timewarp": {
      "command": "%s",
      "args": ["-mcp", "-dbpath", "%s"]
    }
  }
}`, exeEsc, pathEsc)
		},
		OnSetDBPath: func(newPath string) {
			// Close old tracker
			if t := getTracker(); t != nil {
				t.Close()
				setTracker(nil)
			}
			// Open new tracker
			t, err := db.Open(newPath)
			if err != nil {
				log.Printf("Failed to open DB at %s: %v", newPath, err)
				return
			}
			setTracker(t)
			dbpath = newPath
			log.Printf("DB path changed to: %s", newPath)
		},
	})
}
