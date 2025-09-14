// Package export example demonstrates the usage of the export functionality.
// This file provides examples of how to use the Exporter for various export scenarios.
package export

import (
	"context"
	"fmt"
	"time"

	"github.com/sglavoie/dev-helpers/go/qf/internal/config"
	"github.com/sglavoie/dev-helpers/go/qf/internal/session"
)

// ExampleUsage demonstrates basic export functionality
func ExampleUsage() {
	// Create configuration and export options
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()

	// Create an exporter
	exporter := NewExporter(cfg, options)

	// Sample log lines to export
	lines := []string{
		"[INFO] Application started successfully",
		"[DEBUG] Loading configuration file",
		"[ERROR] Failed to connect to database",
		"[INFO] Retrying connection in 5 seconds",
		"[INFO] Database connection established",
	}
	lineNumbers := []int{0, 1, 2, 3, 4}

	ctx := context.Background()

	// Example 1: Export to text format with line numbers
	fmt.Println("=== Text Export Example ===")
	result, err := exporter.ExportToText(ctx, lines, lineNumbers)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Success: %v\n", result.Success)
	fmt.Printf("Lines exported: %d\n", result.LinesExported)
	fmt.Printf("Bytes exported: %d\n", result.BytesExported)
	fmt.Printf("Export duration: %v\n", result.ExportDuration)
	fmt.Printf("Content:\n%s\n", result.Content)

	// Example 2: Export to JSON format
	fmt.Println("=== JSON Export Example ===")
	jsonOptions := options
	jsonOptions.Format = FormatJSON
	jsonExporter := NewExporter(cfg, jsonOptions)

	jsonResult, err := jsonExporter.exportToJSON(ctx, lines, lineNumbers)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("JSON export successful, %d bytes\n", jsonResult.BytesExported)

	// Example 3: Generate ripgrep command
	fmt.Println("=== Ripgrep Command Example ===")
	filterSet := session.FilterSet{
		Name: "example-filters",
		Include: []session.FilterPattern{
			{
				ID:         "include-1",
				Expression: "ERROR|WARN",
				Type:       session.FilterInclude,
				IsValid:    true,
			},
			{
				ID:         "include-2",
				Expression: "database",
				Type:       session.FilterInclude,
				IsValid:    true,
			},
		},
		Exclude: []session.FilterPattern{
			{
				ID:         "exclude-1",
				Expression: "DEBUG",
				Type:       session.FilterExclude,
				IsValid:    true,
			},
		},
	}

	targetFiles := []string{"application.log", "error.log"}
	rgResult, err := exporter.GenerateRipgrepCommand(filterSet, targetFiles)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Ripgrep command: %s\n", rgResult.RipgrepCommand)

	// Example 4: Export to file with timestamp
	fmt.Println("=== File Export Example ===")
	fileOptions := options
	fileOptions.UseTimestampSuffix = true
	fileOptions.OutputDirectory = "/tmp"
	fileExporter := NewExporter(cfg, fileOptions)

	fileResult, err := fileExporter.ExportToFile(ctx, lines, lineNumbers, "qf-export")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("File export successful: %s\n", fileResult.OutputPath)
	fmt.Printf("Lines exported: %d\n", fileResult.LinesExported)

	// Example 5: Export with progress tracking
	fmt.Println("=== Progress Tracking Example ===")
	progressExporter := NewExporter(cfg, options)
	progressExporter.SetProgressCallback(func(progress ExportProgress) {
		fmt.Printf("Progress: %d/%d lines processed (%.1f%%)\n",
			progress.ProcessedLines, progress.TotalLines,
			float64(progress.ProcessedLines)/float64(progress.TotalLines)*100)
	})

	// Simulate larger dataset for progress tracking
	largeLines := make([]string, 10000)
	largeLineNumbers := make([]int, 10000)
	for i := range largeLines {
		largeLines[i] = fmt.Sprintf("[INFO] Log entry %d with timestamp %s", i, time.Now().Format("15:04:05.000"))
		largeLineNumbers[i] = i
	}

	progressResult, err := progressExporter.ExportToText(ctx, largeLines, largeLineNumbers)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Large export completed: %d lines in %v\n",
		progressResult.LinesExported, progressResult.ExportDuration)
}

