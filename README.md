# Timewarp

Timewarp is a Windows app that silently tracks which applications and windows you focus on throughout the day, stores the data in a local SQLite database, and exposes it to Claude via an MCP server for automated timecard generation and productivity summaries.

It also exposes Prometheus metrics for real-time dashboarding.

## How It Works

1. **Timewarp runs in your system tray** and polls the active window every second using Windows API calls
2. **Sessions are stitched together** — brief alt-tabs and copy-paste switches under 10 seconds are discarded, and gaps under 30 seconds within the same app are bridged into a single session
3. **Project numbers are extracted** from window titles automatically (pattern: `YY-NNN`, e.g. `25-125`)
4. **Data is stored locally** in a SQLite database named `timewarp-HOSTNAME.db`
5. **Claude reads the data** via the MCP server to produce weekly timecard narratives

## Installation

### Build from source

```
git clone https://github.com/vinistoisr/timewarp.git
cd timewarp
go build -ldflags -H=windowsgui -o timewarp.exe .
```

### Install as a startup task

```
timewarp.exe -install -dbpath "C:\Users\You\OneDrive\TimewarpData"
```

This creates a Windows scheduled task that runs Timewarp at logon. To remove it:

```
timewarp.exe -uninstall
```

### Multi-machine sync

Point `-dbpath` at a OneDrive (or similar) folder. Each machine writes to its own `timewarp-HOSTNAME.db` file, so there are no sync conflicts. The MCP server reads all `timewarp-*.db` files in the directory and aggregates across machines automatically.

## MCP Server

Timewarp includes an MCP stdio server that exposes your focus data to Claude. Add this to your Claude Desktop config (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "timewarp": {
      "command": "C:\\path\\to\\timewarp.exe",
      "args": ["-mcp", "-dbpath", "C:\\Users\\You\\OneDrive\\TimewarpData"]
    }
  }
}
```

### Tools

| Tool | Description |
|------|-------------|
| `get_weekly_summary` | Returns attributed project time, unattributed app time, meetings, and inactivity for a given week. Claude uses this to write timecard narratives. |
| `get_focus_time` | Returns total focused minutes for a specific process across a date range. |
| `list_top_apps` | Returns top 10 processes by focused time for a given week. |

### Example: Weekly Summary

Ask Claude: *"Summarize my work this week for my timecard"*

Claude calls `get_weekly_summary` and receives structured data like:

```json
{
  "week": "2026-03-02/2026-03-08",
  "machines": ["DESKTOP-VINC", "LAPTOP-VINC"],
  "attributed": [
    {
      "project_number": "25-125",
      "total_minutes": 252,
      "processes": ["acad.exe", "OUTLOOK.EXE", "Bluebeam Revu"],
      "sample_titles": ["25-125_SLD-E101.dwg - AutoCAD"]
    }
  ],
  "unattributed": [
    { "process": "chrome.exe", "total_minutes": 48, "sample_titles": ["..."] }
  ],
  "meetings": [
    { "subject": "25-019 CATSA YVR Design Review", "total_minutes": 62, "sessions": 2 }
  ],
  "inactivity_minutes": 94
}
```

## Command-line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-dbpath` | Directory for database files | Same directory as executable |
| `-mcp` | Run as MCP stdio server (no GUI, no Prometheus) | `false` |
| `-silent` | Run without system tray icon | `false` |
| `-install` | Create a Windows startup scheduled task | |
| `-uninstall` | Remove the startup scheduled task | |
| `-inactivityThreshold` | Inactivity threshold in seconds | `60` |
| `-interface` | Network interface to listen on | All interfaces |
| `-port` | Prometheus metrics port | `9183` |
| `-private` | Replace window titles with process names in Prometheus labels | `false` |
| `-debug` | Print debug output to console | `false` |

## Prometheus Metrics

Timewarp exposes a Prometheus endpoint at `http://localhost:9183/metrics`:

| Metric | Type | Description |
|--------|------|-------------|
| `focused_window_pid` | Gauge | PID of the currently focused window |
| `focused_window_duration_seconds` | Counter | Total seconds focused per process |
| `focused_window_changes_total` | Counter | Number of focus changes |
| `focus_inactivity_seconds_total` | Counter | Total seconds of inactivity |
| `meeting_duration_seconds` | Counter | Seconds spent in meetings |

## Suppressed Processes

These processes are never recorded to the database (they produce noise, not useful data):

- `mstsc.exe` — RDP client (the remote machine captures the real activity)
- `ApplicationFrameHost.exe`, `ShellExperienceHost.exe`, `LockApp.exe`, `LogonUI.exe`

## Architecture

```
timewarp.exe
  |
  +-- System tray icon (default) or silent mode
  +-- Prometheus HTTP server (:9183)
  +-- SQLite writer (session stitching, project extraction)
  +-- MCP stdio server (-mcp flag)
       +-- Reads all timewarp-*.db files
       +-- JSON-RPC 2.0 over stdin/stdout
```
