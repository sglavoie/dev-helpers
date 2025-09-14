package contract

import (
	"context"
	"fmt"

	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
)

// filterEngineAdapter adapts our core.FilterEngine to the contract test interface
type filterEngineAdapter struct {
	engine core.FilterEngine
}

func (a *filterEngineAdapter) AddPattern(pattern FilterPattern) error {
	corePattern := core.FilterPattern{
		ID:         pattern.ID,
		Expression: pattern.Expression,
		Type:       core.FilterPatternType(pattern.Type),
		Color:      pattern.Color,
	}
	return a.engine.AddPattern(corePattern)
}

func (a *filterEngineAdapter) RemovePattern(id string) error {
	return a.engine.RemovePattern(id)
}

func (a *filterEngineAdapter) UpdatePattern(id string, pattern FilterPattern) error {
	corePattern := core.FilterPattern{
		ID:         pattern.ID,
		Expression: pattern.Expression,
		Type:       core.FilterPatternType(pattern.Type),
		Color:      pattern.Color,
	}
	return a.engine.UpdatePattern(id, corePattern)
}

func (a *filterEngineAdapter) GetPattern(id string) (FilterPattern, error) {
	patterns := a.GetPatterns()
	for _, p := range patterns {
		if p.ID == id {
			return p, nil
		}
	}
	return FilterPattern{}, fmt.Errorf("pattern with ID %s not found", id)
}

func (a *filterEngineAdapter) GetPatterns() []FilterPattern {
	corePatterns := a.engine.GetPatterns()
	contractPatterns := make([]FilterPattern, len(corePatterns))
	for i, p := range corePatterns {
		contractPatterns[i] = FilterPattern{
			ID:         p.ID,
			Expression: p.Expression,
			Type:       FilterPatternType(p.Type),
			Color:      p.Color,
			IsValid:    p.IsValid,
			MatchCount: p.MatchCount,
			Created:    p.Created,
		}
	}
	return contractPatterns
}

func (a *filterEngineAdapter) ClearPatterns() {
	a.engine.ClearPatterns()
}

func (a *filterEngineAdapter) ValidatePattern(expression string) error {
	return a.engine.ValidatePattern(expression)
}

func (a *filterEngineAdapter) ApplyFilters(ctx context.Context, lines []string) (FilterResult, error) {
	result, err := a.engine.ApplyFilters(ctx, lines)
	if err != nil {
		return FilterResult{}, err
	}

	// Convert result back to contract test format
	matchedLines := make([]string, len(result.MatchedLines))
	lineNumbers := make([]int, len(result.MatchedLines))
	highlights := make(map[int][]Highlight)

	copy(matchedLines, result.MatchedLines)

	for i := range matchedLines {
		lineNumbers[i] = i + 1 // Simple line numbering for contract test
	}

	return FilterResult{
		MatchedLines:    matchedLines,
		LineNumbers:     lineNumbers,
		MatchHighlights: highlights,
		Stats: FilterStats{
			TotalLines:     len(lines),
			MatchedLines:   len(matchedLines),
			ProcessingTime: result.Stats.ProcessingTime,
			CacheHits:      int(result.Stats.CacheHits),
			CacheMisses:    int(result.Stats.CacheMisses),
		},
	}, nil
}

func (a *filterEngineAdapter) GetCacheStats() (hits int, misses int, size int) {
	return a.engine.GetCacheStats()
}
