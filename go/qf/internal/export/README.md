# Export Package

The `export` package provides comprehensive export functionality for the qf Interactive Log Filter Composer. It supports multiple export formats, clipboard integration, and flexible output options for filtered log content and search commands.

## Features

### Export Formats

- **Plain Text**: Exports with optional line numbers and timestamps
- **Ripgrep Commands**: Generates executable ripgrep commands from filter patterns
- **JSON**: Structured export with metadata and highlighting information
- **CSV**: Comma-separated values for data analysis tools

### Key Capabilities

- **Clipboard Integration**: Direct copy to system clipboard using cross-platform clipboard package
- **File Export**: Atomic file writing with timestamp suffix support
- **Progress Tracking**: Real-time progress callbacks for large export operations
- **Configuration-Driven**: Flexible options for customizing export behavior
- **Multiple Output Targets**: Memory, clipboard, or file system

## Quick Start

```go
import (
    "context"
    "github.com/sglavoie/dev-helpers/go/qf/internal/config"
    "github.com/sglavoie/dev-helpers/go/qf/internal/export"
    "github.com/sglavoie/dev-helpers/go/qf/internal/session"
)

// Basic text export
cfg := config.NewDefaultConfig()
options := export.DefaultExportOptions()
exporter := export.NewExporter(cfg, options)

lines := []string{
    "[INFO] Application started",
    "[ERROR] Database connection failed",
    "[INFO] Retrying connection",
}
lineNumbers := []int{1, 2, 3}

ctx := context.Background()
result, err := exporter.ExportToText(ctx, lines, lineNumbers)
if err != nil {
    log.Printf("Export failed: %v", err)
    return
}

fmt.Printf("Exported %d lines:\n%s", result.LinesExported, result.Content)
```

## Export Options

The `ExportOptions` struct provides extensive customization:

```go
options := export.ExportOptions{
    Format:             export.FormatText,
    IncludeLineNumbers: true,
    LineNumberFormat:   "%6d: ",              // Custom line number format
    IncludeTimestamp:   true,
    TimestampFormat:    "2006-01-02 15:04:05", // Custom timestamp format
    IncludeHeaders:     true,                  // Metadata headers
    CustomDelimiter:    ",",                   // CSV delimiter
    OutputDirectory:    "/tmp/exports",        // File export directory
    FilenamePattern:    "qf-export-{timestamp}", // Filename template
    UseTimestampSuffix: true,                  // Automatic timestamp suffix
}
```

## Export Methods

### Text Export

```go
// Export to text format with line numbers
result, err := exporter.ExportToText(ctx, lines, lineNumbers)
```

### Clipboard Export

```go
// Copy filtered content directly to clipboard
result, err := exporter.ExportToClipboard(ctx, lines, lineNumbers)
```

### File Export

```go
// Export to file with timestamp suffix
result, err := exporter.ExportToFile(ctx, lines, lineNumbers, "my-export")
// Creates file: my-export-20230914-143022.txt
```

### Ripgrep Command Generation

```go
filterSet := session.FilterSet{
    Include: []session.FilterPattern{
        {Expression: "ERROR|WARN", IsValid: true},
        {Expression: "database", IsValid: true},
    },
    Exclude: []session.FilterPattern{
        {Expression: "DEBUG", IsValid: true},
    },
}

targetFiles := []string{"app.log", "error.log"}
result, err := exporter.GenerateRipgrepCommand(filterSet, targetFiles)
// Generates: rg '(ERROR|WARN|database)' --invert-match 'DEBUG' --line-number --color=always app.log error.log
```

### Structured Export (JSON/CSV)

```go
// JSON export with metadata and highlights
options.Format = export.FormatJSON
result, err := exporter.ExportFilteredContent(ctx, filterResult, "source.log")

// CSV export for data analysis
options.Format = export.FormatCSV
result, err := exporter.ExportFilteredContent(ctx, filterResult, "source.log")
```

## Progress Tracking

For large exports, you can track progress in real-time:

```go
exporter.SetProgressCallback(func(progress export.ExportProgress) {
    percentage := float64(progress.ProcessedLines) / float64(progress.TotalLines) * 100
    fmt.Printf("Progress: %.1f%% (%d/%d lines)\n",
        percentage, progress.ProcessedLines, progress.TotalLines)

    if progress.EstimatedTimeRemaining > 0 {
        fmt.Printf("ETA: %v\n", progress.EstimatedTimeRemaining)
    }
})

result, err := exporter.ExportToText(ctx, largeLineSet, lineNumbers)
```

## Export Results

All export methods return an `ExportResult` struct:

```go
type ExportResult struct {
    Success        bool          // Export completed successfully
    OutputPath     string        // Path to exported file (file exports only)
    Content        string        // Exported content (memory exports)
    LinesExported  int           // Number of lines exported
    BytesExported  int64         // Total bytes exported
    ExportDuration time.Duration // Time taken for export
    RipgrepCommand string        // Generated command (ripgrep format only)
    Error          error         // Any error that occurred
}
```

