package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type WeeklySummary struct {
	Week               string              `json:"week"`
	Machines           []string            `json:"machines"`
	Attributed         []AttributedProject `json:"attributed"`
	Unattributed       []UnattributedApp   `json:"unattributed"`
	Meetings           []MeetingSummary    `json:"meetings"`
	InactivityMinutes  float64             `json:"inactivity_minutes"`
}

type AttributedProject struct {
	ProjectNumber string   `json:"project_number"`
	TotalMinutes  float64  `json:"total_minutes"`
	Processes     []string `json:"processes"`
	SampleTitles  []string `json:"sample_titles"`
}

type UnattributedApp struct {
	Process      string   `json:"process"`
	TotalMinutes float64  `json:"total_minutes"`
	SampleTitles []string `json:"sample_titles"`
}

type MeetingSummary struct {
	Subject      string  `json:"subject"`
	TotalMinutes float64 `json:"total_minutes"`
	Sessions     int     `json:"sessions"`
}

type FocusTimeResult struct {
	ProcessName  string  `json:"process_name"`
	TotalMinutes float64 `json:"total_minutes"`
	DateFrom     string  `json:"date_from"`
	DateTo       string  `json:"date_to"`
}

type TopApp struct {
	ProcessName  string  `json:"process_name"`
	TotalMinutes float64 `json:"total_minutes"`
}

// openAllDBs opens all focus-*.db files in the given directory.
func openAllDBs(dbpath string) ([]*sql.DB, error) {
	pattern := filepath.Join(dbpath, "timewarp-*.db")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no focus-*.db files found in %s", dbpath)
	}

	var dbs []*sql.DB
	for _, m := range matches {
		dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&mode=ro", m)
		d, err := sql.Open("sqlite", dsn)
		if err != nil {
			d.Close()
			for _, prev := range dbs {
				prev.Close()
			}
			return nil, fmt.Errorf("open %s: %w", m, err)
		}
		d.SetMaxOpenConns(1)
		dbs = append(dbs, d)
	}
	return dbs, nil
}

func GetWeeklySummary(dbpath string, weekStart time.Time) (json.RawMessage, error) {
	dbs, err := openAllDBs(dbpath)
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, d := range dbs {
			d.Close()
		}
	}()

	weekEnd := weekStart.AddDate(0, 0, 7)
	weekStr := fmt.Sprintf("%s/%s", weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02"))

	machineSet := map[string]bool{}
	// project_number -> aggregation
	type projAgg struct {
		minutes    float64
		processes  map[string]bool
		titles     []string
	}
	attributed := map[string]*projAgg{}

	// process -> aggregation (unattributed)
	type appAgg struct {
		minutes float64
		titles  []string
	}
	unattributed := map[string]*appAgg{}

	// meeting subject -> aggregation
	type mtgAgg struct {
		minutes  float64
		sessions int
	}
	meetings := map[string]*mtgAgg{}

	var totalInactivity float64

	startStr := weekStart.Format("2006-01-02 15:04:05")
	endStr := weekEnd.Format("2006-01-02 15:04:05")

	for _, db := range dbs {
		// Machines
		rows, err := db.Query(`SELECT DISTINCT hostname FROM focus_events WHERE started_at >= ? AND started_at < ?`, startStr, endStr)
		if err == nil {
			for rows.Next() {
				var h string
				rows.Scan(&h)
				machineSet[h] = true
			}
			rows.Close()
		}

		// Focus events
		rows, err = db.Query(`SELECT process_name, window_title, project_number, duration_seconds FROM focus_events WHERE started_at >= ? AND started_at < ?`, startStr, endStr)
		if err == nil {
			for rows.Next() {
				var proc, title string
				var projNum sql.NullString
				var dur float64
				rows.Scan(&proc, &title, &projNum, &dur)
				mins := dur / 60.0

				if projNum.Valid && projNum.String != "" {
					agg, ok := attributed[projNum.String]
					if !ok {
						agg = &projAgg{processes: map[string]bool{}}
						attributed[projNum.String] = agg
					}
					agg.minutes += mins
					agg.processes[proc] = true
					if len(agg.titles) < 3 {
						agg.titles = append(agg.titles, title)
					}
				} else {
					agg, ok := unattributed[proc]
					if !ok {
						agg = &appAgg{}
						unattributed[proc] = agg
					}
					agg.minutes += mins
					if len(agg.titles) < 3 {
						agg.titles = append(agg.titles, title)
					}
				}
			}
			rows.Close()
		}

		// Meetings
		rows, err = db.Query(`SELECT subject, duration_seconds FROM meeting_sessions WHERE started_at >= ? AND started_at < ?`, startStr, endStr)
		if err == nil {
			for rows.Next() {
				var subj string
				var dur float64
				rows.Scan(&subj, &dur)
				agg, ok := meetings[subj]
				if !ok {
					agg = &mtgAgg{}
					meetings[subj] = agg
				}
				agg.minutes += dur / 60.0
				agg.sessions++
			}
			rows.Close()
		}

		// Inactivity
		row := db.QueryRow(`SELECT COALESCE(SUM(duration_seconds), 0) FROM inactivity_periods WHERE started_at >= ? AND started_at < ?`, startStr, endStr)
		var inact float64
		if row.Scan(&inact) == nil {
			totalInactivity += inact / 60.0
		}
	}

	// Build result
	var machines []string
	for m := range machineSet {
		machines = append(machines, m)
	}

	var attrList []AttributedProject
	for pn, agg := range attributed {
		var procs []string
		for p := range agg.processes {
			procs = append(procs, p)
		}
		attrList = append(attrList, AttributedProject{
			ProjectNumber: pn,
			TotalMinutes:  round1(agg.minutes),
			Processes:     procs,
			SampleTitles:  agg.titles,
		})
	}

	var unattrList []UnattributedApp
	for proc, agg := range unattributed {
		unattrList = append(unattrList, UnattributedApp{
			Process:      proc,
			TotalMinutes: round1(agg.minutes),
			SampleTitles: agg.titles,
		})
	}

	var mtgList []MeetingSummary
	for subj, agg := range meetings {
		mtgList = append(mtgList, MeetingSummary{
			Subject:      subj,
			TotalMinutes: round1(agg.minutes),
			Sessions:     agg.sessions,
		})
	}

	summary := WeeklySummary{
		Week:              weekStr,
		Machines:          machines,
		Attributed:        attrList,
		Unattributed:      unattrList,
		Meetings:          mtgList,
		InactivityMinutes: round1(totalInactivity),
	}

	return json.Marshal(summary)
}

