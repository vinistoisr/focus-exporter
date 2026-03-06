# Timewarp

## TL;DR

**Timewarp helps you fill out your timesheets without keeping track of what you worked on.**

It runs quietly on your Windows PC and watches which window is in front — that's it. It notes the app name and window title (like `25-125_SLD-E101.dwg - AutoCAD`) and saves it to a local database on your own computer. Nothing is sent anywhere. No server, no cloud, no tracking dashboard. **Your manager cannot see this data. Nobody can, except you.**

When it's time to fill out your timecard, you open Claude and ask in plain English:

> *"What did I work on this week?"*

Claude reads your local data and tells you: "You spent 4 hours in AutoCAD on project 25-125, 1 hour in a design review meeting, 30 minutes on emails..." — and you copy that into your timesheet. Done.

**How it works under the hood:** Every second, Timewarp checks which window is active using the Windows API. It groups those seconds into sessions (ignoring brief alt-tabs), extracts project numbers from file names when possible, and stores everything in a local SQLite database. Claude accesses this database through a local MCP connection — the same way it accesses files on your computer — and can answer questions about your work history.

---

## Architecture

```mermaid
graph LR
    subgraph Your PC
        TW["🕐 Timewarp<br/><i>system tray app</i>"]
        DB[("💾 SQLite DB<br/><code>timewarp-*.db</code>")]
        CD["🤖 Claude Desktop"]

        TW -- "polls active window<br/>every 1 second" --> WIN["🪟 Windows API"]
        TW -- "writes sessions" --> DB
        CD -- "asks: what did I work on?" --> MCP["📡 MCP Server<br/><i>local stdio</i>"]
        MCP -- "reads" --> DB
    end

    OD["☁️ OneDrive<br/><i>(optional sync)</i>"]
    DB -. "sync DB folder<br/>across machines" .-> OD

    style TW fill:#5a3cdc,color:#fff,stroke:#5a3cdc
    style DB fill:#1a1832,color:#fff,stroke:#5a3cdc
    style MCP fill:#1a1832,color:#fff,stroke:#5a3cdc
    style CD fill:#d4a574,color:#1a1832,stroke:#d4a574
    style WIN fill:#333,color:#fff,stroke:#555
    style OD fill:#0078d4,color:#fff,stroke:#0078d4
```

> **Everything stays on your machine.** The MCP server is a local process that communicates with Claude Desktop over stdin/stdout — it does not open any network ports or send data anywhere.

---

## Installation

### Step 1: Download

