package main

import (
	"fmt"
	"os"

	"vlsa/internal/bus"
	"vlsa/internal/log"
	"vlsa/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	model := tui.Model{}

	p := tea.NewProgram(model)

	go func() {
		logChannel := make(chan log.LogProcessingMsg)
		go func() {
			for msg := range logChannel {
				p.Send(msg)
			}
		}()
		log.ProcessLogs(os.Args[1], logChannel)
	}()

	appLogs := make(chan string)
	go func() {
		f, err := os.OpenFile("log.csv", os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		for log := range appLogs {
			_, err := f.WriteString(fmt.Sprintf("%s\n", log))
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error writing to log file: %v\n", err)
				os.Exit(1)
			}
		}

	}()

	bus.LogChannel = appLogs
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}

}
