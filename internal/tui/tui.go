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

	"github.com/charmbracelet/bubbles/list"
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

// SourceItem represents an item in the source selector list
type SourceItem struct {
	path string
	line int
	idx  int
}

func (s SourceItem) FilterValue() string { return s.path }
func (s SourceItem) Title() string       { return fmt.Sprintf("%s:%d", s.path, s.line) }
func (s SourceItem) Description() string { return "Source file location" }

// Model of the application state
type Model struct {
	// Application state
	logs          []log.Log
	currentLogIdx int

	// UI specific fields
	x                  int
	y                  int
	logTable           table.Model
	sourcesView        viewport.Model
	sourceSelector     list.Model
	selectedSourceIdx  int              // Track which source is selected for current log
	showSourceSelector bool             // Whether to show the selector pane
	currentWindow      int              // 0=logs, 1=sources, 2=selector
	progress           int
	quit               bool
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	// Update the appropriate component based on current window
	switch m.currentWindow {
	case 0: // Logs table
		m.logTable, cmd = m.logTable.Update(msg)
		// Check if we need to update source selector when log changes
		if len(m.logs) > 0 && m.logTable.Cursor() < len(m.logs) {
			m.updateSourceSelector()
		}
	case 1: // Sources view
		m.sourcesView, cmd = m.sourcesView.Update(msg)
	case 2: // Source selector
		m.sourceSelector, cmd = m.sourceSelector.Update(msg)
	}

	switch msg := msg.(type) {
	case log.LogProcessingMsg:
		m.progress = msg.Progress
		if m.progress >= 100 {
			m.logs = msg.Logs
			m.logTable = createLogTable(m.logs)
			m.logTable.KeyMap.HalfPageDown.SetEnabled(false)
			m.sourcesView = viewport.New(m.getSourcesViewWidth(), m.y-3)
			m.updateSourceSelector()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		// Switch between panes
		case "tab":
			m.nextWindow()
		case "shift+tab":
			m.prevWindow()

		// Source selector specific keys
		case "enter":
			if m.currentWindow == 2 && m.showSourceSelector {
				// Select source and return to source view
				if selectedItem, ok := m.sourceSelector.SelectedItem().(SourceItem); ok {
					// Update the current log's selected source index
					if m.logTable.Cursor() < len(m.logs) {
						m.logs[m.logTable.Cursor()].SelectedSourceIdx = selectedItem.idx
					}
					m.showSourceSelector = false
					m.currentWindow = 1 // Switch back to source view
				}
			} else if m.currentWindow == 1 {
				// Open source in editor
				m.openCurrentSourceInEditor()
			}

		// Show source selector if multiple sources available
		case "s":
			if m.currentWindow == 1 && len(m.logs) > 0 && len(m.logs[m.logTable.Cursor()].Sources) > 1 {
				m.showSourceSelector = true
				m.currentWindow = 2
			}

		// Apply source to similar logs
		case "a":
			if m.currentWindow == 2 && m.showSourceSelector {
				m.applySourceToSimilarLogs()
				m.showSourceSelector = false
				m.currentWindow = 1
			}

		// Escape from source selector
		case "esc":
			if m.currentWindow == 2 {
				m.showSourceSelector = false
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
				m.updateSourceSelector()
			}

		// Quit
		case "ctrl+c", "q":
			m.quit = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.x, m.y = msg.Width, msg.Height
		m.sourcesView.Width = m.getSourcesViewWidth()
		m.sourcesView.Height = m.y - 3
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
	
	header := keywordStyle.Render("VLSA - Visual Log Source Analyzer")
	
	// Render based on whether source selector is shown
	if m.showSourceSelector {
		// Three-pane layout
		var logsPane, sourcesPane, selectorPane string
		
		if m.currentWindow == 0 {
			logsPane = focusedWindowStyle.Render(renderLogs(m))
		} else {
			logsPane = modelStyle.Render(renderLogs(m))
		}
		
		if m.currentWindow == 1 {
			sourcesPane = focusedWindowStyle.Render(renderSources(m))
		} else {
			sourcesPane = modelStyle.Render(renderSources(m))
		}
		
		if m.currentWindow == 2 {
			selectorPane = focusedWindowStyle.Render(renderSourceSelector(m))
		} else {
			selectorPane = modelStyle.Render(renderSourceSelector(m))
		}
		
		return lipgloss.JoinVertical(lipgloss.Top, 
			header, 
			lipgloss.JoinHorizontal(lipgloss.Top, logsPane, sourcesPane, selectorPane))
	} else {
		// Two-pane layout (original)
		var logsPane, sourcesPane string
		
		if m.currentWindow == 0 {
			logsPane = focusedWindowStyle.Render(renderLogs(m))
			sourcesPane = modelStyle.Render(renderSources(m))
		} else {
			logsPane = modelStyle.Render(renderLogs(m))
			sourcesPane = focusedWindowStyle.Render(renderSources(m))
		}
		
		return lipgloss.JoinVertical(lipgloss.Top, 
			header, 
			lipgloss.JoinHorizontal(lipgloss.Top, logsPane, sourcesPane))
	}
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
	width := (m.x / 2) - 2
	if m.showSourceSelector {
		width = (m.x / 3) - 2
	}
	
	m.logTable.SetWidth(width)
	m.logTable.SetHeight(m.y - 4)

	columns := []table.Column{
		{Title: "Timestamp", Width: 20},
		{Title: "Log", Width: width - 22},
	}
	m.logTable.SetColumns(columns)
	return m.logTable.View() + "\n"
}

func renderSources(m Model) string {
	width := m.getSourcesViewWidth()
	m.sourcesView.Width = width
	m.sourcesView.Height = m.y - 4
	
	if len(m.logs) == 0 || m.logTable.Cursor() >= len(m.logs) {
		m.sourcesView.SetContent("No logs available")
		return m.sourcesView.View()
	}
	
	currentLog := m.logs[m.logTable.Cursor()]
	if len(currentLog.Sources) == 0 {
		m.sourcesView.SetContent("No source code available")
		return m.sourcesView.View()
	}
	
	// Use the log's selected source index, default to 0
	sourceIdx := currentLog.SelectedSourceIdx
	if sourceIdx >= len(currentLog.Sources) {
		sourceIdx = 0
	}
	
	source := currentLog.Sources[sourceIdx]
	content := setSourceCodeView(source.SourceCode, source.Line, m.y-4)
	
	// Add header showing current source
	header := fmt.Sprintf("Source: %s:%d", source.Path, source.Line)
	if len(currentLog.Sources) > 1 {
		header += fmt.Sprintf(" (%d of %d sources - press 's' to select)", sourceIdx+1, len(currentLog.Sources))
	}
	content = subtleStyle.Render(header) + "\n" + content
	
	m.sourcesView.SetContent(content)
	return m.sourcesView.View()
}

func renderSourceSelector(m Model) string {
	if !m.showSourceSelector {
		return ""
	}
	
	width := (m.x / 3) - 2
	height := m.y - 4
	
	m.sourceSelector.SetWidth(width)
	m.sourceSelector.SetHeight(height)
	
	// Add instructions at the bottom
	instructions := subtleStyle.Render("↑↓: Navigate • Enter: Select • A: Apply to similar • Esc: Cancel")
	
	return m.sourceSelector.View() + "\n" + instructions
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

// Helper methods for Model

func (m *Model) updateSourceSelector() {
	if len(m.logs) == 0 || m.logTable.Cursor() >= len(m.logs) {
		return
	}
	
	currentLog := m.logs[m.logTable.Cursor()]
	if len(currentLog.Sources) <= 1 {
		m.showSourceSelector = false
		return
	}

	// Create list items for source selector
	var items []list.Item
	for i, source := range currentLog.Sources {
		items = append(items, SourceItem{
			path: source.Path,
			line: source.Line,
			idx:  i,
		})
	}

	m.sourceSelector = list.New(items, list.NewDefaultDelegate(), 0, 0)
	m.sourceSelector.Title = "Multiple Sources Available"
	m.sourceSelector.SetShowStatusBar(false)
	m.sourceSelector.SetFilteringEnabled(false)
}

func (m *Model) getSourcesViewWidth() int {
	if m.showSourceSelector {
		return (m.x / 3) - 2 // Three pane layout
	}
	return (m.x / 2) - 2 // Two pane layout
}

func (m *Model) nextWindow() {
	maxWindow := 1
	if m.showSourceSelector {
		maxWindow = 2
	}
	
	m.currentWindow = (m.currentWindow + 1) % (maxWindow + 1)
}

func (m *Model) prevWindow() {
	maxWindow := 1
	if m.showSourceSelector {
		maxWindow = 2
	}
	
	m.currentWindow = (m.currentWindow - 1 + maxWindow + 1) % (maxWindow + 1)
}

func (m *Model) openCurrentSourceInEditor() {
	if len(m.logs) == 0 || m.logTable.Cursor() >= len(m.logs) {
		return
	}
	
	currentLog := m.logs[m.logTable.Cursor()]
	if len(currentLog.Sources) == 0 {
		return
	}
	
	// Use the log's selected source index
	sourceIdx := currentLog.SelectedSourceIdx
	if sourceIdx >= len(currentLog.Sources) {
		sourceIdx = 0
	}
	
	source := currentLog.Sources[sourceIdx]
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "code"
	}

	var cmd *exec.Cmd
	if editor == "vim" || editor == "nvim" {
		cmd = exec.Command(editor, source.Path, "+", fmt.Sprintf("%d", source.Line))
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd = exec.Command(editor, source.Path, "-g", fmt.Sprintf("%d", source.Line))
	}

	if err := cmd.Run(); err != nil {
		bus.LogChannel <- fmt.Sprintf("Error opening source code: %v", err)
	} else {
		bus.LogChannel <- "Source code opened successfully"
	}
}

func (m *Model) applySourceToSimilarLogs() {
	if len(m.logs) == 0 || m.logTable.Cursor() >= len(m.logs) {
		return
	}
	
	currentLog := m.logs[m.logTable.Cursor()]
	currentMessage := currentLog.Message
	selectedSourceIdx := currentLog.SelectedSourceIdx
	
	// Find all logs with the same message and update their selected source
	count := 0
	for i := range m.logs {
		if m.logs[i].Message == currentMessage && len(m.logs[i].Sources) > selectedSourceIdx {
			// Update the selected source index for similar logs
			m.logs[i].SelectedSourceIdx = selectedSourceIdx
			count++
		}
	}
	
	bus.LogChannel <- fmt.Sprintf("Applied source selection to %d similar logs", count)
}
