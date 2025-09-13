# Filtering Engine Contracts

**Date**: 2025-09-13
**Purpose**: Define interfaces for regex pattern compilation, filtering logic, and performance optimization

## Filter Engine Interface

```go
// FilterEngine is the core filtering system
type FilterEngine interface {
    // SetFilterSet updates the active filter configuration
    SetFilterSet(filterSet FilterSet) error

    // ApplyFilters processes lines through include/exclude logic
    ApplyFilters(ctx context.Context, lines []Line) ([]Line, error)

    // ApplyFiltersStream processes streaming lines
    ApplyFiltersStream(ctx context.Context, input <-chan Line, output chan<- Line) error

    // ValidatePattern checks regex syntax and performance
    ValidatePattern(pattern string) (*PatternValidation, error)

    // GetStatistics returns filtering performance metrics
    GetStatistics() FilterStatistics

    // ClearCache removes cached compiled patterns
    ClearCache()
}

type PatternValidation struct {
    IsValid        bool
    Error          string
    CompileTime    time.Duration
    EstimatedPerf  PerformanceRating
    Warnings       []string
}

type PerformanceRating int

const (
    PerfFast PerformanceRating = iota    // Simple patterns, good performance
    PerfModerate                         // Complex patterns, acceptable performance
    PerfSlow                            // Potentially slow patterns
    PerfDangerous                       // Patterns that could cause exponential backtracking
)

type FilterStatistics struct {
    TotalLinesProcessed int64
    FilteredLines       int64
    AvgProcessingTime   time.Duration
    CacheHitRate       float64
    PatternStats       map[string]PatternStats
}

type PatternStats struct {
    MatchCount      int64
    ProcessingTime  time.Duration
    CacheHits       int64
    CacheMisses     int64
}
```

**Implementation Contract**:

- MUST validate all regex patterns before use
- MUST cache compiled patterns for performance
- MUST handle context cancellation gracefully
- MUST provide thread-safe operations
- MUST detect and warn about dangerous regex patterns

## Pattern Manager Interface

```go
// PatternManager handles pattern compilation and caching
type PatternManager interface {
    // CompilePattern compiles and caches regex pattern
    CompilePattern(id string, expression string) (*CompiledPattern, error)

    // GetPattern retrieves cached compiled pattern
    GetPattern(id string) (*CompiledPattern, bool)

    // RemovePattern removes pattern from cache
    RemovePattern(id string)

    // ClearCache removes all cached patterns
    ClearCache()

    // GetCacheStats returns cache performance metrics
    GetCacheStats() CacheStatistics
}

type CompiledPattern struct {
    ID           string
    Expression   string
    Regex        *regexp.Regexp
    CompileTime  time.Duration
    AccessCount  int64
    LastAccessed time.Time
    IsValid      bool
    Error        string
}

type CacheStatistics struct {
    Size         int
    MaxSize      int
    HitRate      float64
    Evictions    int64
    CompileTime  time.Duration
}
```

**Implementation Contract**:

- MUST implement LRU cache eviction
- MUST be thread-safe for concurrent pattern access
- MUST track access statistics for optimization
- MUST handle invalid regex patterns gracefully
- MUST respect configured cache size limits

## Pattern Validator Interface

```go
// PatternValidator checks regex patterns for correctness and performance
type PatternValidator interface {
    // Validate checks syntax and estimates performance
    Validate(pattern string) ValidationResult

    // CheckSafety detects potentially dangerous patterns
    CheckSafety(pattern string) SafetyResult

    // SuggestOptimizations provides pattern improvement suggestions
    SuggestOptimizations(pattern string) []OptimizationSuggestion
}

type ValidationResult struct {
    IsValid      bool
    SyntaxError  string
    Performance  PerformanceRating
    Warnings     []string
    TestResults  TestResults
}

type SafetyResult struct {
    IsSafe              bool
    DangerLevel         DangerLevel
    PotentialIssues     []SafetyIssue
    RecommendedTimeout  time.Duration
}

type DangerLevel int

const (
    SafePattern DangerLevel = iota
    CautiousPattern
    DangerousPattern
    ProhibitedPattern
)

type SafetyIssue struct {
    Type        IssueType
    Description string
    Example     string
    Suggestion  string
}

type IssueType int

const (
    ExponentialBacktracking IssueType = iota
    CatastrophicBacktracking
    InfiniteLoop
    ExcessiveMemoryUsage
)

type TestResults struct {
    TestCases       []PatternTest
    AverageTime     time.Duration
    MaxTime         time.Duration
    MemoryUsage     int64
}

type PatternTest struct {
    Input       string
    Matches     bool
    MatchTime   time.Duration
    MatchGroups []string
}

type OptimizationSuggestion struct {
    Type         OptimizationType
    Original     string
    Optimized    string
    Improvement  string
    Explanation  string
}

type OptimizationType int

const (
    ReduceBacktracking OptimizationType = iota
    SimplifyQuantifiers
    UseCharacterClasses
    EliminateAlternation
    AnchorPattern
)
```

**Implementation Contract**:

- MUST test patterns against common inputs
- MUST detect catastrophic backtracking patterns
- MUST provide actionable optimization suggestions
- MUST complete validation within reasonable time limits
- MUST handle edge cases in regex syntax

## Match Highlighter Interface

