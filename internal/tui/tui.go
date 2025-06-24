package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"slices"

	"vlsa/internal/bus"
	"vlsa/internal/log"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	progressBarWidth  = 71
	progressFullChar  = "█"
	progressEmptyChar = "░"
	dotChar           = " • "
)

var (
	keywordStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	subtleStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	ticksStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("79"))
	checkboxStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	progressEmpty      = subtleStyle.Render(progressEmptyChar)
	dotStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(dotChar)
	mainStyle          = lipgloss.NewStyle().MarginLeft(2)
	focusedWindowStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("63"))
	modelStyle         = lipgloss.NewStyle().BorderStyle(lipgloss.HiddenBorder())
)

// Model of the application state
type Model struct {
	// Application state
	logs          []log.Log
	currentLogIdx int

	// UI specific fields
	x             int
	y             int
	logTable      table.Model
	sourcesView   viewport.Model
	currentWindow int // 0 is logs, 1 is sources
	progress      int
	quit          bool
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	// Use the log table cursor as the source of truth for the current log index

	if m.currentWindow == 0 {
		m.logTable, cmd = m.logTable.Update(msg)
	}

	switch msg := msg.(type) {
	case log.LogProcessingMsg:
		m.progress = msg.Progress
		if m.progress >= 100 {
			m.logs = msg.Logs
			m.logTable = createLogTable(m.logs)
			m.logTable.KeyMap.HalfPageDown.SetEnabled(false)
			m.sourcesView = viewport.New(m.x/2-2, m.y-3)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		// Switch between logs and sources
		case "tab", "shift+tab":
			if m.currentWindow == 1 {
				m.currentWindow = 0
			} else {
				m.currentWindow = 1
			}

		// Delete log from records
		case "d", "delete", "backspace":
			if len(m.logs) > 1 && m.currentWindow == 0 {
				m.logs = slices.Delete(m.logs, m.logTable.Cursor(), m.logTable.Cursor()+1)
				if m.logTable.Cursor() >= len(m.logs) && len(m.logs) > 0 {
					m.logTable.SetCursor(len(m.logs) - 1)
				} else if len(m.logs) == 0 {
					return nil, tea.Quit
				}
				m.logTable.SetRows(slices.Delete(m.logTable.Rows(), m.logTable.Cursor(), m.logTable.Cursor()+1))
				// os.OpenFile(m.logs[m.logTable.Cursor()].Sources[0].Path)
				m.sourcesView.SetContent("DLETE")

			}

		// Open log info or source in user's editor
		case "o", "enter":
			// Open Log
			if m.currentWindow == 0 {

			}
			// Open Source code
			if m.currentWindow == 1 {
				editor := os.Getenv("EDITOR")
				if editor == "" {
					editor = "code"
				}

				var cmd *exec.Cmd

				if editor == "vim" || editor == "nvim" {
					cmd = exec.Command(editor, m.logs[m.logTable.Cursor()].Sources[0].Path, "+", fmt.Sprintf("%d", m.logs[m.logTable.Cursor()].Sources[0].Line))
					cmd.Stdin = os.Stdin
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
				} else {
					cmd = exec.Command(editor, m.logs[m.logTable.Cursor()].Sources[0].Path, "-g", fmt.Sprintf("%d", m.logs[m.logTable.Cursor()].Sources[0].Line))
				}

				if err := cmd.Run(); err != nil {
					bus.LogChannel <- fmt.Sprintf("Error opening source code: %v", err)
				} else {
					bus.LogChannel <- "Source code opened successfully"
				}
			}

		// Quit
		case "ctrl+c", "q":
			m.quit = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.x, m.y = msg.Width, msg.Height
	}

	return m, cmd
}

func createLogTable(logs []log.Log) table.Model {
	// Convert to table rows
	var rows []table.Row
	for _, log := range logs {
		row := table.Row{
			fmt.Sprintf("%v", log.Time),
			log.Message,
		}
		rows = append(rows, row)

	}
	columns := []table.Column{
		{Title: "Timestamp", Width: 20},
		{Title: "Log", Width: 20},
	}

	// Create table
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithWidth(30),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return t
}

func (m Model) View() string {
	if m.progress < 100 {
		return renderPrettySpinner(m)
	}
	var s string
	header := keywordStyle.Render("VLSA - Visual Log Source Analyzer")

	if m.currentWindow == 0 {
		s += lipgloss.JoinVertical(lipgloss.Top, header, lipgloss.JoinHorizontal(lipgloss.Top, focusedWindowStyle.Render(renderLogs(m)), modelStyle.Render(renderSources(m))))
	} else {
		s += lipgloss.JoinVertical(lipgloss.Top, header, lipgloss.JoinHorizontal(lipgloss.Top, modelStyle.Render(renderLogs(m)), focusedWindowStyle.Render(renderSources(m))))
	}

	return s
}

func renderPrettySpinner(m Model) string {
	s := ""
	s += mainStyle.Render("VLSA - Visual Log Source Analyzer\n")
	if m.y > 25 && m.x > 50 && m.x < 230 {
		s += VLSA + "\n"
	}
	if m.y > 25 && m.x >= 230 {
		s += VLSA2 + "\n"
	}

	s += "Loading logs..." + strconv.Itoa(m.progress) + "%\n"
	return s
}

func renderLogs(m Model) string {
	m.logTable.SetWidth((m.x / 2) - 2)
	m.logTable.SetHeight(m.y - 4)

	lw := m.x / 2
	columns := []table.Column{
		{Title: "Timestamp", Width: 20},
		{Title: "Log", Width: lw - 20},
	}
	m.logTable.SetColumns(columns)
	return m.logTable.View() + "\n"
}

func renderSources(m Model) string {
	m.sourcesView.Width = (m.x / 2) - 2
	m.sourcesView.Height = m.y - 4
	m.sourcesView.SetContent(setSourceCodeView(m.logs[m.logTable.Cursor()].Sources[0].SourceCode, m.logs[m.logTable.Cursor()].Sources[0].Line, m.y-4))
	return m.sourcesView.View()
}

func setSourceCodeView(sourceCode string, line, height int) string {
	s := "No source code available"
	if len(sourceCode) == 0 {
		return "No source code available"
	}

	lines := strings.Split(sourceCode, "\n")

	half := height / 2
	if len(lines) < height {
		s = strings.Join(lines[:line-1], "\n")
		s += "\n" + lipgloss.NewStyle().Background(lipgloss.Color("57")).Foreground(lipgloss.Color("229")).Render(lines[line-1]) + "\n"
		s += strings.Join(lines[line:], "\n")
		return s
	} else {
		start := max(line-half, 0)
		end := min(line+half, len(lines))
		s = strings.Join(lines[start:line-1], "\n")
		s += "\n" + lipgloss.NewStyle().Background(lipgloss.Color("57")).Foreground(lipgloss.Color("229")).Render(lines[line-1]) + "\n"
		s += strings.Join(lines[line:end], "\n")
		return s
	}
}
