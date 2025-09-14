package export

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/qf/internal/config"
	"github.com/sglavoie/dev-helpers/go/qf/internal/session"
)

func TestExportToText_BasicFunctionality(t *testing.T) {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()
	exporter := NewExporter(cfg, options)

	lines := []string{
		"[INFO] Application started",
		"[ERROR] Database connection failed",
		"[INFO] Retrying connection",
	}
	lineNumbers := []int{0, 1, 2}

	ctx := context.Background()
	result, err := exporter.ExportToText(ctx, lines, lineNumbers)

	if err != nil {
		t.Fatalf("ExportToText failed: %v", err)
	}

	if !result.Success {
		t.Fatal("Export should have succeeded")
	}

	if result.LinesExported != 3 {
		t.Errorf("Expected 3 lines exported, got %d", result.LinesExported)
	}

	if result.BytesExported <= 0 {
		t.Errorf("Expected positive bytes exported, got %d", result.BytesExported)
	}

	if result.ExportDuration <= 0 {
		t.Errorf("Expected positive export duration, got %v", result.ExportDuration)
	}

	// Check content includes line numbers (default option)
	if !strings.Contains(result.Content, "1:") {
		t.Error("Expected content to include line numbers")
	}

	// Check all lines are present
	for _, line := range lines {
		if !strings.Contains(result.Content, line) {
			t.Errorf("Expected content to include line: %s", line)
		}
	}
}

func TestExportToText_WithHeaders(t *testing.T) {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()
	options.IncludeHeaders = true
	exporter := NewExporter(cfg, options)

	lines := []string{"Test line"}
	lineNumbers := []int{0}

	ctx := context.Background()
	result, err := exporter.ExportToText(ctx, lines, lineNumbers)

	if err != nil {
		t.Fatalf("ExportToText with headers failed: %v", err)
	}

	// Check headers are present
	if !strings.Contains(result.Content, "# qf Export") {
		t.Error("Expected content to include qf Export header")
	}

	if !strings.Contains(result.Content, "# Lines: 1") {
		t.Error("Expected content to include line count header")
	}
}

func TestExportToText_WithTimestamp(t *testing.T) {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()
	options.IncludeTimestamp = true
	options.TimestampFormat = "2006-01-02"
	exporter := NewExporter(cfg, options)

	lines := []string{"Test line with timestamp"}
	lineNumbers := []int{0}

	ctx := context.Background()
	result, err := exporter.ExportToText(ctx, lines, lineNumbers)

	if err != nil {
		t.Fatalf("ExportToText with timestamp failed: %v", err)
	}

	// Check timestamp is present (should be today's date)
	today := time.Now().Format("2006-01-02")
	if !strings.Contains(result.Content, today) {
		t.Errorf("Expected content to include today's date: %s", today)
	}
}

func TestExportToText_EmptyLines(t *testing.T) {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()
	exporter := NewExporter(cfg, options)

	ctx := context.Background()
	result, err := exporter.ExportToText(ctx, []string{}, []int{})

	if err != nil {
		t.Fatalf("ExportToText with empty lines failed: %v", err)
	}

	if !result.Success {
		t.Fatal("Export should have succeeded even with empty lines")
	}

	if result.LinesExported != 0 {
		t.Errorf("Expected 0 lines exported, got %d", result.LinesExported)
	}

	if result.Content != "" {
		t.Errorf("Expected empty content, got: %s", result.Content)
	}
}

