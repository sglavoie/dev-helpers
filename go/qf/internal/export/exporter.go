// Package export provides comprehensive export functionality for the qf Interactive Log Filter Composer.
//
// This package supports multiple export formats including plain text with line numbers,
// ripgrep command generation from filter patterns, clipboard integration, and file export
// with timestamp suffixes. It provides flexible output options for filtered log content
// and search commands while maintaining compatibility with the existing FilterSet and Pattern types.
package export

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/atotto/clipboard"

	"github.com/sglavoie/dev-helpers/go/qf/internal/config"
	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
	"github.com/sglavoie/dev-helpers/go/qf/internal/session"
)

// ExportFormat represents the supported export formats
type ExportFormat int

const (
	// FormatText exports plain text with optional line numbers
	FormatText ExportFormat = iota
	// FormatRipgrepCommand generates executable ripgrep commands
	FormatRipgrepCommand
	// FormatJSON exports structured JSON data
	FormatJSON
	// FormatCSV exports CSV format for data analysis tools
	FormatCSV
)

// String returns a string representation of the ExportFormat
func (f ExportFormat) String() string {
	switch f {
	case FormatText:
		return "text"
	case FormatRipgrepCommand:
		return "ripgrep"
	case FormatJSON:
		return "json"
	case FormatCSV:
		return "csv"
	default:
		return "unknown"
	}
}

// ExportOptions configures export behavior and output formatting
type ExportOptions struct {
	// Format specifies the export format
	Format ExportFormat `json:"format"`

	// IncludeLineNumbers controls whether line numbers are included in text exports
	IncludeLineNumbers bool `json:"include_line_numbers"`

	// LineNumberFormat specifies the format string for line numbers (default: "%6d: ")
	LineNumberFormat string `json:"line_number_format"`

	// IncludeTimestamp adds timestamp prefix to each line
	IncludeTimestamp bool `json:"include_timestamp"`

	// TimestampFormat specifies the timestamp format (default: "2006-01-02 15:04:05")
	TimestampFormat string `json:"timestamp_format"`

	// IncludeHeaders adds metadata headers to exports (source file, export time)
	IncludeHeaders bool `json:"include_headers"`

	// CustomDelimiter specifies delimiter for CSV exports
	CustomDelimiter string `json:"custom_delimiter"`

	// OutputDirectory specifies directory for file exports
	OutputDirectory string `json:"output_directory"`

	// FilenamePattern specifies pattern for output filenames with timestamp suffix
	FilenamePattern string `json:"filename_pattern"`

	// UseTimestampSuffix adds timestamp suffix to output filenames
	UseTimestampSuffix bool `json:"use_timestamp_suffix"`

	// CompressOutput compresses the output for large exports
	CompressOutput bool `json:"compress_output"`
}

// DefaultExportOptions returns ExportOptions with sensible defaults
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		Format:             FormatText,
		IncludeLineNumbers: true,
		LineNumberFormat:   "%6d: ",
		IncludeTimestamp:   false,
		TimestampFormat:    "2006-01-02 15:04:05",
		IncludeHeaders:     true,
		CustomDelimiter:    ",",
		OutputDirectory:    "",
		FilenamePattern:    "qf-export-{timestamp}",
		UseTimestampSuffix: true,
		CompressOutput:     false,
	}
}

// ExportProgress tracks progress for large export operations
type ExportProgress struct {
	// TotalLines is the total number of lines to export
	TotalLines int `json:"total_lines"`

	// ProcessedLines is the number of lines processed so far
	ProcessedLines int `json:"processed_lines"`

	// BytesProcessed is the total bytes processed
	BytesProcessed int64 `json:"bytes_processed"`

	// StartTime is when the export started
	StartTime time.Time `json:"start_time"`

	// ElapsedTime is the time elapsed since export started
	ElapsedTime time.Duration `json:"elapsed_time"`

	// EstimatedTimeRemaining is the estimated time remaining
	EstimatedTimeRemaining time.Duration `json:"estimated_time_remaining"`

	// Status is the current export status
	Status string `json:"status"`
}