Download `timewarp.exe` from the [latest release](https://github.com/vinistoisr/timewarp/releases/latest) and put it somewhere permanent (e.g. `C:\Tools\Timewarp\timewarp.exe`).

### Step 2: Choose a database folder

Timewarp stores your data in a folder you choose. You have two options:

| Option | Path example | Why |
|--------|-------------|-----|
| **Local only** | `C:\Users\You\TimewarpData` | Simple. Data stays on this PC. |
| **OneDrive** (recommended) | `C:\Users\You\OneDrive\TimewarpData` | Syncs across your work laptop and desktop automatically. Each machine writes its own file (`timewarp-DESKTOP.db`, `timewarp-LAPTOP.db`), so there are no sync conflicts. |

Create the folder now — Timewarp won't create it for you.

### Step 3: Run it

Double-click `timewarp.exe`. A small purple clock icon appears in your system tray — Timewarp is now tracking.

To set your database folder, right-click the tray icon and choose **Set DB Folder...**, then select the folder you created.

### Step 4: Start at login (optional)

Open a terminal and run:

```
timewarp.exe -install -dbpath "C:\Users\You\OneDrive\TimewarpData"
```

This creates a Windows scheduled task that starts Timewarp automatically when you log in. A UAC prompt will appear since it needs admin rights to create the task. To remove it later:

```
timewarp.exe -uninstall
```

### Step 5: Connect to Claude Desktop

This is what lets you ask Claude about your work. Right-click the tray icon and click **Copy MCP Config** — this copies the correct JSON to your clipboard.

Then open your Claude Desktop config file:

```
%APPDATA%\Claude\claude_desktop_config.json
```

Paste the config inside the `"mcpServers"` section. The result should look something like this:

```json
{
  "mcpServers": {
    "timewarp": {
      "command": "C:\\Tools\\Timewarp\\timewarp.exe",
      "args": ["-mcp", "-dbpath", "C:\\Users\\You\\OneDrive\\TimewarpData"]
    }
  }
}
```

Restart Claude Desktop. You should see "timewarp" listed as a connected MCP server. Now you can ask things like:

- *"What did I work on this week?"*
- *"How much time did I spend in AutoCAD?"*
- *"Break down my week by day for my timecard"*
- *"List my top apps this week"*

---

## What to Ask Claude

Once Timewarp is running and connected to Claude Desktop, just ask in plain English. Here are some examples:

| What you want | What to type |
|---------------|-------------|
| Fill out your weekly timecard | *"Summarize my work this week for my timecard"* |
| Daily breakdown for QuickBooks Time | *"Break down my work by day this week, with hours per project"* |
| Check time on a specific project | *"How much time did I spend on project 25-125 this week?"* |
| See what you did on a specific day | *"What did I work on last Tuesday?"* |
| Find out where your time went | *"What were my top apps this week and how long did I use each?"* |
| Generate a timesheet narrative | *"Write a timesheet entry for this week, grouped by project number"* |
| Check meeting time | *"How many hours of meetings did I have this week?"* |
| Compare weeks | *"Compare my project time this week vs last week"* |
| Catch untracked time | *"I was on site Monday with no laptop — can you show me Tuesday through Friday only?"* |

Claude will call the appropriate Timewarp tools automatically. You don't need to know the tool names or syntax — just describe what you need.

---

## System Tray Menu

Right-click the tray icon to access these options:

| Menu Item | What it does |
|-----------|-------------|
| **Stop Tracking / Start Tracking** | Pause and resume tracking. The icon turns grey when paused. |
| **Inactivity Threshold** | How long you must be idle before time stops counting (30s, 1m, 2m, 5m, 10m). Default: 1 minute. |
| **Enable Prometheus Endpoint** | Toggle the Prometheus metrics HTTP server on/off (off by default). |
| **Open DB Folder** | Opens your database folder in Explorer. |
| **Set DB Folder...** | Pick a new folder for the database. Takes effect immediately. |
| **Copy MCP Config** | Copies the Claude Desktop MCP config JSON to your clipboard. |
| **Quit** | Stops Timewarp. |

---

## MCP Tools

These are the tools Claude can call to query your data:

| Tool | Description |
|------|-------------|
| `get_weekly_summary` | Attributed project time, unattributed app time, meetings, and inactivity for a week. |
| `get_daily_breakdown` | Same data broken down by day — ideal for filling out daily timecards or QuickBooks Time. |
| `get_focus_time` | Total focused minutes for a specific process across a date range. |
| `list_top_apps` | Top 10 processes by focused time for a week. |

### Example: Weekly Summary

Ask Claude: *"Summarize my work this week for my timecard"*

Claude calls `get_weekly_summary` and receives:

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
    { "subject": "25-019 Design Review", "total_minutes": 62, "sessions": 2 }
  ],
  "inactivity_minutes": 94
}
```

---

## How Sessions Work

Timewarp doesn't just record every second individually. It stitches activity into sessions:

- **Gaps under 30 seconds** in the same app are bridged (you alt-tabbed to copy something and came back)
- **Sessions under 10 seconds** are discarded (you accidentally clicked the wrong window)
- **Project numbers** are extracted from window titles using the pattern `YY-NNN` (e.g. `25-125` from `25-125_SLD-E101.dwg`)
- **Meetings** are detected from Teams/Zoom/Webex window titles
- **Suppressed processes** like `mstsc.exe` (Remote Desktop), `LockApp.exe`, and `ShellExperienceHost.exe` are never recorded

---

## Command-line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-dbpath` | Directory for database files | Same directory as executable |
| `-mcp` | Run as MCP stdio server (used by Claude Desktop) | `false` |
| `-install` | Create a Windows startup scheduled task | |
| `-uninstall` | Remove the startup scheduled task | |
| `-silent` | Run without system tray icon | `false` |
| `-inactivityThreshold` | Inactivity threshold in seconds | `60` |
| `-interface` | Network interface for Prometheus | All interfaces |
| `-port` | Prometheus metrics port | `9183` |
| `-private` | Replace window titles with process names in metrics | `false` |
| `-debug` | Print debug output to console | `false` |

---

## Prometheus Metrics (Optional)

Timewarp can expose a Prometheus endpoint for real-time dashboarding. It is **off by default** — enable it from the tray menu or by toggling it on.

Endpoint: `http://localhost:9183/metrics`

| Metric | Type | Description |
|--------|------|-------------|
| `focused_window_pid` | Gauge | PID of the currently focused window |
| `focused_window_duration_seconds` | Counter | Total seconds focused per process |
| `focused_window_changes_total` | Counter | Number of focus changes |
| `focus_inactivity_seconds_total` | Counter | Total seconds of inactivity |
| `meeting_duration_seconds` | Counter | Seconds spent in meetings |

---

## Building from Source

```
git clone https://github.com/vinistoisr/timewarp.git
cd timewarp
go build -ldflags "-H=windowsgui" -o timewarp.exe .
```

Requires Go 1.24+.