func TestGenerateRipgrepCommand_BasicCommand(t *testing.T) {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()
	exporter := NewExporter(cfg, options)

	filterSet := session.FilterSet{
		Name: "test-filters",
		Include: []session.FilterPattern{
			{
				ID:         "include-1",
				Expression: "ERROR",
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

	targetFiles := []string{"app.log"}
	result, err := exporter.GenerateRipgrepCommand(filterSet, targetFiles)

	if err != nil {
		t.Fatalf("GenerateRipgrepCommand failed: %v", err)
	}

	if !result.Success {
		t.Fatal("Command generation should have succeeded")
	}

	cmd := result.RipgrepCommand

	// Check basic command structure
	if !strings.HasPrefix(cmd, "rg ") {
		t.Error("Command should start with 'rg '")
	}

	// Check include pattern is present
	if !strings.Contains(cmd, "'ERROR'") {
		t.Error("Command should include ERROR pattern")
	}

	// Check exclude pattern is present
	if !strings.Contains(cmd, "--invert-match 'DEBUG'") {
		t.Error("Command should include inverted DEBUG pattern")
	}

	// Check target file is present
	if !strings.Contains(cmd, "app.log") {
		t.Error("Command should include target file")
	}

	// Check common options
	if !strings.Contains(cmd, "--line-number") {
		t.Error("Command should include --line-number option")
	}

	if !strings.Contains(cmd, "--color=always") {
		t.Error("Command should include --color=always option")
	}
}

func TestGenerateRipgrepCommand_MultipleIncludePatterns(t *testing.T) {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()
	exporter := NewExporter(cfg, options)

	filterSet := session.FilterSet{
		Include: []session.FilterPattern{
			{
				ID:         "include-1",
				Expression: "ERROR",
				Type:       session.FilterInclude,
				IsValid:    true,
			},
			{
				ID:         "include-2",
				Expression: "WARN",
				Type:       session.FilterInclude,
				IsValid:    true,
			},
		},
	}

	targetFiles := []string{"test.log"}
	result, err := exporter.GenerateRipgrepCommand(filterSet, targetFiles)

	if err != nil {
		t.Fatalf("GenerateRipgrepCommand with multiple patterns failed: %v", err)
	}

	cmd := result.RipgrepCommand

	// Should use OR logic for multiple includes: (ERROR|WARN)
	if !strings.Contains(cmd, "'(ERROR|WARN)'") {
		t.Errorf("Command should combine include patterns with OR logic, got: %s", cmd)
	}
}

func TestGenerateRipgrepCommand_EscapeSpecialCharacters(t *testing.T) {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()
	exporter := NewExporter(cfg, options)

	filterSet := session.FilterSet{
		Include: []session.FilterPattern{
			{
				ID:         "include-1",
				Expression: "can't connect",
				Type:       session.FilterInclude,
				IsValid:    true,
			},
		},
	}

	targetFiles := []string{"test.log"}
	result, err := exporter.GenerateRipgrepCommand(filterSet, targetFiles)

	if err != nil {
		t.Fatalf("GenerateRipgrepCommand with special characters failed: %v", err)
	}

	cmd := result.RipgrepCommand

	// Check that single quotes are properly escaped
	if strings.Contains(cmd, "'can't connect'") {
		t.Errorf("Single quotes should be escaped in command: %s", cmd)
	}

	// Should contain escaped version
	if !strings.Contains(cmd, "can'\\''t connect") {
		t.Errorf("Should contain properly escaped single quotes: %s", cmd)
	}
}

func TestExportJSON_BasicFunctionality(t *testing.T) {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()
	exporter := NewExporter(cfg, options)

	lines := []string{
		"First line",
		"Second line",
	}
	lineNumbers := []int{0, 1}

	ctx := context.Background()
	result, err := exporter.exportToJSON(ctx, lines, lineNumbers)

	if err != nil {
		t.Fatalf("JSON export failed: %v", err)
	}

	if !result.Success {
		t.Fatal("JSON export should have succeeded")
	}

	if result.LinesExported != 2 {
		t.Errorf("Expected 2 lines exported, got %d", result.LinesExported)
	}

	// Check that result contains valid JSON structure indicators
	if !strings.Contains(result.Content, "\"lines\"") {
		t.Error("JSON should contain 'lines' field")
	}

	if !strings.Contains(result.Content, "\"metadata\"") {
		t.Error("JSON should contain 'metadata' field")
	}

	if !strings.Contains(result.Content, "First line") {
		t.Error("JSON should contain the first line content")
	}
}

func TestExportCSV_BasicFunctionality(t *testing.T) {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()
	options.IncludeHeaders = true
	exporter := NewExporter(cfg, options)

	lines := []string{
		"CSV line 1",
		"CSV line 2",
	}
	lineNumbers := []int{10, 20}

	ctx := context.Background()
	result, err := exporter.exportToCSV(ctx, lines, lineNumbers)

	if err != nil {
		t.Fatalf("CSV export failed: %v", err)
	}

	if !result.Success {
		t.Fatal("CSV export should have succeeded")
	}

	// Check CSV structure
	if !strings.Contains(result.Content, "line_number,content") {
		t.Error("CSV should contain headers")
	}

	if !strings.Contains(result.Content, "11,CSV line 1") {
		t.Error("CSV should contain first line with 1-based line number")
	}

	if !strings.Contains(result.Content, "21,CSV line 2") {
		t.Error("CSV should contain second line with 1-based line number")
	}
}

func TestDefaultExportOptions(t *testing.T) {
	options := DefaultExportOptions()

	if options.Format != FormatText {
		t.Errorf("Expected default format to be Text, got %v", options.Format)
	}

	if !options.IncludeLineNumbers {
		t.Error("Expected default to include line numbers")
	}

	if options.LineNumberFormat != "%6d: " {
		t.Errorf("Expected default line number format '%%6d: ', got %s", options.LineNumberFormat)
	}

	if !options.IncludeHeaders {
		t.Error("Expected default to include headers")
	}

	if !options.UseTimestampSuffix {
		t.Error("Expected default to use timestamp suffix")
	}
}

func TestExportFormat_String(t *testing.T) {
	tests := []struct {
		format   ExportFormat
		expected string
	}{
		{FormatText, "text"},
		{FormatRipgrepCommand, "ripgrep"},
		{FormatJSON, "json"},
		{FormatCSV, "csv"},
		{ExportFormat(999), "unknown"},
	}

	for _, test := range tests {
		if test.format.String() != test.expected {
			t.Errorf("Format %v should return %s, got %s", test.format, test.expected, test.format.String())
		}
	}
}

func TestGenerateFilename(t *testing.T) {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()
	options.UseTimestampSuffix = true
	options.FilenamePattern = "test-export-{timestamp}"
	options.Format = FormatJSON

	exporter := NewExporter(cfg, options)

	filename := exporter.generateFilename("base-name")

	// Should contain timestamp and JSON extension
	if !strings.Contains(filename, "test-export-") {
		t.Error("Filename should contain pattern prefix")
	}

	if !strings.HasSuffix(filename, ".json") {
		t.Error("Filename should have JSON extension")
	}

	// Should contain timestamp (format: YYYYMMDD-HHMMSS)
	// We can't test exact timestamp, but we can check length and pattern
	parts := strings.Split(filename, "-")
	if len(parts) < 3 {
		t.Errorf("Expected timestamp in filename, got: %s", filename)
	}
}

func TestNewExporter(t *testing.T) {
	cfg := config.NewDefaultConfig()
	options := DefaultExportOptions()

	exporter := NewExporter(cfg, options)

	if exporter == nil {
		t.Fatal("NewExporter should return non-nil exporter")
	}

	if exporter.config != cfg {
		t.Error("Exporter should store provided config")
	}

	if exporter.options.Format != options.Format {
		t.Error("Exporter should store provided options")
	}
}

func TestNewExporter_NilConfig(t *testing.T) {
	options := DefaultExportOptions()

	exporter := NewExporter(nil, options)

	if exporter == nil {
		t.Fatal("NewExporter should handle nil config gracefully")
	}

	if exporter.config == nil {
		t.Error("Exporter should create default config when nil provided")
	}
}
