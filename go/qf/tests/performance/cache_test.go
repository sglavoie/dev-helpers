// Package performance provides comprehensive benchmarks for the qf regex caching system.
// These benchmarks test the PatternManager LRU cache, pattern compilation performance,
// and filtering throughput with targets of:
// - >80% cache hit rate
// - Pattern compilation <20ms
// - 100K lines/sec for simple patterns
package performance

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
)

// Benchmark targets (performance requirements)
const (
	TargetCacheHitRate      = 0.80   // 80% cache hit rate
	TargetCompilationTimeMs = 20     // 20ms max compilation time
	TargetLinesPerSec       = 100000 // 100K lines/sec throughput
)

// Test data generators for reproducible benchmarks

// generateTestPatterns creates a set of realistic regex patterns for testing
func generateTestPatterns(count int) []string {
	patterns := []string{
		`ERROR`,
		`WARN`,
		`INFO`,
		`DEBUG`,
		`\d{4}-\d{2}-\d{2}`,                 // Date pattern
		`\d{2}:\d{2}:\d{2}`,                 // Time pattern
		`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`, // IP address
		`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`, // Email
		`HTTP/\d\.\d \d{3}`,           // HTTP status
		`\[.*?\]`,                     // Bracketed content
		`"[^"]*"`,                     // Quoted strings
		`user_\d+`,                    // User IDs
		`request_id=[a-f0-9-]{36}`,    // UUID pattern
		`duration=\d+ms`,              // Duration
		`memory=\d+MB`,                // Memory usage
		`cpu=\d+%`,                    // CPU usage
		`\b(GET|POST|PUT|DELETE)\b`,   // HTTP methods
		`/api/v\d+/\w+`,               // API endpoints
		`\berror\b`,                   // Case-insensitive error
		`\d{1,3}\.\d{1,3}\.\d{1,3}ms`, // Latency
	}

	// If we need more patterns, cycle through and add variations
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		base := patterns[i%len(patterns)]
		if i >= len(patterns) {
			// Add variations for additional patterns
			result = append(result, fmt.Sprintf("(%s|%s_\\d+)", base, base))
		} else {
			result = append(result, base)
		}
	}

	return result
}

// generateTestLogLines creates realistic log lines for filtering benchmarks
func generateTestLogLines(count int) []string {
	templates := []string{
		"2024-09-14 %s [INFO] Application started on port %d",
		"2024-09-14 %s [ERROR] Failed to connect to database: %s",
		"2024-09-14 %s [WARN] High memory usage detected: %dMB",
		"2024-09-14 %s [DEBUG] Processing user_%d request",
		"2024-09-14 %s [INFO] HTTP/1.1 %d GET /api/v1/users duration=%dms",
		"2024-09-14 %s [ERROR] Authentication failed for user_%d from %s",
		"2024-09-14 %s [INFO] Cache hit ratio: %.2f%% size=%d entries",
		"2024-09-14 %s [WARN] Rate limit exceeded for IP %s",
		"2024-09-14 %s [DEBUG] Query executed in %dms: %s",
		"2024-09-14 %s [INFO] Email sent to %s request_id=%s",
	}

	errors := []string{
		"connection timeout",
		"permission denied",
		"invalid credentials",
		"resource not found",
		"service unavailable",
	}

	ips := []string{
		"192.168.1.100",
		"10.0.0.1",
		"172.16.254.1",
		"203.0.113.42",
		"198.51.100.14",
	}

	emails := []string{
		"user@example.com",
		"admin@test.com",
		"service@company.org",
		"support@help.net",
		"noreply@system.io",
	}

	uuids := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"6ba7b811-9dad-11d1-80b4-00c04fd430c8",
		"01234567-89ab-cdef-0123-456789abcdef",
		"f47ac10b-58cc-4372-a567-0e02b2c3d479",
	}

	queries := []string{
		"SELECT * FROM users WHERE active = 1",
		"UPDATE sessions SET last_seen = NOW()",
		"INSERT INTO logs (level, message) VALUES (?, ?)",
		"DELETE FROM cache WHERE expired < NOW()",
		"CREATE INDEX idx_user_email ON users(email)",
	}

	lines := make([]string, count)
	rng := rand.New(rand.NewSource(42)) // Fixed seed for reproducible results

	for i := 0; i < count; i++ {
		template := templates[rng.Intn(len(templates))]
		timestamp := fmt.Sprintf("%02d:%02d:%02d.%03d",
			rng.Intn(24), rng.Intn(60), rng.Intn(60), rng.Intn(1000))

		switch template {
		case templates[0]: // Application started
			lines[i] = fmt.Sprintf(template, timestamp, 8080+rng.Intn(1000))
		case templates[1]: // Database error
			lines[i] = fmt.Sprintf(template, timestamp, errors[rng.Intn(len(errors))])
		case templates[2]: // Memory warning
			lines[i] = fmt.Sprintf(template, timestamp, 500+rng.Intn(2048))
		case templates[3]: // Debug user request
			lines[i] = fmt.Sprintf(template, timestamp, 1000+rng.Intn(9999))
		case templates[4]: // HTTP request
			lines[i] = fmt.Sprintf(template, timestamp, 200+rng.Intn(400), 10+rng.Intn(500))
		case templates[5]: // Auth failure
			lines[i] = fmt.Sprintf(template, timestamp, 1000+rng.Intn(9999), ips[rng.Intn(len(ips))])
		case templates[6]: // Cache stats
			lines[i] = fmt.Sprintf(template, timestamp, 75.0+rng.Float64()*20, 100+rng.Intn(1000))
		case templates[7]: // Rate limit
			lines[i] = fmt.Sprintf(template, timestamp, ips[rng.Intn(len(ips))])
		case templates[8]: // Query debug
			lines[i] = fmt.Sprintf(template, timestamp, 1+rng.Intn(100), queries[rng.Intn(len(queries))])
		case templates[9]: // Email sent
			lines[i] = fmt.Sprintf(template, timestamp, emails[rng.Intn(len(emails))], uuids[rng.Intn(len(uuids))])
		}
	}

	return lines
}