func GetFocusTime(dbpath string, processName string, dateFrom, dateTo time.Time) (json.RawMessage, error) {
	dbs, err := openAllDBs(dbpath)
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, d := range dbs {
			d.Close()
		}
	}()

	fromStr := dateFrom.Format("2006-01-02 15:04:05")
	toStr := dateTo.Format("2006-01-02 15:04:05")
	procLower := strings.ToLower(processName)

	var totalSeconds float64
	for _, db := range dbs {
		row := db.QueryRow(
			`SELECT COALESCE(SUM(duration_seconds), 0) FROM focus_events WHERE LOWER(process_name) = ? AND started_at >= ? AND started_at < ?`,
			procLower, fromStr, toStr,
		)
		var s float64
		if row.Scan(&s) == nil {
			totalSeconds += s
		}
	}

	result := FocusTimeResult{
		ProcessName:  processName,
		TotalMinutes: round1(totalSeconds / 60.0),
		DateFrom:     dateFrom.Format("2006-01-02"),
		DateTo:       dateTo.Format("2006-01-02"),
	}
	return json.Marshal(result)
}

func ListTopApps(dbpath string, weekStart time.Time) (json.RawMessage, error) {
	dbs, err := openAllDBs(dbpath)
	if err != nil {
		return nil, err
	}
	defer func() {
		for _, d := range dbs {
			d.Close()
		}
	}()

	weekEnd := weekStart.AddDate(0, 0, 7)
	startStr := weekStart.Format("2006-01-02 15:04:05")
	endStr := weekEnd.Format("2006-01-02 15:04:05")

	totals := map[string]float64{}
	for _, db := range dbs {
		rows, err := db.Query(
			`SELECT process_name, SUM(duration_seconds) FROM focus_events WHERE started_at >= ? AND started_at < ? GROUP BY process_name`,
			startStr, endStr,
		)
		if err != nil {
			continue
		}
		for rows.Next() {
			var proc string
			var dur float64
			rows.Scan(&proc, &dur)
			totals[proc] += dur
		}
		rows.Close()
	}

	// Sort by total and take top 10
	type kv struct {
		proc string
		dur  float64
	}
	var sorted []kv
	for p, d := range totals {
		sorted = append(sorted, kv{p, d})
	}
	// Simple selection sort for <=N items
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].dur > sorted[i].dur {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	if len(sorted) > 10 {
		sorted = sorted[:10]
	}

	var result []TopApp
	for _, s := range sorted {
		result = append(result, TopApp{
			ProcessName:  s.proc,
			TotalMinutes: round1(s.dur / 60.0),
		})
	}

	return json.Marshal(result)
}

// currentWeekMonday returns the Monday of the current week.
func CurrentWeekMonday() time.Time {
	now := time.Now().UTC()
	weekday := now.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := now.AddDate(0, 0, -int(weekday-time.Monday))
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}

// ParseWeekStart parses an ISO date string and ensures it's a Monday.
func ParseWeekStart(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date: %w", err)
	}
	if t.Weekday() != time.Monday {
		return time.Time{}, fmt.Errorf("date %s is not a Monday", s)
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), nil
}

// ExeDir returns the directory of the running executable.
func ExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func round1(f float64) float64 {
	return math.Round(f*10) / 10
}
