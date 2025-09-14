// Package performance contains performance benchmarks for the qf application.
//
// This package focuses on measuring and validating performance characteristics
// of the large file streaming functionality, including memory usage, throughput,
// and responsiveness requirements as defined in the project specifications.
package performance

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/qf/internal/file"
)

// BenchmarkConfig defines configuration for streaming performance benchmarks
type BenchmarkConfig struct {
	// File size thresholds for testing
	SmallFileSizeMB     int // Files under this size should use regular loading
	LargeFileSizeMB     int // Files over this size should use streaming
	VeryLargeFileSizeMB int // Maximum test file size

	// Performance targets
	MaxFileLoadSeconds  float64 // Maximum time to start streaming a file
	MaxMemoryUsageMB    int     // Maximum memory usage during streaming
	MinThroughputMBps   float64 // Minimum streaming throughput in MB/s
	MaxUIResponseTimeMs int     // Maximum UI response time in milliseconds

	// Scrolling performance targets (equivalent to 60fps)
	MaxScrollLatencyMs    int     // Maximum latency for scroll operations
	MinScrollFPS          float64 // Minimum scroll frames per second
	ScrollTestDurationSec int     // Duration for sustained scrolling test
}

// DefaultBenchmarkConfig provides the target performance specifications
var DefaultBenchmarkConfig = BenchmarkConfig{
	SmallFileSizeMB:       10,
	LargeFileSizeMB:       100,
	VeryLargeFileSizeMB:   500,
	MaxFileLoadSeconds:    3.0,
	MaxMemoryUsageMB:      100,
	MinThroughputMBps:     30.0, // 30 MB/s minimum throughput
	MaxUIResponseTimeMs:   50,   // 50ms for UI responsiveness
	MaxScrollLatencyMs:    16,   // ~60fps equivalent (16.7ms per frame)
	MinScrollFPS:          60.0,
	ScrollTestDurationSec: 10,
}

// BenchmarkMetrics holds performance measurement results
type BenchmarkMetrics struct {
	FileLoadTime        time.Duration
	FirstLineTime       time.Duration
	ThroughputMBps      float64
	PeakMemoryUsageMB   int64
	AvgMemoryUsageMB    int64
	TotalLinesProcessed int64
	ProcessingErrors    int64
	ScrollLatencyMs     float64
	ScrollFPS           float64
}

// TestFileGenerator creates realistic test files for benchmarking
type TestFileGenerator struct {
	logPatterns []string
	mu          sync.Mutex
}

// NewTestFileGenerator creates a new test file generator with realistic log patterns
func NewTestFileGenerator() *TestFileGenerator {
	patterns := []string{
		"[2024-09-14T10:15:%02d.%03dZ] INFO  com.example.Application - Application started successfully",
		"[2024-09-14T10:15:%02d.%03dZ] DEBUG com.example.Config - Loading configuration from config.yaml",
		"[2024-09-14T10:15:%02d.%03dZ] INFO  com.example.Database - Database connection established (pool size: 20)",
		"[2024-09-14T10:15:%02d.%03dZ] WARN  com.example.Api - Deprecated API endpoint accessed: /api/v1/legacy",
		"[2024-09-14T10:15:%02d.%03dZ] ERROR com.example.Auth - Exception in user authentication: invalid token - user_id: %d",
		"[2024-09-14T10:15:%02d.%03dZ] INFO  com.example.Request - Processing user request for /api/data - request_id: %s",
		"[2024-09-14T10:15:%02d.%03dZ] ERROR com.example.Database - Database query failed: connection timeout after 30s",
		"[2024-09-14T10:15:%02d.%03dZ] INFO  com.example.Request - Request processed successfully in %dms - response_size: %dKB",
		"[2024-09-14T10:15:%02d.%03dZ] DEBUG com.example.Cache - Cache hit for key: user_session_%s - ttl: %ds",
		"[2024-09-14T10:15:%02d.%03dZ] WARN  com.example.RateLimit - Rate limit approaching for IP %s - requests: %d/100",
		"[2024-09-14T10:15:%02d.%03dZ] ERROR com.example.Processing - Exception in data processing: null pointer reference at line %d",
		"[2024-09-14T10:15:%02d.%03dZ] INFO  com.example.Task - Background task completed successfully - duration: %dms, items: %d",
		"[2024-09-14T10:15:%02d.%03dZ] TRACE com.example.Performance - Method execution time: %s took %dμs",
		"[2024-09-14T10:15:%02d.%03dZ] FATAL com.example.System - System critical error: out of memory - heap size: %dMB",
		"[2024-09-14T10:15:%02d.%03dZ] INFO  com.example.Security - Security scan completed - vulnerabilities: %d, severity: %s",
	}

	return &TestFileGenerator{
		logPatterns: patterns,
	}
}

