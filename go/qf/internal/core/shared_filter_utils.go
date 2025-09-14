// Package core provides shared filter utilities to reduce code duplication
// across the qf application's filtering components.
package core

import (
	"fmt"
	"regexp"
	"time"
)

// PatternValidator provides common validation logic for filter patterns
type PatternValidator struct {
	cache map[string]bool // Cache validation results for performance
}

// NewPatternValidator creates a new pattern validator with caching
func NewPatternValidator() *PatternValidator {
	return &PatternValidator{
		cache: make(map[string]bool),
	}
}

// ValidateAndCompile validates a regex pattern and returns the compiled regex
func (pv *PatternValidator) ValidateAndCompile(expression string) (*regexp.Regexp, error) {
	if expression == "" {
		return nil, fmt.Errorf("pattern expression cannot be empty")
	}

	// Check cache first
	if valid, exists := pv.cache[expression]; exists && !valid {
		return nil, fmt.Errorf("invalid regex pattern (cached): %q", expression)
	}

	compiled, err := regexp.Compile(expression)
	if err != nil {
		pv.cache[expression] = false
		return nil, fmt.Errorf("invalid regex pattern %q: %w", expression, err)
	}

	pv.cache[expression] = true
	return compiled, nil
}

// ClearCache clears the validation cache
func (pv *PatternValidator) ClearCache() {
	pv.cache = make(map[string]bool)
}

// FilterPatternConverter provides common conversion utilities between different pattern types
type FilterPatternConverter struct{}

// NewFilterPatternConverter creates a new pattern converter
func NewFilterPatternConverter() *FilterPatternConverter {
	return &FilterPatternConverter{}
}

// CoreToSession converts core.FilterPattern to session.FilterPattern format
func (c *FilterPatternConverter) CoreToSession(corePattern FilterPattern) interface{} {
	// This would return the session package's FilterPattern type
	// For now, return a map to represent the conversion
	return map[string]interface{}{
		"id":          corePattern.ID,
		"expression":  corePattern.Expression,
		"type":        int(corePattern.Type),
		"match_count": corePattern.MatchCount,
		"color":       corePattern.Color,
		"created":     corePattern.Created,
		"is_valid":    corePattern.IsValid,
	}
}

// SessionToCore converts session.FilterPattern to core.FilterPattern format
func (c *FilterPatternConverter) SessionToCore(sessionPattern interface{}) FilterPattern {
	// This would accept the session package's FilterPattern type
	// For now, accept a map and convert to core.FilterPattern
	if patternMap, ok := sessionPattern.(map[string]interface{}); ok {
		return FilterPattern{
			ID:         patternMap["id"].(string),
			Expression: patternMap["expression"].(string),
			Type:       FilterPatternType(patternMap["type"].(int)),
			MatchCount: patternMap["match_count"].(int),
			Color:      patternMap["color"].(string),
			Created:    patternMap["created"].(time.Time),
			IsValid:    patternMap["is_valid"].(bool),
		}
	}

	// Return empty pattern if conversion fails
	return FilterPattern{}
}

// LineFilterProcessor provides shared line filtering logic
type LineFilterProcessor struct {
	validator *PatternValidator
}

// NewLineFilterProcessor creates a new line filter processor
func NewLineFilterProcessor() *LineFilterProcessor {
	return &LineFilterProcessor{
		validator: NewPatternValidator(),
	}
}

// ShouldIncludeLine determines if a line should be included based on filter logic
// This is the shared implementation used by both the FilterEngine and UI components
func (lfp *LineFilterProcessor) ShouldIncludeLine(line string, includePatterns, excludePatterns []FilterPattern) (bool, error) {
	// Compile exclude patterns and check them first (veto logic)
	for _, pattern := range excludePatterns {
		if !pattern.IsValid {
			continue
		}

		compiled, err := lfp.validator.ValidateAndCompile(pattern.Expression)
		if err != nil {
			return false, fmt.Errorf("failed to compile exclude pattern %s: %w", pattern.ID, err)
		}

		if compiled.MatchString(line) {
			return false, nil // Line is excluded
		}
	}

	// If no include patterns, show all (minus excludes)
	if len(includePatterns) == 0 {
		return true, nil
	}

	// Check include patterns (OR logic)
	for _, pattern := range includePatterns {
		if !pattern.IsValid {
			continue
		}

		compiled, err := lfp.validator.ValidateAndCompile(pattern.Expression)
		if err != nil {
			return false, fmt.Errorf("failed to compile include pattern %s: %w", pattern.ID, err)
		}

		if compiled.MatchString(line) {
			return true, nil // Line matches an include pattern
		}
	}

	return false, nil // No include patterns matched
}

// GeneratePatternHighlights creates highlight information for matching patterns in a line
// This is shared between the FilterEngine and UI components
func (lfp *LineFilterProcessor) GeneratePatternHighlights(line string, patterns []FilterPattern) ([]Highlight, error) {
	var highlights []Highlight

	for _, pattern := range patterns {
		if !pattern.IsValid {
			continue
		}

		compiled, err := lfp.validator.ValidateAndCompile(pattern.Expression)
		if err != nil {
			return nil, fmt.Errorf("failed to compile pattern %s: %w", pattern.ID, err)
		}

		matches := compiled.FindAllStringIndex(line, -1)
		for _, match := range matches {
			highlights = append(highlights, Highlight{
				Start:     match[0],
				End:       match[1],
				PatternID: pattern.ID,
				Color:     pattern.Color,
			})
		}
	}

	return highlights, nil
}

// BatchValidatePatterns validates multiple patterns efficiently
func (lfp *LineFilterProcessor) BatchValidatePatterns(patterns []FilterPattern) []ValidationError {
	var errors []ValidationError

	for i := range patterns {
		_, err := lfp.validator.ValidateAndCompile(patterns[i].Expression)
		if err != nil {
			errors = append(errors, ValidationError{
				PatternID: patterns[i].ID,
				Pattern:   patterns[i].Expression,
				Reason:    "regex compilation failed",
				Err:       err,
			})
			// Mark pattern as invalid
			patterns[i].IsValid = false
		} else {
			// Mark pattern as valid
			patterns[i].IsValid = true
		}
	}

	return errors
}

// Global instances for shared usage
var (
	// DefaultValidator is a shared pattern validator instance
	DefaultValidator = NewPatternValidator()

	// DefaultConverter is a shared pattern converter instance
	DefaultConverter = NewFilterPatternConverter()

	// DefaultLineProcessor is a shared line filter processor instance
	DefaultLineProcessor = NewLineFilterProcessor()
)

// Convenience functions using global instances

// ValidatePattern validates a pattern using the default validator
func ValidatePattern(expression string) error {
	_, err := DefaultValidator.ValidateAndCompile(expression)
	return err
}

// ShouldIncludeLine determines if a line should be included using default processor
func ShouldIncludeLine(line string, includePatterns, excludePatterns []FilterPattern) (bool, error) {
	return DefaultLineProcessor.ShouldIncludeLine(line, includePatterns, excludePatterns)
}

// GenerateHighlights generates highlights using the default processor
func GenerateHighlights(line string, patterns []FilterPattern) ([]Highlight, error) {
	return DefaultLineProcessor.GeneratePatternHighlights(line, patterns)
}
