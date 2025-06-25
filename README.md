# VLSA - Visual Log Source Analyzer

A powerful CLI tool that bridges the gap between log analysis and source code debugging. VLSA allows you to step through logs as if you're debugging a live program by automatically correlating log messages with their corresponding source code locations.

## Overview

VLSA transforms the tedious process of log analysis by providing a split-pane terminal interface where you can:
- Browse logs in a structured table format (left pane)
- View matching source code automatically (right pane)
- Navigate through logs like stepping through a debugger

Perfect for debugging production issues, analyzing Datadog exports, or understanding application flow through log traces.

## Features

### 🔍 **Smart Source Code Correlation**
- Automatically searches your codebase for log message origins using ripgrep
- Intelligently parses JSON logs and formatted strings to extract meaningful search terms
- Excludes log files from source searches to avoid false matches

### 📊 **Dual Input Support**
- **CSV Logs**: Import Datadog exports or other structured log formats
- **Plain Text**: Analyze simple text-based log files
- Automatic format detection and parsing

### 🖥️ **Interactive TUI Interface**
- Split-pane layout: logs on left, source code on right
- Keyboard-driven navigation for efficient analysis
- Real-time source code updates as you navigate logs
- Progress indicator during log processing

### ⚡ **Efficient Log Management**
- Delete irrelevant logs on-the-fly (`d` key)
- Quick navigation between logs and source views (`Tab`)
- Timestamp parsing and display for temporal analysis

### 🎯 **Multiple Source Selection**
- When multiple source files match a log message, choose the correct one
- Interactive source selector with keyboard navigation
- Apply source selection to all similar log messages with one command
- Three-pane layout when multiple sources are available

## Installation

### Prerequisites
- Go 1.24.0 or later
- [ripgrep](https://github.com/BurntSushi/ripgrep) (`rg` command)

### Install ripgrep
```bash
# macOS
brew install ripgrep

# Ubuntu/Debian
sudo apt install ripgrep

# Windows
winget install BurntSushi.ripgrep.MSVC
```

### Build VLSA
```bash
git clone <repository-url>
cd lsa
go build -o vlsa main.go
```

## Usage

### Basic Usage
```bash
# Analyze a CSV log file (e.g., Datadog export)
./vlsa logs.csv

# Analyze a plain text log file
./vlsa application.log
```

### CSV Format Support
VLSA expects CSV files with the following columns:
- Column 0: Timestamp (ISO 8601 format: `2006-01-02T15:04:05.000Z`)
- Column 1: (Optional additional data)
- Column 2: Service name
- Column 3: Log message

Example CSV row:
```csv
2025-06-19T03:40:54.794Z,INFO,user-service,"User login attempt for user@example.com"
```

## Interface Guide

### Keyboard Shortcuts
| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Switch between panes (logs, sources, source selector) |
| `↑` / `↓` | Navigate through log entries or source options |
| `s` | Show source selector (when multiple sources available) |
| `Enter` | Select source (in selector) or open in editor (in source view) |
| `a` | Apply selected source to all similar logs (in selector) |
| `Esc` | Cancel source selection and return to source view |
| `d` / `Delete` / `Backspace` | Remove current log entry |
| `q` / `Ctrl+C` | Quit application |

### Interface Layout
```
┌─ VLSA - Visual Log Source Analyzer ─────────────────────────────────┐
│ ┌─ Logs ──────────────────┐ ┌─ Source Code ─────────────────────┐ │
│ │ Timestamp    │ Message  │ │ ./src/auth.go:42                  │ │
│ │ 03:40:54     │ Login... │ │ func authenticateUser(email str.. │ │
│ │ 03:41:02     │ Failed.. │ │     log.Info("User login attempt  │ │
│ │ 03:41:15     │ Success. │ │         for %s", email)           │ │
│ └──────────────────────────┘ └───────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

## Use Cases

### 🐛 **Production Debugging**
Export logs from your monitoring system (Datadog, CloudWatch, etc.) and quickly locate the source code responsible for errors or unexpected behavior.

### 📈 **Performance Analysis**
Step through performance-related logs while viewing the corresponding code to identify bottlenecks and optimization opportunities.

### 🔄 **Code Flow Understanding**
Trace application execution flow by following logs chronologically while seeing the actual code that generated each log entry.

### 🧪 **Testing & Validation**
Verify that your application is logging appropriately by correlating expected log messages with their source implementations.

## Technical Details

### Architecture
- **Framework**: Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) TUI framework
- **Components**: Uses Charm's [Bubbles](https://github.com/charmbracelet/bubbles) for table and viewport components
- **Search Engine**: Leverages [ripgrep](https://github.com/BurntSushi/ripgrep) for fast source code searching
- **Styling**: [Lipgloss](https://github.com/charmbracelet/lipgloss) for terminal styling

### Log Processing
1. **Parse Input**: Detect and parse CSV or plain text format
2. **Extract Messages**: Clean log messages by removing JSON formatting and extracting meaningful text
3. **Source Search**: Use ripgrep to find matching source code locations
4. **Build Interface**: Create interactive table and source view components

### Performance
- Asynchronous log processing with progress indication
- Efficient source code searching using ripgrep's optimized algorithms
- Memory-efficient handling of large log files

## Contributing

Contributions are welcome! Areas for improvement:
- Additional log format support
- External editor integration
- Enhanced source code context display
- Log filtering and search capabilities

## License

[Add your license information here]

## Acknowledgments

Built with the excellent [Charm](https://charm.sh/) ecosystem:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling

Special thanks to the [ripgrep](https://github.com/BurntSushi/ripgrep) project for blazing-fast text searching.
