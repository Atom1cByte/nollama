package main

import (
	"fmt"
	"os"

	"nollama/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	baseURL := "http://localhost:11434"
	if len(os.Args) > 1 {
		baseURL = os.Args[1]
	}

	m := tui.NewModel(baseURL)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Store the program in the model so we can send messages from goroutines
	m.SetProgram(p)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