// ExportResult contains the results of an export operation
type ExportResult struct {
	// Success indicates if the export completed successfully
	Success bool `json:"success"`

	// OutputPath is the path to the exported file (for file exports)
	OutputPath string `json:"output_path,omitempty"`

	// Content contains the exported content (for in-memory exports)
	Content string `json:"content,omitempty"`

	// LinesExported is the number of lines that were exported
	LinesExported int `json:"lines_exported"`

	// BytesExported is the total bytes exported
	BytesExported int64 `json:"bytes_exported"`

	// ExportDuration is the time taken to complete the export
	ExportDuration time.Duration `json:"export_duration"`

	// RipgrepCommand contains the generated ripgrep command (for ripgrep format)
	RipgrepCommand string `json:"ripgrep_command,omitempty"`

	// Error contains any error that occurred during export
	Error error `json:"error,omitempty"`
}

// ExportData represents the data structure for structured exports (JSON/CSV)
type ExportData struct {
	// Metadata about the export
	Metadata ExportMetadata `json:"metadata"`

	// FilterSet used for this export
	FilterSet session.FilterSet `json:"filter_set"`

	// Lines contain the filtered content
	Lines []ExportLine `json:"lines"`

	// Stats contain export statistics
	Stats ExportStats `json:"stats"`
}

// ExportMetadata contains metadata about the export
type ExportMetadata struct {
	// ExportTime is when the export was created
	ExportTime time.Time `json:"export_time"`

	// SourceFile is the original file path
	SourceFile string `json:"source_file"`

	// ExportFormat is the format used
	ExportFormat string `json:"export_format"`

	// QFVersion is the version of qf used
	QFVersion string `json:"qf_version"`

	// ConfigVersion is the config version used
	ConfigVersion string `json:"config_version"`
}

// ExportLine represents a single line in structured exports
type ExportLine struct {
	// Number is the original line number
	Number int `json:"number"`

	// Content is the line content
	Content string `json:"content"`

	// Timestamp is the timestamp if included
	Timestamp time.Time `json:"timestamp,omitempty"`

	// Highlights contains match highlight information
	Highlights []core.Highlight `json:"highlights,omitempty"`
}

// ExportStats contains statistics about the export
type ExportStats struct {
	// OriginalLineCount is the total lines in the original file
	OriginalLineCount int `json:"original_line_count"`

	// FilteredLineCount is the lines after filtering
	FilteredLineCount int `json:"filtered_line_count"`

	// ProcessingTime is the time taken to process
	ProcessingTime time.Duration `json:"processing_time"`

	// ExportTime is the time taken to export
	ExportTime time.Duration `json:"export_time"`

	// PatternsUsed is the number of patterns used in filtering
	PatternsUsed int `json:"patterns_used"`
}

// ProgressCallback is called during long-running export operations
type ProgressCallback func(progress ExportProgress)

// Exporter provides export functionality with multiple format support
type Exporter struct {
	// config holds the application configuration
	config *config.Config

	// options holds the export options
	options ExportOptions

	// progressCallback is called during export progress updates
	progressCallback ProgressCallback
}

// NewExporter creates a new Exporter instance with the given configuration and options
func NewExporter(cfg *config.Config, options ExportOptions) *Exporter {
	if cfg == nil {
		cfg = config.NewDefaultConfig()
	}

	return &Exporter{
		config:  cfg,
		options: options,
	}
}

// SetProgressCallback sets a callback function for progress updates
func (e *Exporter) SetProgressCallback(callback ProgressCallback) {
	e.progressCallback = callback
}

