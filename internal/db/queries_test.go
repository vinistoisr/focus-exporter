package db

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func seedTestDB(t *testing.T, dir string, hostname string) {
	t.Helper()

	dbFile := filepath.Join(dir, "timewarp-"+hostname+".db")
	dsn := "file:" + dbFile + "?_journal_mode=WAL"
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.SetMaxOpenConns(1)
	initSchema(d)

	// Monday 2026-03-02 through Friday 2026-03-06
	base := time.Date(2026, 3, 2, 9, 0, 0, 0, time.UTC)

	// Attributed: project 25-125, acad.exe, 2 hours
	d.Exec(`INSERT INTO focus_events (hostname, username, process_name, window_title, project_number, started_at, ended_at, duration_seconds) VALUES (?,?,?,?,?,?,?,?)`,
		hostname, "user", "acad.exe", "25-125_SLD-E101.dwg - AutoCAD", "25-125",
		base, base.Add(2*time.Hour), 7200.0)

	// Attributed: project 25-125, OUTLOOK.EXE, 30 min
	d.Exec(`INSERT INTO focus_events (hostname, username, process_name, window_title, project_number, started_at, ended_at, duration_seconds) VALUES (?,?,?,?,?,?,?,?)`,
		hostname, "user", "OUTLOOK.EXE", "RE: 25-125 RFI #12", "25-125",
		base.Add(2*time.Hour), base.Add(150*time.Minute), 1800.0)

	// Unattributed: chrome, 45 min
	d.Exec(`INSERT INTO focus_events (hostname, username, process_name, window_title, project_number, started_at, ended_at, duration_seconds) VALUES (?,?,?,?,?,?,?,?)`,
		hostname, "user", "chrome.exe", "ESPN - NBA Scores", nil,
		base.Add(3*time.Hour), base.Add(225*time.Minute), 2700.0)

	// Meeting: 1 hour
	d.Exec(`INSERT INTO meeting_sessions (hostname, username, process_name, subject, started_at, ended_at, duration_seconds) VALUES (?,?,?,?,?,?,?)`,
		hostname, "user", "ms-teams.exe", "25-019 Design Review",
		base.Add(4*time.Hour), base.Add(5*time.Hour), 3600.0)

	// Inactivity: 30 min
	d.Exec(`INSERT INTO inactivity_periods (hostname, username, started_at, ended_at, duration_seconds) VALUES (?,?,?,?,?)`,
		hostname, "user",
		base.Add(5*time.Hour), base.Add(330*time.Minute), 1800.0)
}

func TestGetWeeklySummary(t *testing.T) {
	dir := t.TempDir()
	seedTestDB(t, dir, "DESKTOP-TEST")

	weekStart := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	raw, err := GetWeeklySummary(dir, weekStart)
	if err != nil {
		t.Fatal(err)
	}

	var summary WeeklySummary
	if err := json.Unmarshal(raw, &summary); err != nil {
		t.Fatal(err)
	}

	if summary.Week != "2026-03-02/2026-03-09" {
		t.Errorf("unexpected week: %s", summary.Week)
	}

	if len(summary.Machines) != 1 || summary.Machines[0] != "DESKTOP-TEST" {
		t.Errorf("unexpected machines: %v", summary.Machines)
	}

	if len(summary.Attributed) != 1 {
		t.Fatalf("expected 1 attributed project, got %d", len(summary.Attributed))
	}
	proj := summary.Attributed[0]
	if proj.ProjectNumber != "25-125" {
		t.Errorf("unexpected project number: %s", proj.ProjectNumber)
	}
	if proj.TotalMinutes != 150 {
		t.Errorf("expected 150 minutes for 25-125, got %v", proj.TotalMinutes)
	}

	if len(summary.Unattributed) != 1 {
		t.Fatalf("expected 1 unattributed app, got %d", len(summary.Unattributed))
	}
	if summary.Unattributed[0].TotalMinutes != 45 {
		t.Errorf("expected 45 minutes unattributed chrome, got %v", summary.Unattributed[0].TotalMinutes)
	}

	if len(summary.Meetings) != 1 {
		t.Fatalf("expected 1 meeting, got %d", len(summary.Meetings))
	}
	if summary.Meetings[0].TotalMinutes != 60 {
		t.Errorf("expected 60 minute meeting, got %v", summary.Meetings[0].TotalMinutes)
	}

	if summary.InactivityMinutes != 30 {
		t.Errorf("expected 30 min inactivity, got %v", summary.InactivityMinutes)
	}
}

func TestGetWeeklySummary_MultiMachine(t *testing.T) {
	dir := t.TempDir()
	seedTestDB(t, dir, "DESKTOP-A")
	seedTestDB(t, dir, "LAPTOP-B")

	weekStart := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	raw, err := GetWeeklySummary(dir, weekStart)
	if err != nil {
		t.Fatal(err)
	}

	var summary WeeklySummary
	json.Unmarshal(raw, &summary)

	if len(summary.Machines) != 2 {
		t.Errorf("expected 2 machines, got %d", len(summary.Machines))
	}

	// Project minutes should be doubled (both machines have same data)
	if len(summary.Attributed) != 1 {
		t.Fatalf("expected 1 attributed project, got %d", len(summary.Attributed))
	}
	if summary.Attributed[0].TotalMinutes != 300 {
		t.Errorf("expected 300 minutes (2x150), got %v", summary.Attributed[0].TotalMinutes)
	}
}

func TestGetFocusTime(t *testing.T) {
	dir := t.TempDir()
	seedTestDB(t, dir, "HOST")

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)

	raw, err := GetFocusTime(dir, "acad.exe", from, to)
	if err != nil {
		t.Fatal(err)
	}

	var result FocusTimeResult
	json.Unmarshal(raw, &result)

	if result.TotalMinutes != 120 {
		t.Errorf("expected 120 minutes for acad.exe, got %v", result.TotalMinutes)
	}
}

func TestGetFocusTime_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	seedTestDB(t, dir, "HOST")

	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)

	raw, err := GetFocusTime(dir, "ACAD.EXE", from, to)
	if err != nil {
		t.Fatal(err)
	}

	var result FocusTimeResult
	json.Unmarshal(raw, &result)

	if result.TotalMinutes != 120 {
		t.Errorf("expected 120 minutes (case insensitive), got %v", result.TotalMinutes)
	}
}

func TestListTopApps(t *testing.T) {
	dir := t.TempDir()
	seedTestDB(t, dir, "HOST")

	weekStart := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	raw, err := ListTopApps(dir, weekStart)
	if err != nil {
		t.Fatal(err)
	}

	var apps []TopApp
	json.Unmarshal(raw, &apps)

	if len(apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(apps))
	}

	// Should be sorted by time descending: acad (120m) > chrome (45m) > outlook (30m)
	if apps[0].ProcessName != "acad.exe" {
		t.Errorf("expected acad.exe first, got %s", apps[0].ProcessName)
	}
}

func TestParseWeekStart_Valid(t *testing.T) {
	ws, err := ParseWeekStart("2026-03-02")
	if err != nil {
		t.Fatal(err)
	}
	if ws.Weekday() != time.Monday {
		t.Errorf("expected Monday, got %s", ws.Weekday())
	}
}

func TestParseWeekStart_NotMonday(t *testing.T) {
	_, err := ParseWeekStart("2026-03-03") // Tuesday
	if err == nil {
		t.Error("expected error for non-Monday date")
	}
}

func TestGetWeeklySummary_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	weekStart := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	_, err := GetWeeklySummary(dir, weekStart)
	if err == nil {
		t.Error("expected error for empty directory")
	}
}