// PatternManager LRU Cache Benchmarks

// BenchmarkPatternManagerCacheHit tests cache hit performance
func BenchmarkPatternManagerCacheHit(b *testing.B) {
	patterns := generateTestPatterns(50)
	manager := core.NewPatternManager(core.WithMaxSize(100))

	// Pre-populate cache
	for _, pattern := range patterns {
		manager.Get(pattern)
	}

	// Reset stats but keep cache populated
	manager.ResetStats()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			pattern := patterns[rng.Intn(len(patterns))]
			_, _ = manager.Get(pattern)
		}
	})

	// Verify cache hit rate meets target
	stats := manager.Stats()
	hitRate := stats.HitRate()
	if hitRate < TargetCacheHitRate {
		b.Errorf("Cache hit rate %.2f%% below target %.2f%%", hitRate*100, TargetCacheHitRate*100)
	}

	b.ReportMetric(hitRate*100, "hit_rate_%")
	b.ReportMetric(float64(stats.Size), "cache_size")
}

// BenchmarkPatternManagerCacheMiss tests cache miss and compilation performance
func BenchmarkPatternManagerCacheMiss(b *testing.B) {
	manager := core.NewPatternManager(core.WithMaxSize(100))
	patterns := generateTestPatterns(1000) // More patterns than cache size

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			// Use unique pattern each time to force cache miss
			pattern := fmt.Sprintf("%s_%d", patterns[rng.Intn(len(patterns))], rng.Int())
			_, _ = manager.Get(pattern)
		}
	})

	stats := manager.Stats()
	b.ReportMetric(stats.HitRate()*100, "hit_rate_%")
	b.ReportMetric(float64(stats.Misses), "misses")
}

// BenchmarkPatternManagerConcurrentAccess tests thread safety and concurrent performance
func BenchmarkPatternManagerConcurrentAccess(b *testing.B) {
	patterns := generateTestPatterns(100)
	manager := core.NewPatternManager(core.WithMaxSize(200))

	// Pre-populate some patterns
	for i := 0; i < 50; i++ {
		manager.Get(patterns[i])
	}

	var wg sync.WaitGroup
	concurrency := 10

	b.ResetTimer()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(int64(workerID)))

			operations := b.N / concurrency
			for j := 0; j < operations; j++ {
				pattern := patterns[rng.Intn(len(patterns))]
				_, _ = manager.Get(pattern)
			}
		}(i)
	}

	wg.Wait()

	stats := manager.Stats()
	b.ReportMetric(stats.HitRate()*100, "hit_rate_%")
	b.ReportMetric(float64(stats.Size), "cache_size")
}