## Filename Generation

The exporter supports flexible filename generation:

- **Timestamp Suffix**: Automatically adds timestamps to avoid conflicts
- **Custom Patterns**: Use `{timestamp}` placeholder in filename patterns
- **Format Extensions**: Automatically adds appropriate extensions (.txt, .json, .csv)

```go
options := export.ExportOptions{
    FilenamePattern:    "filtered-logs-{timestamp}",
    UseTimestampSuffix: true,
    Format:             export.FormatJSON,
}
// Generates: filtered-logs-20230914-143022.json
```

## Ripgrep Command Generation

The package generates optimized ripgrep commands from filter patterns:

### Include Patterns (OR Logic)
Multiple include patterns are combined with OR logic:
```bash
rg '(pattern1|pattern2|pattern3)' files...
```

### Exclude Patterns (Veto Logic)
Exclude patterns use `--invert-match`:
```bash
rg 'include_pattern' --invert-match 'exclude1' --invert-match 'exclude2' files...
```

### Shell Escaping
Special characters are automatically escaped for safe shell execution:
```go
// Pattern with single quotes: "can't connect"
// Generated: rg 'can'\''t connect' file.log
```

## Error Handling

The export package provides comprehensive error handling:

```go
result, err := exporter.ExportToFile(ctx, lines, lineNumbers, "output")
if err != nil {
    if result.Error != nil {
        // Check specific error details
        log.Printf("Export error: %v", result.Error)
    }

    // Handle different error types
    switch {
    case os.IsNotExist(err):
        log.Printf("Output directory does not exist")
    case os.IsPermission(err):
        log.Printf("Permission denied writing to file")
    default:
        log.Printf("Unexpected error: %v", err)
    }
}
```

## Performance Considerations

- **Streaming**: Large exports are processed in chunks to manage memory usage
- **Atomic Writes**: File exports use temporary files to ensure consistency
- **Progress Callbacks**: Called every 1000 lines to balance responsiveness and performance
- **Context Support**: All operations support context cancellation for timeouts

## Integration with qf Components

The export package integrates seamlessly with other qf components:

- **Config**: Uses application configuration for default settings
- **Session**: Works with FilterSet and FilterPattern types
- **Core**: Processes FilterResult data with highlighting information
- **Clipboard**: Cross-platform clipboard support via atotto/clipboard

## Example Use Cases

### 1. Quick Log Analysis Export
```go
// Export filtered logs with line numbers for analysis
options := export.DefaultExportOptions()
options.IncludeLineNumbers = true
options.IncludeHeaders = true

result, _ := exporter.ExportToText(ctx, filteredLines, lineNumbers)
```

### 2. Sharing Filters as Commands
```go
// Generate ripgrep command to share with team
result, _ := exporter.GenerateRipgrepCommand(filterSet, []string{"*.log"})
fmt.Printf("Run this command: %s\n", result.RipgrepCommand)
```

### 3. Automated Report Generation
```go
// Export daily logs in multiple formats
formats := []export.ExportFormat{
    export.FormatText,  // For human reading
    export.FormatJSON,  // For automated processing
    export.FormatCSV,   // For spreadsheet analysis
}

for _, format := range formats {
    options.Format = format
    exporter := export.NewExporter(cfg, options)
    result, _ := exporter.ExportToFile(ctx, lines, lineNumbers, "daily-report")
}
```

### 4. Real-time Export Progress
```go
// Monitor large export operations
exporter.SetProgressCallback(func(progress export.ExportProgress) {
    if progress.ProcessedLines%10000 == 0 {
        log.Printf("Processed %d/%d lines in %v",
            progress.ProcessedLines, progress.TotalLines, progress.ElapsedTime)
    }
})
```

## Testing

The export package is designed to be easily testable:

```go
// Mock configuration for testing
cfg := config.NewDefaultConfig()
options := export.DefaultExportOptions()
options.OutputDirectory = "/tmp/test"

exporter := export.NewExporter(cfg, options)

// Test with sample data
testLines := []string{"test line 1", "test line 2"}
testLineNumbers := []int{1, 2}

result, err := exporter.ExportToText(context.Background(), testLines, testLineNumbers)
assert.NoError(t, err)
assert.True(t, result.Success)
assert.Equal(t, 2, result.LinesExported)
```

## Dependencies

- `github.com/atotto/clipboard`: Cross-platform clipboard access
- Standard library packages: `encoding/json`, `encoding/csv`, `context`, `os`, `path/filepath`
- Internal qf packages: `config`, `core`, `session`

This comprehensive export system provides flexible, efficient, and user-friendly export capabilities that integrate seamlessly with the qf Interactive Log Filter Composer.