// CreateTestFile generates a test file with realistic log content
func (g *TestFileGenerator) CreateTestFile(sizeMB int, t testing.TB) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("qf_bench_%dmb_*.log", sizeMB))
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	targetBytes := int64(sizeMB) * 1024 * 1024
	bytesWritten := int64(0)
	lineNumber := 0

	start := time.Now()

	for bytesWritten < targetBytes {
		// Generate varied log content
		pattern := g.logPatterns[lineNumber%len(g.logPatterns)]
		seconds := (lineNumber / 10) % 60
		millis := (lineNumber * 137) % 1000

		var line string
		switch lineNumber % len(g.logPatterns) {
		case 4: // Auth error with user ID
			line = fmt.Sprintf(pattern, seconds, millis, lineNumber%10000)
		case 5: // Request with ID
			line = fmt.Sprintf(pattern, seconds, millis, fmt.Sprintf("req_%d", lineNumber))
		case 7: // Request processing time
			line = fmt.Sprintf(pattern, seconds, millis, 50+lineNumber%500, 1+lineNumber%100)
		case 8: // Cache with session and TTL
			line = fmt.Sprintf(pattern, seconds, millis, fmt.Sprintf("sess_%d", lineNumber), 300+lineNumber%1800)
		case 9: // Rate limit with IP
			line = fmt.Sprintf(pattern, seconds, millis, fmt.Sprintf("192.168.1.%d", 1+lineNumber%254), 1+lineNumber%100)
		case 10: // Processing error with line number
			line = fmt.Sprintf(pattern, seconds, millis, lineNumber)
		case 11: // Background task with metrics
			line = fmt.Sprintf(pattern, seconds, millis, 100+lineNumber%900, 1+lineNumber%1000)
		case 12: // Performance trace
			line = fmt.Sprintf(pattern, seconds, millis, "processData", 50+lineNumber%950)
		case 13: // Fatal with heap size
			line = fmt.Sprintf(pattern, seconds, millis, 512+lineNumber%1024)
		case 14: // Security scan
			severities := []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}
			line = fmt.Sprintf(pattern, seconds, millis, lineNumber%5, severities[lineNumber%4])
		default:
			line = fmt.Sprintf(pattern, seconds, millis)
		}

		line += "\n"

		n, err := io.WriteString(tmpFile, line)
		if err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			t.Fatalf("Failed to write to temp file: %v", err)
		}

		bytesWritten += int64(n)
		lineNumber++

		// Progress feedback for very large files
		if sizeMB > 100 && lineNumber%100000 == 0 {
			elapsed := time.Since(start)
			progress := float64(bytesWritten) / float64(targetBytes) * 100
			t.Logf("Test file generation: %.1f%% complete (%d MB) - %v elapsed",
				progress, bytesWritten/1024/1024, elapsed)
		}
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to close temp file: %v", err)
	}

	elapsed := time.Since(start)
	stat, _ := os.Stat(tmpFile.Name())
	actualSizeMB := float64(stat.Size()) / 1024 / 1024

	t.Logf("Generated test file: %s (%.1f MB, %d lines) in %v",
		tmpFile.Name(), actualSizeMB, lineNumber, elapsed)

	return tmpFile.Name()
}