// ExampleClipboardExport demonstrates clipboard functionality
func ExampleClipboardExport() {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()
	exporter := NewExporter(cfg, options)

	lines := []string{
		"Important log entry 1",
		"Critical error message",
		"Recovery successful",
	}
	lineNumbers := []int{100, 101, 102}

	ctx := context.Background()
	result, err := exporter.ExportToClipboard(ctx, lines, lineNumbers)
	if err != nil {
		fmt.Printf("Clipboard export failed: %v\n", err)
		return
	}

	fmt.Printf("Successfully copied %d lines to clipboard\n", result.LinesExported)
}

// ExampleMultipleFormats demonstrates exporting the same data in different formats
func ExampleMultipleFormats() {
	cfg := config.NewDefaultConfig()
	lines := []string{
		"[2023-09-14 10:30:15] INFO: Server started on port 8080",
		"[2023-09-14 10:30:16] DEBUG: Loading configuration from /etc/app.conf",
		"[2023-09-14 10:30:17] ERROR: Database connection failed",
	}
	lineNumbers := []int{1, 2, 3}

	ctx := context.Background()

	// Export formats to demonstrate
	formats := []ExportFormat{FormatText, FormatJSON, FormatCSV}

	for _, format := range formats {
		options := DefaultExportOptions()
		options.Format = format
		options.IncludeHeaders = true

		exporter := NewExporter(cfg, options)

		switch format {
		case FormatText:
			result, err := exporter.ExportToText(ctx, lines, lineNumbers)
			if err != nil {
				fmt.Printf("Text export error: %v\n", err)
				continue
			}
			fmt.Printf("=== TEXT FORMAT ===\n%s\n", result.Content)

		case FormatJSON:
			result, err := exporter.exportToJSON(ctx, lines, lineNumbers)
			if err != nil {
				fmt.Printf("JSON export error: %v\n", err)
				continue
			}
			fmt.Printf("=== JSON FORMAT ===\n%s\n", result.Content)

		case FormatCSV:
			result, err := exporter.exportToCSV(ctx, lines, lineNumbers)
			if err != nil {
				fmt.Printf("CSV export error: %v\n", err)
				continue
			}
			fmt.Printf("=== CSV FORMAT ===\n%s\n", result.Content)
		}
	}
}

// ExampleAdvancedOptions demonstrates advanced export options
func ExampleAdvancedOptions() {
	cfg := config.NewDefaultConfig()

	// Configure advanced export options
	options := ExportOptions{
		Format:             FormatText,
		IncludeLineNumbers: true,
		LineNumberFormat:   "%8d | ", // Custom line number format
		IncludeTimestamp:   true,
		TimestampFormat:    "2006-01-02T15:04:05.000Z", // ISO 8601 format
		IncludeHeaders:     true,
		CustomDelimiter:    ";", // For CSV exports
		UseTimestampSuffix: true,
		FilenamePattern:    "advanced-export-{timestamp}",
	}

	exporter := NewExporter(cfg, options)

	lines := []string{
		"Application initialization complete",
		"User authentication successful",
		"Data processing started",
		"Batch job completed successfully",
	}
	lineNumbers := []int{10, 25, 50, 100}

	ctx := context.Background()
	result, err := exporter.ExportToText(ctx, lines, lineNumbers)
	if err != nil {
		fmt.Printf("Advanced export error: %v\n", err)
		return
	}

	fmt.Printf("=== ADVANCED OPTIONS EXPORT ===\n%s\n", result.Content)
	fmt.Printf("Export completed in %v\n", result.ExportDuration)
}