// BenchmarkPatternManagerLRUEviction tests LRU eviction performance
func BenchmarkPatternManagerLRUEviction(b *testing.B) {
	cacheSize := 50
	manager := core.NewPatternManager(core.WithMaxSize(cacheSize))
	patterns := generateTestPatterns(cacheSize * 3) // 3x cache size to force evictions

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Access patterns in sequence to trigger LRU eviction
		pattern := patterns[i%len(patterns)]
		_, _ = manager.Get(pattern)
	}

	stats := manager.Stats()
	if stats.Size > cacheSize {
		b.Errorf("Cache size %d exceeds maximum %d", stats.Size, cacheSize)
	}

	b.ReportMetric(float64(stats.Size), "final_cache_size")
	b.ReportMetric(stats.HitRate()*100, "hit_rate_%")
}

// Pattern Compilation Performance Benchmarks

// BenchmarkPatternCompilation tests raw regex compilation speed
func BenchmarkPatternCompilation(b *testing.B) {
	patterns := generateTestPatterns(100)

	for _, complexity := range []string{"simple", "complex"} {
		b.Run(complexity, func(b *testing.B) {
			var testPatterns []string
			if complexity == "simple" {
				testPatterns = patterns[:20] // Simple patterns
			} else {
				// Add more complex patterns
				testPatterns = append(patterns,
					`(?i)\b(error|exception|fail|panic)\b.*(?:line\s+\d+|at\s+\w+)`,
					`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})`,
					`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`,
				)
			}

			b.ResetTimer()
			start := time.Now()

			for i := 0; i < b.N; i++ {
				pattern := testPatterns[i%len(testPatterns)]
				_, err := regexp.Compile(pattern)
				if err != nil {
					b.Fatalf("Failed to compile pattern %q: %v", pattern, err)
				}
			}

			elapsed := time.Since(start)
			avgCompilationTime := elapsed.Milliseconds() / int64(b.N)

			if avgCompilationTime > TargetCompilationTimeMs {
				b.Errorf("Average compilation time %dms exceeds target %dms",
					avgCompilationTime, TargetCompilationTimeMs)
			}

			b.ReportMetric(float64(avgCompilationTime), "avg_compilation_ms")
		})
	}
}

// BenchmarkPatternValidation tests Pattern.Validate() performance
func BenchmarkPatternValidation(b *testing.B) {
	patterns := generateTestPatterns(50)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pattern := core.NewPattern(patterns[i%len(patterns)], core.Include, "#ff0000")
		isValid, err := pattern.Validate()
		if !isValid || err != nil {
			b.Fatalf("Pattern validation failed: %v", err)
		}
	}
}

// Filtering Throughput Benchmarks

// BenchmarkFilteringThroughputSimple tests filtering performance with simple patterns
func BenchmarkFilteringThroughputSimple(b *testing.B) {
	lines := generateTestLogLines(10000)
	engine := core.NewFilterEngine()

	// Add simple patterns
	simplePatterns := []string{"ERROR", "WARN", "INFO"}
	for i, pattern := range simplePatterns {
		err := engine.AddPattern(core.FilterPattern{
			ID:         fmt.Sprintf("pattern_%d", i),
			Expression: pattern,
			Type:       core.FilterInclude,
			Color:      "#ff0000",
			Created:    time.Now(),
		})
		if err != nil {
			b.Fatalf("Failed to add pattern: %v", err)
		}
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			b.Fatalf("Filter application failed: %v", err)
		}

		// Calculate throughput
		linesPerSec := float64(len(lines)) / result.Stats.ProcessingTime.Seconds()
		if i == 0 && linesPerSec < TargetLinesPerSec { // Only check on first iteration
			b.Errorf("Throughput %.0f lines/sec below target %d lines/sec",
				linesPerSec, TargetLinesPerSec)
		}

		if i == 0 { // Report metrics only once
			b.ReportMetric(linesPerSec, "lines_per_sec")
			b.ReportMetric(float64(result.Stats.ProcessingTime.Nanoseconds()/1000000), "processing_time_ms")
			b.ReportMetric(float64(result.Stats.CacheHits), "cache_hits")
			b.ReportMetric(float64(result.Stats.CacheMisses), "cache_misses")
		}
	}
}