// ExportToText exports filtered content as plain text with optional line numbers
func (e *Exporter) ExportToText(ctx context.Context, lines []string, lineNumbers []int) (ExportResult, error) {
	start := time.Now()

	result := ExportResult{
		Success: false,
	}

	if len(lines) == 0 {
		result.Content = ""
		result.Success = true
		result.ExportDuration = time.Since(start)
		return result, nil
	}

	var builder strings.Builder

	// Add headers if requested
	if e.options.IncludeHeaders {
		builder.WriteString(fmt.Sprintf("# qf Export - %s\n", time.Now().Format(e.options.TimestampFormat)))
		builder.WriteString(fmt.Sprintf("# Format: %s\n", e.options.Format.String()))
		builder.WriteString(fmt.Sprintf("# Lines: %d\n", len(lines)))
		builder.WriteString("#\n")
	}

	// Process each line
	totalLines := len(lines)
	for i, line := range lines {
		// Check for context cancellation
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			default:
			}

			// Update progress
			if e.progressCallback != nil {
				progress := ExportProgress{
					TotalLines:     totalLines,
					ProcessedLines: i,
					StartTime:      start,
					ElapsedTime:    time.Since(start),
					Status:         "Processing lines",
				}

				if i > 0 {
					rate := float64(i) / time.Since(start).Seconds()
					remaining := float64(totalLines-i) / rate
					progress.EstimatedTimeRemaining = time.Duration(remaining) * time.Second
				}

				e.progressCallback(progress)
			}
		}

		// Add timestamp if requested
		if e.options.IncludeTimestamp {
			builder.WriteString(time.Now().Format(e.options.TimestampFormat))
			builder.WriteString(" ")
		}

		// Add line number if requested
		if e.options.IncludeLineNumbers {
			lineNum := i + 1
			if len(lineNumbers) > i {
				lineNum = lineNumbers[i] + 1 // Convert from 0-based to 1-based
			}
			builder.WriteString(fmt.Sprintf(e.options.LineNumberFormat, lineNum))
		}

		// Add the line content
		builder.WriteString(line)
		builder.WriteString("\n")
	}

	result.Content = builder.String()
	result.Success = true
	result.LinesExported = len(lines)
	result.BytesExported = int64(len(result.Content))
	result.ExportDuration = time.Since(start)

	return result, nil
}

// ExportToClipboard copies exported content to the system clipboard
func (e *Exporter) ExportToClipboard(ctx context.Context, lines []string, lineNumbers []int) (ExportResult, error) {
	// First export to text format
	textResult, err := e.ExportToText(ctx, lines, lineNumbers)
	if err != nil {
		return textResult, fmt.Errorf("failed to generate text for clipboard: %w", err)
	}

	// Copy to clipboard
	err = clipboard.WriteAll(textResult.Content)
	if err != nil {
		textResult.Success = false
		textResult.Error = fmt.Errorf("failed to write to clipboard: %w", err)
		return textResult, err
	}

	return textResult, nil
}

