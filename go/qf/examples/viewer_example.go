// Package main provides an example of using the ViewerModel component
// from the qf interactive log filter composer.
//
// This example demonstrates:
// - Creating and configuring a ViewerModel
// - Loading content from a FileTab
// - Handling basic navigation and message passing
// - Integrating with the filtering system
//
// To run this example:
//
//	go run examples/viewer_example.go
package main

import (
	"fmt"
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
	"github.com/sglavoie/dev-helpers/go/qf/internal/file"
	"github.com/sglavoie/dev-helpers/go/qf/internal/ui"
)

// ExampleApp demonstrates using the ViewerModel in a minimal Bubble Tea application
type ExampleApp struct {
	viewer    *ui.ViewerModel
	tab       *file.FileTab
	filterEng core.FilterEngine
	focused   string
	width     int
	height    int
}

func main() {
	// Initialize the example application
	app := &ExampleApp{
		viewer:    ui.NewViewerModel(),
		filterEng: core.NewFilterEngine(),
		focused:   "viewer",
		width:     80,
		height:    24,
	}

	// Create sample content for demonstration
	app.setupSampleContent()

	// Run the Bubble Tea program
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

func (a *ExampleApp) Init() tea.Cmd {
	return nil
}

func (a *ExampleApp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "f":
			// Demonstrate adding a filter
			cmd := a.addSampleFilter()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case "s":
			// Demonstrate search functionality
			cmd := a.performSampleSearch()
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case "r":
			// Reset view to top
			a.viewer.LoadFileTab(a.tab)
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		resizeMsg := ui.ResizeMsg{Width: msg.Width, Height: msg.Height}

		updatedViewer, cmd := a.viewer.Update(resizeMsg)
		a.viewer = updatedViewer.(*ui.ViewerModel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}

	case ui.StatusUpdateMsg:
		// Handle status updates (in a real app, this would go to status bar)
		fmt.Printf("Status: %s\n", msg.Message)

	default:
		// Pass other messages to the viewer
		updatedViewer, cmd := a.viewer.Update(msg)
		a.viewer = updatedViewer.(*ui.ViewerModel)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	return a, tea.Batch(cmds...)
}

func (a *ExampleApp) View() string {
	// Simple layout with viewer and help text
	help := "Example ViewerModel Demo\n" +
		"Keys: q=quit, f=add filter, s=search, r=reset, j/k=navigate\n" +
		"Use vim-style navigation in the viewer below:\n\n"

	viewerContent := a.viewer.View()

	return help + viewerContent
}

func (a *ExampleApp) setupSampleContent() {
	// Create a file tab with sample log content
	a.tab = file.NewFileTab("/tmp/sample.log")

	// Add sample log lines
	sampleLines := []string{
		"2023-09-14 10:00:00 [INFO] Application started",
		"2023-09-14 10:00:01 [DEBUG] Loading configuration file",
		"2023-09-14 10:00:02 [INFO] Database connection established",
		"2023-09-14 10:00:03 [WARN] Cache warming up, performance may be slow",
		"2023-09-14 10:00:04 [DEBUG] Processing user request",
		"2023-09-14 10:00:05 [ERROR] Failed to connect to external API",
		"2023-09-14 10:00:06 [INFO] Retrying API connection",
		"2023-09-14 10:00:07 [DEBUG] API connection successful",
		"2023-09-14 10:00:08 [INFO] Request processed successfully",
		"2023-09-14 10:00:09 [DEBUG] Cleaning up temporary files",
		"2023-09-14 10:00:10 [WARN] High memory usage detected",
		"2023-09-14 10:00:11 [INFO] Memory cleanup initiated",
		"2023-09-14 10:00:12 [DEBUG] Memory usage normalized",
		"2023-09-14 10:00:13 [INFO] Application healthy",
		"2023-09-14 10:00:14 [ERROR] Unexpected error in user handler",
		"2023-09-14 10:00:15 [INFO] Error logged and reported",
		"2023-09-14 10:00:16 [DEBUG] Continuing normal operation",
		"2023-09-14 10:00:17 [INFO] Daily statistics generated",
		"2023-09-14 10:00:18 [DEBUG] Statistics saved to database",
		"2023-09-14 10:00:19 [INFO] System status: all services operational",
	}

	for i, content := range sampleLines {
		line := file.Line{
			Number:      i + 1,
			Content:     content,
			Offset:      int64(i * 50), // Approximate offset
			Highlighted: false,
		}
		a.tab.Content = append(a.tab.Content, line)
	}

	a.tab.IsLoaded = true
	a.tab.UpdateLastAccessed()

	// Load the tab into the viewer
	a.viewer.LoadFileTab(a.tab)
	a.viewer.SetFocused(true)
}

func (a *ExampleApp) addSampleFilter() tea.Cmd {
	// Create a sample filter pattern to show only ERROR and WARN lines
	pattern := core.FilterPattern{
		ID:         "error-warn-filter",
		Expression: "\\[(ERROR|WARN)\\]",
		Type:       core.FilterInclude,
		MatchCount: 0,
		Color:      "red",
		Created:    time.Now(),
		IsValid:    true,
	}

	// Add pattern to filter engine
	err := a.filterEng.AddPattern(pattern)
	if err != nil {
		return func() tea.Msg {
			return ui.NewErrorMsg(
				fmt.Sprintf("Failed to add filter: %v", err),
				"filter_creation",
				"example_app",
				true,
			)
		}
	}

	// Apply filters to content
	lines := make([]string, 0, len(a.tab.Content))
	for _, line := range a.tab.Content {
		lines = append(lines, line.Content)
	}

	result, err := a.filterEng.ApplyFilters(tea.Context(nil), lines)
	if err != nil {
		return func() tea.Msg {
			return ui.NewErrorMsg(
				fmt.Sprintf("Failed to apply filters: %v", err),
				"filter_application",
				"example_app",
				true,
			)
		}
	}

	// Send content update to viewer
	return func() tea.Msg {
		return ui.NewContentUpdateMsg(a.tab.ID, result)
	}
}

func (a *ExampleApp) performSampleSearch() tea.Cmd {
	// Demonstrate search functionality
	return func() tea.Msg {
		return ui.SearchMsg{
			Pattern:       "connection",
			CaseSensitive: false,
			Direction:     ui.SearchForward,
			TabID:         a.tab.ID,
		}
	}
}

// Demonstrate creating a filter set
func createSampleFilterSet() ui.FilterSet {
	return ui.FilterSet{
		Name: "error-analysis",
		Include: []core.FilterPattern{
			{
				ID:         "errors",
				Expression: "ERROR",
				Type:       core.FilterInclude,
				Color:      "red",
				Created:    time.Now(),
				IsValid:    true,
			},
		},
		Exclude: []core.FilterPattern{
			{
				ID:         "debug",
				Expression: "DEBUG",
				Type:       core.FilterExclude,
				Color:      "gray",
				Created:    time.Now(),
				IsValid:    true,
			},
		},
	}
}

// Example of handling focus changes
func (a *ExampleApp) handleFocusChange(component string) tea.Cmd {
	a.focused = component
	return func() tea.Msg {
		return ui.FocusMsg{
			Component: component,
			PrevFocus: a.focused,
			Reason:    "user_interaction",
		}
	}
}

// Example of creating viewport updates
func (a *ExampleApp) createViewportUpdate(line int) tea.Cmd {
	return func() tea.Msg {
		return ui.ViewportUpdateMsg{
			TabID:          a.tab.ID,
			ScrollPosition: line - 1,
			CursorLine:     line,
			ViewportHeight: a.height - 4, // Account for help text
			Source:         "example_app",
		}
	}
}
