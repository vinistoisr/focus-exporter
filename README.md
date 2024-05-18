# Focus Exporter

## Introduction

Focus Exporter is a Prometheus Exporter for Windows, designed to collect and monitor metrics about your time spent on Windows. 

Heavily inspired by the [Windows Exporter](https://github.com/prometheus-community/windows_exporter), Focus Exporter uses a combination of Windows API calls to collect and expose metrics about active windows, user inactivity, and focused window durations.

By default, exposes a prometheus metrics endpoing at ```http://localhost:9183/metrics```

## Motivation

Microsoft Viva Insights offers great metrics, but they are part of a broader suite of services through office 365. Focus Exporter is my effort to collect a subset of these metrics independently. This project is my first project in Golang, any feedback is very appreciated. 

## Features

- Collects metrics about focused windows and user inactivity.
- Attempts to track time spent in meetings for applications like Microsoft Teams and Zoom.
- Exposes metrics through a Prometheus endpoint.
- Configurable through command-line parameters.
- privacy mode to avoid exposing sensitive window titles.

## Usage

Serves an endpoint at ```http://$host:$port/metrics``` that must be scraped by a [Prometheus](https://github.com/prometheus-community) metrics server. Data can be visualized in a program such as [Grafana](https://github.com/grafana/grafana). Example dashboards coming soon. 

This does _not_ need to be ran as a Priviledged user. 

A scheduled task can be used to run this service at logon. 

### Command-line Parameters

- `-inactivityThreshold`: The threshold in seconds for detecting user inactivity (default: 60 seconds).
- `-interface`: The network interface to listen on (default: all interfaces).
- `-port`: The port to listen on (default: 9183).
- `-private`: When true, the window title will be replaced with the process name for increased privacy.
- `-debug`: When true, output all values to the console.

### Example Commands

#### Default Usage

```focus-exporter``` This will start a server at ```http://localhost:9183/metrics``` and expose all metrics.

#### Custom Inactivity Threshold

```focus-exporter -inactivityThreshold 120``` This will start a server at ```http://localhost:9183/metrics``` with an inactivity threshold of 2 minutes. 

#### Specify Network Interface and Port

```focus-exporter -interface 192.168.1.1 -port 9090``` This will start a server at ```http://localhost:9000/metrics``` and expose all metrics.

#### Enable Privacy Mode

```focus-exporter -private``` This will start a server at ```http://localhost:9183/metrics``` and will not expose the full Window_Title in metric labels.

#### Enable Debug Mode

```focus-exporter -debug``` This will start a server with debugging, which prints collected values to the console. 

## Metrics

Focus Exporter exposes the following metrics:

- `focused_window_pid`: Process ID of the currently focused window.
- `focus_inactivity_seconds_total`: Total seconds of user inactivity.
- `focused_window_changes_total`: Total number of times the focused window has changed.
- `focused_window_duration_seconds`: Duration in seconds the window has been focused.
- `meeting_duration_seconds`: Duration in seconds spent in a meeting.
- Standard suite of go application metrics as collected by Prometheus Golang client Library

## Installation

1. Clone the repository:
    ```git clone https://github.com/yourusername/focus-exporter.git```

2. Navigate to the project directory:
    ```cd focus-exporter```

3. Build the project:
    ```go build -ldflags -H=windowsgui exporter.go```

4. Run the exporter:
    ```./exporter.exe```
   (with optional flags):
    ```./exporter.exe -debug -interface <ip or hostname> -port <port> --inactivityThreshold -private```

## Acknowledgments

This project wouldn't have been possible without the help of several AI tools and open-source libraries:

- **GitHub Copilot**
- **Google AI Studio**
- **ChatGPT**

### Libraries Used

- **Prometheus Client Golang**: For metrics collection and exposition.
- **golang/sys/windows**: For Windows system calls and API interactions.

Special thanks to the authors of these libraries and the broader open-source community.