// ExportToFile exports content to a file with timestamp suffix functionality
func (e *Exporter) ExportToFile(ctx context.Context, lines []string, lineNumbers []int, baseFilename string) (ExportResult, error) {
	start := time.Now()

	// Generate filename with timestamp suffix if requested
	filename := e.generateFilename(baseFilename)

	// Ensure output directory exists
	if e.options.OutputDirectory != "" {
		if err := os.MkdirAll(e.options.OutputDirectory, 0755); err != nil {
			return ExportResult{
				Success: false,
				Error:   fmt.Errorf("failed to create output directory: %w", err),
			}, err
		}
		filename = filepath.Join(e.options.OutputDirectory, filename)
	}

	// Export based on format
	var result ExportResult
	var err error

	switch e.options.Format {
	case FormatText:
		result, err = e.ExportToText(ctx, lines, lineNumbers)
	case FormatJSON:
		result, err = e.exportToJSON(ctx, lines, lineNumbers)
	case FormatCSV:
		result, err = e.exportToCSV(ctx, lines, lineNumbers)
	case FormatRipgrepCommand:
		// For ripgrep format, we need the filter set
		return ExportResult{
			Success: false,
			Error:   fmt.Errorf("ripgrep command export requires filter set, use GenerateRipgrepCommand instead"),
		}, fmt.Errorf("invalid format for file export")
	default:
		return ExportResult{
			Success: false,
			Error:   fmt.Errorf("unsupported export format: %v", e.options.Format),
		}, fmt.Errorf("unsupported format")
	}

	if err != nil {
		return result, err
	}

	// Write to temporary file first (atomic write)
	tempFile := filename + ".tmp"
	err = os.WriteFile(tempFile, []byte(result.Content), 0644)
	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to write temporary file: %w", err)
		return result, err
	}

	// Atomic rename
	err = os.Rename(tempFile, filename)
	if err != nil {
		os.Remove(tempFile) // Clean up temp file
		result.Success = false
		result.Error = fmt.Errorf("failed to save file: %w", err)
		return result, err
	}

	result.OutputPath = filename
	result.ExportDuration = time.Since(start)

	return result, nil
}

// GenerateRipgrepCommand generates executable ripgrep commands from filter patterns
func (e *Exporter) GenerateRipgrepCommand(filterSet session.FilterSet, targetFiles []string) (ExportResult, error) {
	start := time.Now()

	var parts []string
	parts = append(parts, "rg")

	// Process include patterns - combine with OR logic
	if len(filterSet.Include) > 0 {
		var includePatterns []string
		for _, pattern := range filterSet.Include {
			if pattern.IsValid && pattern.Expression != "" {
				// Escape special shell characters
				escaped := e.escapeRegexForShell(pattern.Expression)
				includePatterns = append(includePatterns, escaped)
			}
		}

		if len(includePatterns) > 0 {
			if len(includePatterns) == 1 {
				parts = append(parts, fmt.Sprintf("'%s'", includePatterns[0]))
			} else {
				// Combine patterns with OR logic: (pattern1|pattern2|pattern3)
				combined := fmt.Sprintf("'(%s)'", strings.Join(includePatterns, "|"))
				parts = append(parts, combined)
			}
		}
	}

	// Process exclude patterns - use --invert-match
	for _, pattern := range filterSet.Exclude {
		if pattern.IsValid && pattern.Expression != "" {
			escaped := e.escapeRegexForShell(pattern.Expression)
			parts = append(parts, fmt.Sprintf("--invert-match '%s'", escaped))
		}
	}

	// Add common ripgrep options
	parts = append(parts, "--line-number")  // Show line numbers
	parts = append(parts, "--color=always") // Colorize output

	// Add target files
	parts = append(parts, targetFiles...)

	command := strings.Join(parts, " ")

	result := ExportResult{
		Success:        true,
		RipgrepCommand: command,
		Content:        command,
		LinesExported:  1, // One command line
		BytesExported:  int64(len(command)),
		ExportDuration: time.Since(start),
	}

	return result, nil
}

// ExportFilteredContent exports filtered content using the specified format and options
func (e *Exporter) ExportFilteredContent(ctx context.Context, filterResult core.FilterResult, sourceFile string) (ExportResult, error) {
	switch e.options.Format {
	case FormatText:
		return e.ExportToText(ctx, filterResult.MatchedLines, filterResult.LineNumbers)
	case FormatJSON:
		return e.exportFilteredToJSON(ctx, filterResult, sourceFile)
	case FormatCSV:
		return e.exportFilteredToCSV(ctx, filterResult, sourceFile)
	default:
		return ExportResult{
			Success: false,
			Error:   fmt.Errorf("unsupported export format: %v", e.options.Format),
		}, fmt.Errorf("unsupported format")
	}
}

