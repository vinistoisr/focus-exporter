# Focus Exporter

## Introduction

Focus Exporter is a tool inspired by Microsoft Viva Insights, designed to help you collect and monitor metrics about your time spent in various applications and meetings on Windows. This project was a great opportunity to learn Go and delve into the world of performance metrics.

Heavily inspired by the Windows Exporter, Focus Exporter uses a combination of Windows API calls and Prometheus to collect and expose metrics about active windows, user inactivity, and focused window durations. 

## Motivation

Microsoft Viva Insights offers great metrics, but they are part of a broader suite of services. Focus Exporter is my effort to collect these metrics independently. This project allowed me to learn Go and leverage the power of AI tools like GitHub Copilot, Google AI Studio, and ChatGPT.

## Features

- Collects metrics about focused windows and user inactivity.
- Tracks time spent in meetings for applications like Microsoft Teams and Zoom.
- Exposes metrics through a Prometheus endpoint.
- Configurable through command-line parameters.
- Supports privacy mode to avoid exposing sensitive window titles.

## Usage

### Command-line Parameters

- `-inactivityThreshold`: The threshold in seconds for detecting user inactivity (default: 60 seconds).
- `-interface`: The network interface to listen on (default: all interfaces).
- `-port`: The port to listen on (default: 9183).
- `-private`: When true, the window title will be replaced with the process name for increased privacy.
- `-debug`: When true, output all values to the console.

### Example Commands

#### Default Usage

```focus-exporter```

#### Custom Inactivity Threshold

```focus-exporter -inactivityThreshold 120```

#### Specify Network Interface and Port

```focus-exporter -interface 192.168.1.1 -port 9090```

#### Enable Privacy Mode

```focus-exporter -private```

#### Enable Debug Mode

```focus-exporter -debug```

## Metrics

Focus Exporter exposes the following metrics:

- `focused_window_pid`: Process ID of the currently focused window.
- `focus_inactivity_seconds_total`: Total seconds of user inactivity.
- `focused_window_changes_total`: Total number of times the focused window has changed.
- `focused_window_duration_seconds`: Duration in seconds the window has been focused.
- `meeting_duration_seconds`: Duration in seconds spent in a meeting.

## Installation

1. Clone the repository:
    ```git clone https://github.com/yourusername/focus-exporter.git```

2. Navigate to the project directory:
    ```cd focus-exporter```

3. Build the project:
    ```go build -o focus-exporter main.go```

4. Run the exporter:
    ```./focus-exporter```

## Acknowledgments

This project wouldn't have been possible without the help of several AI tools and open-source libraries:

- **GitHub Copilot**: For code suggestions and completions.
- **Google AI Studio**: For powerful AI insights and recommendations.
- **ChatGPT**: For answering countless questions and providing guidance.

### Libraries Used

- **Prometheus Client Golang**: For metrics collection and exposition.
- **golang/sys/windows**: For Windows system calls and API interactions.

Special thanks to the authors of these libraries and the broader open-source community for their invaluable contributions.