// BenchmarkFileReaderStreamingThreshold tests streaming mode activation thresholds
func BenchmarkFileReaderStreamingThreshold(b *testing.B) {
	config := DefaultBenchmarkConfig
	generator := NewTestFileGenerator()

	testCases := []struct {
		name            string
		fileSizeMB      int
		expectStreaming bool
	}{
		{"SmallFile_5MB", 5, false},
		{"MediumFile_50MB", 50, false},
		{"LargeFile_100MB", config.LargeFileSizeMB, true},
		{"VeryLargeFile_200MB", 200, true},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			testFile := generator.CreateTestFile(tc.fileSizeMB, b)
			defer os.Remove(testFile)

			reader := file.NewFileReader(
				file.WithStreamingThreshold(config.LargeFileSizeMB),
				file.WithBufferSize(64), // 64KB buffer
			)
			defer reader.Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				isStreaming, err := reader.IsStreamingMode(testFile)
				if err != nil {
					b.Fatalf("IsStreamingMode failed: %v", err)
				}

				if isStreaming != tc.expectStreaming {
					b.Errorf("Expected streaming=%v for %dMB file, got %v",
						tc.expectStreaming, tc.fileSizeMB, isStreaming)
				}
			}
		})
	}
}

// BenchmarkFileReaderThroughput measures file reading throughput
func BenchmarkFileReaderThroughput(b *testing.B) {
	config := DefaultBenchmarkConfig
	generator := NewTestFileGenerator()

	testSizes := []int{config.LargeFileSizeMB, 200, config.VeryLargeFileSizeMB}

	for _, sizeMB := range testSizes {
		b.Run(fmt.Sprintf("Throughput_%dMB", sizeMB), func(b *testing.B) {
			testFile := generator.CreateTestFile(sizeMB, b)
			defer os.Remove(testFile)

			reader := file.NewFileReader(
				file.WithStreamingThreshold(config.LargeFileSizeMB),
				file.WithBufferSize(64),
			)
			defer reader.Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

				start := time.Now()
				lineChan, err := reader.ReadFile(ctx, testFile)
				if err != nil {
					cancel()
					b.Fatalf("ReadFile failed: %v", err)
				}

				linesProcessed := int64(0)
				bytesProcessed := int64(0)
				var firstLineTime time.Duration

				for line := range lineChan {
					if linesProcessed == 0 {
						firstLineTime = time.Since(start)
					}
					linesProcessed++
					bytesProcessed += int64(len(line.Content))

					// Stop after processing significant amount for benchmark
					if bytesProcessed > int64(sizeMB)*1024*1024/4 {
						break
					}
				}

				elapsed := time.Since(start)
				cancel()

				// Calculate metrics
				throughputMBps := float64(bytesProcessed) / elapsed.Seconds() / 1024 / 1024

				b.ReportMetric(float64(elapsed.Nanoseconds()), "ns/file")
				b.ReportMetric(float64(firstLineTime.Nanoseconds()), "ns/firstline")
				b.ReportMetric(throughputMBps, "MB/s")
				b.ReportMetric(float64(linesProcessed), "lines")

				// Validate performance targets
				if elapsed.Seconds() > config.MaxFileLoadSeconds {
					b.Errorf("File loading too slow: %.2fs (target: %.2fs)",
						elapsed.Seconds(), config.MaxFileLoadSeconds)
				}

				if throughputMBps < config.MinThroughputMBps {
					b.Errorf("Throughput too low: %.2f MB/s (target: %.2f MB/s)",
						throughputMBps, config.MinThroughputMBps)
				}

				if firstLineTime.Milliseconds() > int64(config.MaxUIResponseTimeMs) {
					b.Errorf("First line too slow: %dms (target: %dms)",
						firstLineTime.Milliseconds(), config.MaxUIResponseTimeMs)
				}
			}
		})
	}
}