// generateFilename creates a filename with optional timestamp suffix
func (e *Exporter) generateFilename(baseFilename string) string {
	if !e.options.UseTimestampSuffix {
		return baseFilename
	}

	timestamp := time.Now().Format("20060102-150405")

	// Replace {timestamp} placeholder in pattern
	if e.options.FilenamePattern != "" {
		pattern := strings.ReplaceAll(e.options.FilenamePattern, "{timestamp}", timestamp)

		// Add extension based on format
		var ext string
		switch e.options.Format {
		case FormatJSON:
			ext = ".json"
		case FormatCSV:
			ext = ".csv"
		case FormatText, FormatRipgrepCommand:
			ext = ".txt"
		default:
			ext = ".txt"
		}

		return pattern + ext
	}

	// Fallback: add timestamp before extension
	ext := filepath.Ext(baseFilename)
	name := strings.TrimSuffix(baseFilename, ext)

	if ext == "" {
		switch e.options.Format {
		case FormatJSON:
			ext = ".json"
		case FormatCSV:
			ext = ".csv"
		default:
			ext = ".txt"
		}
	}

	return fmt.Sprintf("%s-%s%s", name, timestamp, ext)
}

// escapeRegexForShell escapes regex patterns for safe use in shell commands
func (e *Exporter) escapeRegexForShell(pattern string) string {
	// Escape single quotes by replacing them with '\''
	escaped := strings.ReplaceAll(pattern, "'", `'\''`)

	// Additional escaping for special shell characters if needed
	// This is conservative - ripgrep handles most regex patterns well
	return escaped
}

// exportToJSON exports lines in JSON format
func (e *Exporter) exportToJSON(ctx context.Context, lines []string, lineNumbers []int) (ExportResult, error) {
	start := time.Now()

	exportLines := make([]ExportLine, len(lines))
	for i, line := range lines {
		lineNum := i + 1
		if len(lineNumbers) > i {
			lineNum = lineNumbers[i] + 1
		}

		exportLine := ExportLine{
			Number:  lineNum,
			Content: line,
		}

		if e.options.IncludeTimestamp {
			exportLine.Timestamp = time.Now()
		}

		exportLines[i] = exportLine
	}

	data := ExportData{
		Metadata: ExportMetadata{
			ExportTime:   time.Now(),
			ExportFormat: e.options.Format.String(),
		},
		Lines: exportLines,
		Stats: ExportStats{
			FilteredLineCount: len(lines),
			ExportTime:        time.Since(start),
		},
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return ExportResult{
			Success: false,
			Error:   fmt.Errorf("failed to marshal JSON: %w", err),
		}, err
	}

	result := ExportResult{
		Success:        true,
		Content:        string(jsonBytes),
		LinesExported:  len(lines),
		BytesExported:  int64(len(jsonBytes)),
		ExportDuration: time.Since(start),
	}

	return result, nil
}

// exportToCSV exports lines in CSV format
func (e *Exporter) exportToCSV(ctx context.Context, lines []string, lineNumbers []int) (ExportResult, error) {
	start := time.Now()

	var builder strings.Builder
	writer := csv.NewWriter(&builder)

	// Set custom delimiter if specified
	if e.options.CustomDelimiter != "" && len(e.options.CustomDelimiter) == 1 {
		writer.Comma = rune(e.options.CustomDelimiter[0])
	}

	// Write header if requested
	if e.options.IncludeHeaders {
		header := []string{"line_number", "content"}
		if e.options.IncludeTimestamp {
			header = append(header, "timestamp")
		}
		writer.Write(header)
	}

	// Write data rows
	for i, line := range lines {
		lineNum := i + 1
		if len(lineNumbers) > i {
			lineNum = lineNumbers[i] + 1
		}

		record := []string{fmt.Sprintf("%d", lineNum), line}
		if e.options.IncludeTimestamp {
			record = append(record, time.Now().Format(e.options.TimestampFormat))
		}

		writer.Write(record)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return ExportResult{
			Success: false,
			Error:   fmt.Errorf("failed to write CSV: %w", err),
		}, err
	}

	content := builder.String()
	result := ExportResult{
		Success:        true,
		Content:        content,
		LinesExported:  len(lines),
		BytesExported:  int64(len(content)),
		ExportDuration: time.Since(start),
	}

	return result, nil
}