// BenchmarkFilteringThroughputComplex tests filtering with complex patterns
func BenchmarkFilteringThroughputComplex(b *testing.B) {
	lines := generateTestLogLines(10000)
	engine := core.NewFilterEngine()

	// Add complex patterns
	complexPatterns := []string{
		`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\.\d{3}`,
		`\[(?:ERROR|WARN|INFO|DEBUG)\]`,
		`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`,
		`request_id=[a-f0-9-]{36}`,
		`duration=\d+ms`,
	}

	for i, pattern := range complexPatterns {
		err := engine.AddPattern(core.FilterPattern{
			ID:         fmt.Sprintf("complex_%d", i),
			Expression: pattern,
			Type:       core.FilterInclude,
			Color:      "#00ff00",
			Created:    time.Now(),
		})
		if err != nil {
			b.Fatalf("Failed to add complex pattern: %v", err)
		}
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			b.Fatalf("Filter application failed: %v", err)
		}

		if i == 0 { // Report metrics only once
			linesPerSec := float64(len(lines)) / result.Stats.ProcessingTime.Seconds()
			b.ReportMetric(linesPerSec, "lines_per_sec")
			b.ReportMetric(float64(result.Stats.ProcessingTime.Nanoseconds()/1000000), "processing_time_ms")
			b.ReportMetric(float64(result.Stats.PatternsUsed), "patterns_used")
		}
	}
}

// BenchmarkFilteringWithMixedPatterns tests realistic mixed include/exclude scenarios
func BenchmarkFilteringWithMixedPatterns(b *testing.B) {
	lines := generateTestLogLines(10000)
	engine := core.NewFilterEngine()

	// Add include patterns
	includePatterns := []string{"ERROR", "WARN", "HTTP/1.1 [45]\\d{2}"}
	for i, pattern := range includePatterns {
		err := engine.AddPattern(core.FilterPattern{
			ID:         fmt.Sprintf("include_%d", i),
			Expression: pattern,
			Type:       core.FilterInclude,
			Color:      "#ff0000",
			Created:    time.Now(),
		})
		if err != nil {
			b.Fatalf("Failed to add include pattern: %v", err)
		}
	}

	// Add exclude patterns
	excludePatterns := []string{"DEBUG", "user_test"}
	for i, pattern := range excludePatterns {
		err := engine.AddPattern(core.FilterPattern{
			ID:         fmt.Sprintf("exclude_%d", i),
			Expression: pattern,
			Type:       core.FilterExclude,
			Color:      "#0000ff",
			Created:    time.Now(),
		})
		if err != nil {
			b.Fatalf("Failed to add exclude pattern: %v", err)
		}
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			b.Fatalf("Filter application failed: %v", err)
		}

		if i == 0 { // Report metrics only once
			linesPerSec := float64(len(lines)) / result.Stats.ProcessingTime.Seconds()
			b.ReportMetric(linesPerSec, "lines_per_sec")
			b.ReportMetric(float64(result.Stats.MatchedLines), "matched_lines")
			b.ReportMetric(float64(result.Stats.PatternsUsed), "patterns_used")
		}
	}
}

// Cache Efficiency Metrics Benchmarks

// BenchmarkCacheEfficiencyMetrics tests comprehensive cache performance
func BenchmarkCacheEfficiencyMetrics(b *testing.B) {
	manager := core.NewPatternManager(core.WithMaxSize(100))
	patterns := generateTestPatterns(200) // 2x cache size

	// Simulate realistic access pattern: 80% requests to 20% of patterns
	popularPatterns := patterns[:20] // 20% of patterns get 80% of access
	rarePatterns := patterns[20:200] // 80% of patterns get 20% of access

	// Warmup phase to establish realistic cache state
	warmupOps := 500
	for i := 0; i < warmupOps; i++ {
		var pattern string
		if rand.Float64() < 0.8 { // 80% chance
			pattern = popularPatterns[rand.Intn(len(popularPatterns))]
		} else { // 20% chance
			pattern = rarePatterns[rand.Intn(len(rarePatterns))]
		}
		_, _ = manager.Get(pattern)
	}

	// Reset stats after warmup
	manager.ResetStats()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var pattern string
		if rand.Float64() < 0.8 { // 80% chance
			pattern = popularPatterns[rand.Intn(len(popularPatterns))]
		} else { // 20% chance
			pattern = rarePatterns[rand.Intn(len(rarePatterns))]
		}

		_, _ = manager.Get(pattern)
	}

	stats := manager.Stats()
	hitRate := stats.HitRate()

	// Verify performance targets
	if hitRate < TargetCacheHitRate {
		b.Errorf("Cache hit rate %.2f%% below target %.2f%%", hitRate*100, TargetCacheHitRate*100)
	}

	// Calculate cache efficiency score
	efficiency := hitRate * (float64(stats.Size) / float64(stats.MaxSize))

	b.ReportMetric(hitRate*100, "hit_rate_%")
	b.ReportMetric(float64(stats.Size), "cache_size")
	b.ReportMetric(efficiency*100, "cache_efficiency_%")
	b.ReportMetric(float64(stats.Hits), "total_hits")
	b.ReportMetric(float64(stats.Misses), "total_misses")
}