// BenchmarkMemoryUsage tests memory usage during streaming operations
func BenchmarkMemoryUsage(b *testing.B) {
	config := DefaultBenchmarkConfig
	generator := NewTestFileGenerator()

	testSizes := []int{config.LargeFileSizeMB, 200, 300}

	for _, sizeMB := range testSizes {
		b.Run(fmt.Sprintf("Memory_%dMB", sizeMB), func(b *testing.B) {
			testFile := generator.CreateTestFile(sizeMB, b)
			defer os.Remove(testFile)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Measure memory usage during streaming
				metrics := measureMemoryUsage(b, testFile, config)

				b.ReportMetric(float64(metrics.PeakMemoryUsageMB), "MB_peak")
				b.ReportMetric(float64(metrics.AvgMemoryUsageMB), "MB_avg")
				b.ReportMetric(float64(metrics.TotalLinesProcessed), "lines_processed")

				// Validate memory usage targets
				if metrics.PeakMemoryUsageMB > int64(config.MaxMemoryUsageMB) {
					b.Errorf("Peak memory usage too high: %d MB (target: %d MB)",
						metrics.PeakMemoryUsageMB, config.MaxMemoryUsageMB)
				}

				if metrics.AvgMemoryUsageMB > int64(config.MaxMemoryUsageMB) {
					b.Errorf("Average memory usage too high: %d MB (target: %d MB)",
						metrics.AvgMemoryUsageMB, config.MaxMemoryUsageMB)
				}
			}
		})
	}
}

// BenchmarkScrollingPerformance tests scroll performance (60fps equivalent)
func BenchmarkScrollingPerformance(b *testing.B) {
	config := DefaultBenchmarkConfig
	generator := NewTestFileGenerator()
	testFile := generator.CreateTestFile(config.LargeFileSizeMB, b)
	defer os.Remove(testFile)

	// Setup buffer manager for scrolling tests
	bufferConfig := file.DefaultBufferConfig
	bufferConfig.CircularBufferLines = 2000 // Reasonable buffer for scrolling
	bufferManager := file.NewBufferManager(bufferConfig)
	defer bufferManager.Close()

	ctx := context.Background()
	err := bufferManager.OpenFile(ctx, testFile, nil)
	if err != nil {
		b.Fatalf("Failed to open file in buffer manager: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.Run("ScrollLatency", func(b *testing.B) {
		// Test individual scroll operations
		for i := 0; i < b.N; i++ {
			start := time.Now()

			// Simulate scrolling to different positions
			lineNumber := 1 + (i*100)%10000
			_, err := bufferManager.GetLine(lineNumber)
			if err != nil {
				// Line might not be available, continue
			}

			latency := time.Since(start)

			if latency.Milliseconds() > int64(config.MaxScrollLatencyMs) {
				b.Errorf("Scroll latency too high: %dms (target: %dms)",
					latency.Milliseconds(), config.MaxScrollLatencyMs)
			}

			b.ReportMetric(float64(latency.Nanoseconds()), "ns/scroll")
		}
	})

	b.Run("SustainedScrolling", func(b *testing.B) {
		// Test sustained scrolling performance (equivalent to 60fps)
		for i := 0; i < b.N; i++ {
			start := time.Now()
			scrollOps := 0
			targetDuration := time.Duration(config.ScrollTestDurationSec) * time.Second

			for time.Since(start) < targetDuration {
				scrollStart := time.Now()

				// Simulate smooth scrolling
				lineNumber := 1 + scrollOps%5000
				_, err := bufferManager.GetLine(lineNumber)
				if err != nil {
					// Continue on error
				}

				scrollLatency := time.Since(scrollStart)
				scrollOps++

				// Maintain 60fps by sleeping if we're too fast
				targetFrameTime := time.Second / 60
				if scrollLatency < targetFrameTime {
					time.Sleep(targetFrameTime - scrollLatency)
				}
			}

			totalTime := time.Since(start)
			actualFPS := float64(scrollOps) / totalTime.Seconds()

			b.ReportMetric(actualFPS, "fps")
			b.ReportMetric(float64(scrollOps), "scroll_ops")

			if actualFPS < config.MinScrollFPS {
				b.Errorf("Scroll FPS too low: %.2f (target: %.2f)",
					actualFPS, config.MinScrollFPS)
			}
		}
	})
}

