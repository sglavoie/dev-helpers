// Package main provides the entry point for the qf Interactive Log Filter Composer.
package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/dev-helpers/go/qf/internal/ui"
)

func main() {
	// Parse command line arguments
	var initialFiles []string
	if len(os.Args) > 1 {
		initialFiles = os.Args[1:]
	}

	// Create the main application model
	app := ui.NewAppModel(initialFiles)

	// Setup Bubble Tea program with proper options
	program := tea.NewProgram(
		app,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Handle cleanup on exit
	defer func() {
		if r := recover(); r != nil {
			app.Cleanup()
			fmt.Fprintf(os.Stderr, "Application panicked: %v\n", r)
			os.Exit(1)
		}
	}()

	// Run the program
	if _, err := program.Run(); err != nil {
		app.Cleanup()
		log.Fatalf("Error running qf: %v", err)
	}

	// Cleanup resources
	app.Cleanup()
}
