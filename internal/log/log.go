package log

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"vlsa/internal/bus"
)

type Log struct {
	Time              time.Time
	Service           string
	Message           string
	Sources           []SourceMapping
	SelectedSourceIdx int // Track which source index is currently selected
}

type SourceMapping struct {
	Path           string
	Line           int
	DisplayMessage string
	SourceCode     string
}

var searchCache = map[string][]SourceMapping{}

// Processes logs at the provided file path.
// Gives progress updates and sends the logs to the provided channel.
func ProcessLogs(fp string, uChan chan LogProcessingMsg) {
	// If a log file is provided, open it and read the logs
	file, err := os.Open(fp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// If the file is a csv, parse it out with the csv package else, just plain text
	logs := []Log{}
	if fp[len(fp)-4:] == ".csv" {
		logs = parseCSVLogs(file)
	} else {
		logs = parsePlainTextLogs(file)
	}

	// Map sources to logs
	for i := range logs {
		// TODO: mapping of sources to logs
		sourceMapLog(&logs[i])

		uChan <- LogProcessingMsg{
			Progress: (i) * 100 / len(logs),
			Logs:     []Log{},
		}
	}

	uChan <- LogProcessingMsg{
		Progress: 100,
		Logs:     logs,
	}
	close(uChan)
	return
}

func parseCSVLogs(file *os.File) []Log {
	// Parse CSV file
	logs := []Log{}
	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading CSV file: %v\n", err)
		os.Exit(1)
	}

	for _, record := range records {
		if len(record) < 2 {
			continue // Skip malformed lines
		}

		// Time is provided as 2025-06-19T03:40:54.794Z
		time, err := time.Parse("2006-01-02T15:04:05.000Z", record[0])
		if err != nil {
			continue
		}
		l := Log{
			Time:    time,
			Service: record[2],
			Message: record[3],
			Sources: []SourceMapping{}, // Sources are added later
		}

		logs = append(logs, l)
	}

	return logs
}

func parsePlainTextLogs(file *os.File) []Log {
	logs := []Log{}
	// TODO: Implement parsing for plain text logs
	panic("Plain text log parsing not implemented yet")

	return logs
}

// Maps source files to logs based on the log message.
func sourceMapLog(l *Log) {
	sm := l.Message
	// JSON or any part of the message after a colon in a
	// log message is likely to be highly dynamic and not
	// correspond with source code so we will parse it out
	sm = parseOutDynamics(sm, true)

	if sm == "" {
		l.Sources = []SourceMapping{{Path: "", Line: 0, DisplayMessage: "This log message was found to be highly dynamic.\nNo source mapping found for this log message..."}}
		return
	} else {
		// Check if we have already searched for this message
		if sources, found := searchCache[sm]; found {
			bus.LogChannel <- fmt.Sprintf("Using cached source mapping for log message: %s", sm)
			l.Sources = sources
			return
		}

		sources := rg(sm)
		// Cache the sources for this message
		searchCache[sm] = sources
		if len(sources) == 0 {
			sm := parseOutDynamics(l.Message, false)
			sources = rg(sm)
			if len(sources) == 0 {
				l.Sources = []SourceMapping{{Path: "", Line: 0, DisplayMessage: "No source mapping found for this log message..."}}
				return
			} else {
				l.Sources = sources
				return
			}
		}
		if len(sources) > 4 {
			// If we have too many sources, we likely didn't include enough text in the search
			// so we will try again with the full message, if that fails
			// (ie. has 0 or greater than equal to results) we will try without the
			// colon dynamics of a message. We should cache whichever result we take
			if fullMsg, found := searchCache[l.Message]; found {
				l.Sources = fullMsg
				return
			}

			sm := parseOutDynamics(l.Message, false)
			if colonSources, found := searchCache[sm]; found {
				l.Sources = colonSources
				return
			}

			fullMsgSources := rg(l.Message)
			if len(fullMsgSources) < len(sources) && len(fullMsgSources) > 0 {
				l.Sources = fullMsgSources
				searchCache[l.Message] = fullMsgSources
				return
			} else if withColonSources := rg(sm); len(withColonSources) < len(sources) && len(withColonSources) < len(fullMsgSources) && len(withColonSources) > 0 {
				l.Sources = withColonSources
				searchCache[sm] = withColonSources
				return
			}

			l.Sources = sources
			return
		} else {
			bus.LogChannel <- fmt.Sprintf("Found %d source mappings for log message: %s", len(sources), sm)
			l.Sources = sources
		}
	}
}

// Applies a filter for dynamics in log messages before they are source mapped.
func parseOutDynamics(message string, colon bool) string {
	// If the message is just a JSON object, we should skip it
	// This is a simple heuristic, but it should work for most cases
	m := ""
	openBraceCount := 0
	for _, c := range message {
		if c == '{' {
			openBraceCount++
			continue
		}
		if c == '}' {
			openBraceCount--
			continue
		}
		if openBraceCount == 0 {
			m = m + string(c)
		}
	}

	// Trim the message whitespace and to any colon so we don't search for formatted strings either
	m = strings.TrimSpace(m)
	if colon {
		m = strings.TrimSpace(strings.Split(m, ":")[0])
	}
	return m
}

func rg(sm string) []SourceMapping {
	cmd := exec.Command("rg", "--line-number", "-F", sm, "--glob", "!**/*.csv")

	out, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			// This is expected if no matches are found, so we can ignore it
			bus.LogChannel <- fmt.Sprintf("no matches found!")
			return []SourceMapping{{Path: "", Line: 0, DisplayMessage: "No source mapping found for this log message..."}}
		} else {
			fmt.Fprintf(os.Stderr, "Error running command: %v\n%s\n\n\n%s", err, string(out), "Do you have ripgrep installed? It is required for source mapping.")
			os.Exit(1)
		}
	}

	return parseRGOutput(string(out))
}

func parseRGOutput(lines string) []SourceMapping {
	sources := []SourceMapping{}

	scanner := bufio.NewScanner(strings.NewReader(lines))
	for scanner.Scan() {
		line := scanner.Text()
		segments := strings.SplitN(line, ":", 3)
		if len(segments) < 3 {
			continue // Skip malformed lines
		}
		lineNum, err := strconv.Atoi(segments[1])
		if err != nil {
			continue // Skip malformed lines
		}

		f, err := os.OpenFile(segments[0], os.O_RDONLY, 0644)
		if err != nil {
			bus.LogChannel <- fmt.Sprintf("Error opening source file %s: %v\n", segments[0], err)
			continue // Skip files that cannot be opened
		}

		defer f.Close()
		// Read the all source code from the file
		source := ""
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			source += scanner.Text() + "\n"
		}

		sources = append(sources, SourceMapping{
			Path:           segments[0],
			Line:           lineNum,
			DisplayMessage: "File found!",
			SourceCode:     source,
		})

	}
	return sources
}

type LogProcessingMsg struct {
	Progress int
	Logs     []Log
}