```go
// MatchHighlighter handles pattern match highlighting and coloring
type MatchHighlighter interface {
    // FindMatches locates all pattern matches in text
    FindMatches(text string, patterns []CompiledPattern) []Match

    // HighlightText applies color formatting to matched text
    HighlightText(text string, matches []Match) string

    // GetColors returns color scheme for pattern highlighting
    GetColors() ColorScheme

    // SetColorScheme updates highlighting colors
    SetColorScheme(scheme ColorScheme) error
}

type Match struct {
    PatternID    string
    PatternType  PatternType
    Start        int
    End          int
    Text         string
    Groups       []string
    Color        string
}

type ColorScheme struct {
    Name            string
    IncludeColors   []string  // Colors for include patterns
    ExcludeColors   []string  // Colors for exclude patterns
    BackgroundColor string
    TextColor       string
    ErrorColor      string
    WarningColor    string
}
```

**Implementation Contract**:

- MUST support multiple overlapping matches
- MUST handle Unicode text correctly
- MUST provide accessible color combinations
- MUST support terminal color capabilities detection
- MUST generate valid ANSI escape sequences

## Filter Logic Interface

```go
// FilterLogic implements include/exclude filtering rules
type FilterLogic interface {
    // ProcessLine applies filtering rules to a single line
    ProcessLine(line Line, includePatterns []CompiledPattern, excludePatterns []CompiledPattern) FilterResult

    // ProcessBatch applies filtering to multiple lines efficiently
    ProcessBatch(ctx context.Context, lines []Line, filterSet FilterSet) ([]Line, error)

    // GetMatchStatistics returns statistics for processed lines
    GetMatchStatistics() MatchStatistics
}

type FilterResult struct {
    IsVisible    bool
    Matches      []Match
    ProcessTime  time.Duration
    Error        error
}

type MatchStatistics struct {
    TotalLines        int64
    IncludeMatches    int64
    ExcludeMatches    int64
    FilteredLines     int64
    ProcessingTime    time.Duration
    PatternEfficiency map[string]float64  // Matches per millisecond
}
```

**Implementation Contract**:

- MUST implement correct include/exclude logic (OR for include, veto for exclude)
- MUST handle empty filter sets correctly
- MUST provide accurate match statistics
- MUST process lines efficiently in batches
- MUST respect context cancellation

## Testing Contracts

### Filter Engine Tests

```go
func TestFilterEngineContract(t *testing.T) {
    testCases := []struct {
        name        string
        includes    []string
        excludes    []string
        input       []string
        expected    []string
    }{
        {
            name:     "Include only",
            includes: []string{"ERROR"},
            excludes: []string{},
            input:    []string{"INFO: Starting", "ERROR: Failed", "DEBUG: Details"},
            expected: []string{"ERROR: Failed"},
        },
        {
            name:     "Exclude only",
            includes: []string{},
            excludes: []string{"DEBUG"},
            input:    []string{"INFO: Starting", "ERROR: Failed", "DEBUG: Details"},
            expected: []string{"INFO: Starting", "ERROR: Failed"},
        },
        {
            name:     "Include and exclude",
            includes: []string{"ERROR"},
            excludes: []string{"connection"},
            input:    []string{"ERROR: Failed", "ERROR: connection timeout", "INFO: Done"},
            expected: []string{"ERROR: Failed"},
        },
        {
            name:     "Empty filters",
            includes: []string{},
            excludes: []string{},
            input:    []string{"Line 1", "Line 2", "Line 3"},
            expected: []string{"Line 1", "Line 2", "Line 3"},
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            // Test filter logic implementation
        })
    }
}
```

### Pattern Validation Tests

```go
func TestPatternValidationContract(t *testing.T) {
    dangerousPatterns := []struct {
        pattern     string
        dangerLevel DangerLevel
        reason      string
    }{
        {"(a+)+b", DangerousPattern, "Exponential backtracking"},
        {".*.*.*", CautiousPattern, "Multiple greedy quantifiers"},
        {"(a|a)*", DangerousPattern, "Alternation with identical branches"},
        {"^valid$", SafePattern, "Anchored pattern"},
    }

    for _, tc := range dangerousPatterns {
        t.Run(tc.pattern, func(t *testing.T) {
            result := validator.CheckSafety(tc.pattern)
            assert.Equal(t, tc.dangerLevel, result.DangerLevel)
        })
    }
}
```

### Performance Tests

```go
func TestFilteringPerformance(t *testing.T) {
    // Test with large datasets
    lines := generateTestLines(100000)
    patterns := []string{"ERROR", "WARNING", "INFO"}

    start := time.Now()
    result, err := engine.ApplyFilters(context.Background(), lines)
    duration := time.Since(start)

    assert.NoError(t, err)
    assert.Less(t, duration, 5*time.Second, "Filtering should complete within 5 seconds")
    assert.Greater(t, len(result), 0, "Should find some matches")
}

func BenchmarkFilteringEngine(b *testing.B) {
    lines := generateTestLines(10000)
    filterSet := FilterSet{
        Include: []Pattern{{Expression: "ERROR|WARNING"}},
        Exclude: []Pattern{{Expression: "connection timeout"}},
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := engine.ApplyFilters(context.Background(), lines)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## Error Handling Requirements

All filtering components MUST handle:

1. **Invalid Regex**: Provide clear syntax error messages
2. **Timeout**: Detect and abort slow regex operations
3. **Memory Limits**: Prevent excessive memory usage
4. **Context Cancellation**: Respect cancellation signals
5. **Thread Safety**: Support concurrent filtering operations
6. **Pattern Conflicts**: Handle overlapping or contradictory patterns

## Performance Requirements

Filtering engine MUST meet these targets:

- **Pattern Compilation**: <20ms per pattern
- **Line Processing**: >100K lines/second for simple patterns
- **Memory Usage**: <100MB for 1M line buffer
- **Cache Hit Rate**: >80% for typical usage patterns
- **Startup Time**: <500ms to initialize with 50 cached patterns
- **Context Switch**: <10ms between different filter sets

---
*Filtering Engine Contracts complete: 2025-09-13*