// exportFilteredToJSON exports FilterResult in structured JSON format
func (e *Exporter) exportFilteredToJSON(ctx context.Context, filterResult core.FilterResult, sourceFile string) (ExportResult, error) {
	start := time.Now()

	exportLines := make([]ExportLine, len(filterResult.MatchedLines))
	for i, line := range filterResult.MatchedLines {
		lineNum := i + 1
		if len(filterResult.LineNumbers) > i {
			lineNum = filterResult.LineNumbers[i] + 1
		}

		exportLine := ExportLine{
			Number:  lineNum,
			Content: line,
		}

		// Add highlights if available
		if highlights, exists := filterResult.MatchHighlights[filterResult.LineNumbers[i]]; exists {
			exportLine.Highlights = highlights
		}

		if e.options.IncludeTimestamp {
			exportLine.Timestamp = time.Now()
		}

		exportLines[i] = exportLine
	}

	data := ExportData{
		Metadata: ExportMetadata{
			ExportTime:   time.Now(),
			SourceFile:   sourceFile,
			ExportFormat: e.options.Format.String(),
		},
		Lines: exportLines,
		Stats: ExportStats{
			OriginalLineCount: filterResult.Stats.TotalLines,
			FilteredLineCount: filterResult.Stats.MatchedLines,
			ProcessingTime:    filterResult.Stats.ProcessingTime,
			ExportTime:        time.Since(start),
			PatternsUsed:      filterResult.Stats.PatternsUsed,
		},
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return ExportResult{
			Success: false,
			Error:   fmt.Errorf("failed to marshal JSON: %w", err),
		}, err
	}

	result := ExportResult{
		Success:        true,
		Content:        string(jsonBytes),
		LinesExported:  len(filterResult.MatchedLines),
		BytesExported:  int64(len(jsonBytes)),
		ExportDuration: time.Since(start),
	}

	return result, nil
}

// exportFilteredToCSV exports FilterResult in CSV format
func (e *Exporter) exportFilteredToCSV(ctx context.Context, filterResult core.FilterResult, sourceFile string) (ExportResult, error) {
	start := time.Now()

	var builder strings.Builder
	writer := csv.NewWriter(&builder)

	// Set custom delimiter if specified
	if e.options.CustomDelimiter != "" && len(e.options.CustomDelimiter) == 1 {
		writer.Comma = rune(e.options.CustomDelimiter[0])
	}

	// Write header
	header := []string{"line_number", "content", "highlights_count"}
	if e.options.IncludeTimestamp {
		header = append(header, "timestamp")
	}
	writer.Write(header)

	// Write data rows
	for i, line := range filterResult.MatchedLines {
		lineNum := i + 1
		if len(filterResult.LineNumbers) > i {
			lineNum = filterResult.LineNumbers[i] + 1
		}

		highlightCount := 0
		if highlights, exists := filterResult.MatchHighlights[filterResult.LineNumbers[i]]; exists {
			highlightCount = len(highlights)
		}

		record := []string{
			fmt.Sprintf("%d", lineNum),
			line,
			fmt.Sprintf("%d", highlightCount),
		}

		if e.options.IncludeTimestamp {
			record = append(record, time.Now().Format(e.options.TimestampFormat))
		}

		writer.Write(record)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return ExportResult{
			Success: false,
			Error:   fmt.Errorf("failed to write CSV: %w", err),
		}, err
	}

	content := builder.String()
	result := ExportResult{
		Success:        true,
		Content:        content,
		LinesExported:  len(filterResult.MatchedLines),
		BytesExported:  int64(len(content)),
		ExportDuration: time.Since(start),
	}

	return result, nil
}