// BenchmarkCacheMemoryPressure tests cache behavior under memory pressure
func BenchmarkCacheMemoryPressure(b *testing.B) {
	smallCache := core.NewPatternManager(core.WithMaxSize(10))
	patterns := generateTestPatterns(100)

	var hitCount, missCount int

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pattern := patterns[i%len(patterns)]
		_, fromCache := smallCache.Get(pattern)

		if fromCache {
			hitCount++
		} else {
			missCount++
		}
	}

	stats := smallCache.Stats()

	b.ReportMetric(float64(hitCount), "hits")
	b.ReportMetric(float64(missCount), "misses")
	b.ReportMetric(float64(stats.Size), "final_cache_size")
	b.ReportMetric(stats.HitRate()*100, "hit_rate_%")
}

// Stress Testing Benchmarks

// BenchmarkHighVolumeFiltering tests performance with large datasets
func BenchmarkHighVolumeFiltering(b *testing.B) {
	// Generate large dataset
	lines := generateTestLogLines(100000) // 100K lines
	engine := core.NewFilterEngine()

	// Add moderate number of patterns
	patterns := generateTestPatterns(20)
	for i, pattern := range patterns {
		err := engine.AddPattern(core.FilterPattern{
			ID:         fmt.Sprintf("pattern_%d", i),
			Expression: pattern,
			Type:       core.FilterInclude,
			Color:      "#ff0000",
			Created:    time.Now(),
		})
		if err != nil {
			b.Fatalf("Failed to add pattern: %v", err)
		}
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		result, err := engine.ApplyFilters(ctx, lines)
		if err != nil {
			b.Fatalf("High volume filtering failed: %v", err)
		}

		if i == 0 { // Report metrics only once
			linesPerSec := float64(len(lines)) / result.Stats.ProcessingTime.Seconds()
			b.ReportMetric(linesPerSec, "lines_per_sec")
			b.ReportMetric(float64(result.Stats.TotalLines), "total_lines")
			b.ReportMetric(float64(result.Stats.MatchedLines), "matched_lines")
			b.ReportMetric(float64(result.Stats.ProcessingTime.Nanoseconds()/1000000), "processing_time_ms")
		}
	}
}

// Test Helper Functions

// TestCachePerformanceTargets validates that our benchmarks meet performance targets
func TestCachePerformanceTargets(t *testing.T) {
	manager := core.NewPatternManager(core.WithMaxSize(100))
	patterns := generateTestPatterns(50)

	// Pre-populate cache
	for _, pattern := range patterns {
		manager.Get(pattern)
	}

	// Test cache hit performance
	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		pattern := patterns[i%len(patterns)]
		_, fromCache := manager.Get(pattern)
		if !fromCache && i < len(patterns) {
			t.Errorf("Expected cache hit for pattern %s, got miss", pattern)
		}
	}

	elapsed := time.Since(start)
	stats := manager.Stats()

	// Validate performance targets
	hitRate := stats.HitRate()
	if hitRate < TargetCacheHitRate {
		t.Errorf("Cache hit rate %.2f%% below target %.2f%%", hitRate*100, TargetCacheHitRate*100)
	}

	avgLookupTime := elapsed.Nanoseconds() / int64(iterations)
	maxLookupTimeNs := int64(1000000) // 1ms maximum lookup time

	if avgLookupTime > maxLookupTimeNs {
		t.Errorf("Average lookup time %dns exceeds maximum %dns", avgLookupTime, maxLookupTimeNs)
	}

	t.Logf("Cache Performance Summary:")
	t.Logf("  Hit Rate: %.2f%%", hitRate*100)
	t.Logf("  Cache Size: %d/%d", stats.Size, stats.MaxSize)
	t.Logf("  Total Operations: %d hits, %d misses", stats.Hits, stats.Misses)
	t.Logf("  Average Lookup Time: %dns", avgLookupTime)
}