// BenchmarkBufferManagerPerformance tests buffer management efficiency
func BenchmarkBufferManagerPerformance(b *testing.B) {
	config := DefaultBenchmarkConfig
	generator := NewTestFileGenerator()

	testConfigs := []struct {
		name                string
		fileSizeMB          int
		circularBufferLines int
		contextWindowSize   int
	}{
		{"Standard_100MB", config.LargeFileSizeMB, 10000, 50},
		{"LargeBuffer_200MB", 200, 20000, 100},
		{"SmallBuffer_100MB", config.LargeFileSizeMB, 5000, 25},
	}

	for _, tc := range testConfigs {
		b.Run(tc.name, func(b *testing.B) {
			testFile := generator.CreateTestFile(tc.fileSizeMB, b)
			defer os.Remove(testFile)

			bufferConfig := file.DefaultBufferConfig
			bufferConfig.CircularBufferLines = tc.circularBufferLines
			bufferConfig.ContextWindowSize = tc.contextWindowSize

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				bufferManager := file.NewBufferManager(bufferConfig)

				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				start := time.Now()

				err := bufferManager.OpenFile(ctx, testFile, func(loaded, total int64, phase string) {
					// Progress callback - measure responsiveness
				})
				if err != nil {
					cancel()
					bufferManager.Close()
					b.Fatalf("Failed to open file: %v", err)
				}

				// Test various buffer operations
				operationStart := time.Now()

				// Test line access patterns
				for j := 0; j < 100; j++ {
					lineNum := 1 + j*10
					_, err := bufferManager.GetLine(lineNum)
					if err != nil {
						// Line may not be available, continue
					}
				}

				// Test context loading
				contextLines, err := bufferManager.LoadContext(ctx, 500, tc.contextWindowSize)
				if err == nil {
					b.ReportMetric(float64(len(contextLines)), "context_lines")
				}

				operationTime := time.Since(operationStart)
				totalTime := time.Since(start)

				// Get buffer statistics
				stats := bufferManager.GetStats()
				memoryUsage := bufferManager.GetMemoryUsage()

				cancel()
				bufferManager.Close()

				b.ReportMetric(float64(totalTime.Nanoseconds()), "ns/total")
				b.ReportMetric(float64(operationTime.Nanoseconds()), "ns/operations")
				b.ReportMetric(float64(memoryUsage)/1024/1024, "MB_memory")

				if circularSize, ok := stats["circular_buffer_size"].(int); ok {
					b.ReportMetric(float64(circularSize), "buffer_size")
				}

				// Validate performance
				memoryMB := memoryUsage / 1024 / 1024
				if memoryMB > int64(config.MaxMemoryUsageMB) {
					b.Errorf("Buffer memory usage too high: %d MB (target: %d MB)",
						memoryMB, config.MaxMemoryUsageMB)
				}
			}
		})
	}
}

// BenchmarkConcurrentAccess tests performance under concurrent access patterns
func BenchmarkConcurrentAccess(b *testing.B) {
	config := DefaultBenchmarkConfig
	generator := NewTestFileGenerator()
	testFile := generator.CreateTestFile(config.LargeFileSizeMB, b)
	defer os.Remove(testFile)

	concurrencyLevels := []int{1, 2, 4, 8}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrent_%d", concurrency), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				var wg sync.WaitGroup
				var totalOps int64
				var totalErrors int64

				start := time.Now()

				for c := 0; c < concurrency; c++ {
					wg.Add(1)
					go func(workerID int) {
						defer wg.Done()

						reader := file.NewFileReader(
							file.WithStreamingThreshold(config.LargeFileSizeMB),
							file.WithBufferSize(64),
						)
						defer reader.Close()

						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()

						lineChan, err := reader.ReadFile(ctx, testFile)
						if err != nil {
							atomic.AddInt64(&totalErrors, 1)
							return
						}

						ops := int64(0)
						for line := range lineChan {
							ops++
							_ = line // Use the line to avoid unused variable

							// Process a reasonable amount per worker
							if ops > 1000 {
								break
							}
						}

						atomic.AddInt64(&totalOps, ops)
					}(c)
				}

				wg.Wait()
				elapsed := time.Since(start)

				b.ReportMetric(float64(elapsed.Nanoseconds()), "ns/concurrent")
				b.ReportMetric(float64(totalOps), "total_ops")
				b.ReportMetric(float64(totalErrors), "errors")
				b.ReportMetric(float64(totalOps)/elapsed.Seconds(), "ops/sec")

				if totalErrors > 0 {
					b.Errorf("Concurrent access had %d errors", totalErrors)
				}
			}
		})
	}
}

// measureMemoryUsage measures memory usage during file processing
func measureMemoryUsage(b *testing.B, testFile string, config BenchmarkConfig) BenchmarkMetrics {
	reader := file.NewFileReader(
		file.WithStreamingThreshold(config.LargeFileSizeMB),
		file.WithBufferSize(64),
	)
	defer reader.Close()

	var metrics BenchmarkMetrics
	var memSamples []int64
	var mu sync.Mutex

	// Start memory monitoring
	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)

				mu.Lock()
				memSamples = append(memSamples, int64(m.Alloc))
				mu.Unlock()

			case <-done:
				return
			}
		}
	}()

	// Process file and measure
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	lineChan, err := reader.ReadFile(ctx, testFile)
	if err != nil {
		b.Fatalf("ReadFile failed: %v", err)
	}

	for line := range lineChan {
		metrics.TotalLinesProcessed++
		_ = line

		// Process a reasonable amount for memory measurement
		if metrics.TotalLinesProcessed > 10000 {
			break
		}
	}

	metrics.FileLoadTime = time.Since(start)

	// Stop memory monitoring
	done <- true

	// Analyze memory samples
	mu.Lock()
	defer mu.Unlock()

	if len(memSamples) > 0 {
		var total int64
		peak := memSamples[0]

		for _, sample := range memSamples {
			total += sample
			if sample > peak {
				peak = sample
			}
		}

		metrics.PeakMemoryUsageMB = peak / 1024 / 1024
		metrics.AvgMemoryUsageMB = (total / int64(len(memSamples))) / 1024 / 1024
	}

	return metrics
}

// BenchmarkRealWorldScenario simulates realistic usage patterns
func BenchmarkRealWorldScenario(b *testing.B) {
	config := DefaultBenchmarkConfig
	generator := NewTestFileGenerator()

	// Create a realistic large log file
	testFile := generator.CreateTestFile(150, b) // 150MB
	defer os.Remove(testFile)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate real-world usage: open file, apply filters, navigate matches
		bufferManager := file.NewBufferManager(file.DefaultBufferConfig)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// 1. Open large file
		openStart := time.Now()
		err := bufferManager.OpenFile(ctx, testFile, nil)
		if err != nil {
			cancel()
			bufferManager.Close()
			b.Fatalf("Failed to open file: %v", err)
		}
		openTime := time.Since(openStart)

		// 2. Simulate scrolling and line access (user browsing)
		browseStart := time.Now()
		for j := 0; j < 50; j++ {
			lineNum := 1 + j*100
			_, err := bufferManager.GetLine(lineNum)
			if err != nil {
				// Line might not be available, continue
			}

			// Simulate human reading time with small delay
			time.Sleep(time.Millisecond)
		}
		browseTime := time.Since(browseStart)

		// 3. Load context around specific lines (user examining matches)
		contextStart := time.Now()
		for j := 0; j < 5; j++ {
			centerLine := 500 + j*1000
			_, err := bufferManager.LoadContext(ctx, centerLine, 20)
			if err != nil {
				// Continue on error
			}
		}
		contextTime := time.Since(contextStart)

		// 4. Check final memory usage
		finalMemory := bufferManager.GetMemoryUsage()

		cancel()
		bufferManager.Close()

		// Report metrics
		b.ReportMetric(float64(openTime.Nanoseconds()), "ns/open")
		b.ReportMetric(float64(browseTime.Nanoseconds()), "ns/browse")
		b.ReportMetric(float64(contextTime.Nanoseconds()), "ns/context")
		b.ReportMetric(float64(finalMemory)/1024/1024, "MB_final")

		// Validate performance targets
		if openTime.Seconds() > config.MaxFileLoadSeconds {
			b.Errorf("File open too slow: %.2fs (target: %.2fs)",
				openTime.Seconds(), config.MaxFileLoadSeconds)
		}

		memoryMB := finalMemory / 1024 / 1024
		if memoryMB > int64(config.MaxMemoryUsageMB) {
			b.Errorf("Memory usage too high: %d MB (target: %d MB)",
				memoryMB, config.MaxMemoryUsageMB)
		}
	}
}
