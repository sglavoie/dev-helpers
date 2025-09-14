// Package performance contains example usage of the performance benchmarking system.
// This file demonstrates how to use the benchmarking tools and interpret results.
package performance

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/qf/internal/file"
)

// TestExampleBenchmarkUsage demonstrates how to use the performance benchmarking tools
func TestExampleBenchmarkUsage(t *testing.T) {
	// Skip in CI to avoid long-running tests
	if os.Getenv("CI") != "" {
		t.Skip("Skipping example in CI environment")
	}

	config := DefaultBenchmarkConfig

	// Create a test file
	generator := NewTestFileGenerator()
	testFile := generator.CreateTestFile(config.LargeFileSizeMB, t)
	defer os.Remove(testFile)

	t.Log("=== Example: FileReader Performance Testing ===")

	// 1. Test FileReader streaming threshold and performance
	reader := file.NewFileReader(
		file.WithStreamingThreshold(config.LargeFileSizeMB),
		file.WithBufferSize(64),
	)
	defer reader.Close()

	// Create a FileReader tester
	readerTester := NewFileReaderTester(config)

	// Test streaming mode detection
	readerTester.TestStreamingThreshold(t, reader, testFile, true)
	t.Log("✓ Streaming mode detection working correctly")

	// Test reading performance
	perfMetrics := readerTester.TestReadingPerformance(t, reader, testFile)
	t.Logf("✓ FileReader performance: %.2f MB/s, first line in %.2fms",
		perfMetrics.ThroughputMBps,
		float64(perfMetrics.FirstLineLatency.Nanoseconds())/1e6)

	t.Log("=== Example: BufferManager Performance Testing ===")

	// 2. Test BufferManager performance
	bufferConfig := file.DefaultBufferConfig
	bufferManager := file.NewBufferManager(bufferConfig)
	defer bufferManager.Close()

	// Create a BufferManager tester
	bufferTester := NewBufferManagerTester(config)

	// Open file in buffer manager
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := bufferManager.OpenFile(ctx, testFile, func(loaded, total int64, phase string) {
		t.Logf("Loading progress: %s - %d/%d lines", phase, loaded, total)
	})
	if err != nil {
		t.Fatalf("Failed to open file in buffer manager: %v", err)
	}

	// Test scrolling performance
	bufferTester.TestScrollingPerformance(t, bufferManager)
	t.Log("✓ Scrolling performance meets 60fps target")

	// Test memory efficiency
	bufferTester.TestMemoryEfficiency(t, bufferManager, testFile)
	t.Log("✓ Buffer manager memory usage within limits")

	t.Log("=== Example: Memory Monitoring ===")

	// 3. Demonstrate memory monitoring
	memMonitor := NewMemoryMonitor(100 * time.Millisecond)
	memMonitor.Start()

	// Simulate some memory-intensive operations
	largeData := make([][]byte, 1000)
	for i := range largeData {
		largeData[i] = make([]byte, 1024) // 1KB each
	}

	time.Sleep(2 * time.Second)

	samples := memMonitor.Stop()
	memStats := memMonitor.GetStats(samples)

	t.Logf("Memory monitoring results:")
	t.Logf("%s", memStats.String())

	t.Log("=== Example: Performance Validation ===")

	// 4. Demonstrate performance validation
	validator := NewPerformanceValidator(config)

	// Test various validations
	if err := validator.ValidateFileLoadTime(2*time.Second, 100); err != nil {
		t.Logf("File load time validation failed: %v", err)
	} else {
		t.Log("✓ File load time meets requirements")
	}

	if err := validator.ValidateMemoryUsage(80); err != nil {
		t.Logf("Memory usage validation failed: %v", err)
	} else {
		t.Log("✓ Memory usage meets requirements")
	}

	if err := validator.ValidateThroughput(35.0); err != nil {
		t.Logf("Throughput validation failed: %v", err)
	} else {
		t.Log("✓ Throughput meets requirements")
	}

	if err := validator.ValidateScrollPerformance(12, 65.0); err != nil {
		t.Logf("Scroll performance validation failed: %v", err)
	} else {
		t.Log("✓ Scroll performance meets 60fps requirements")
	}

	if err := validator.ValidateUIResponsiveness(30); err != nil {
		t.Logf("UI responsiveness validation failed: %v", err)
	} else {
		t.Log("✓ UI responsiveness meets requirements")
	}
}

// TestExampleCustomBenchmarkConfig demonstrates how to customize benchmark configuration
func TestExampleCustomBenchmarkConfig(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping example in CI environment")
	}

	// Create custom configuration for specific testing needs
	customConfig := BenchmarkConfig{
		SmallFileSizeMB:       5,    // Lower threshold for testing
		LargeFileSizeMB:       50,   // Lower threshold for faster tests
		VeryLargeFileSizeMB:   100,  // Reduced max size
		MaxFileLoadSeconds:    2.0,  // Stricter load time requirement
		MaxMemoryUsageMB:      50,   // Lower memory limit
		MinThroughputMBps:     25.0, // Slightly lower throughput requirement
		MaxUIResponseTimeMs:   40,   // Stricter UI response time
		MaxScrollLatencyMs:    12,   // Stricter scroll latency (~83fps)
		MinScrollFPS:          75.0, // Higher FPS requirement
		ScrollTestDurationSec: 5,    // Shorter test duration
	}

	t.Log("=== Example: Custom Configuration ===")
	t.Logf("Custom config - File load: %.1fs, Memory: %dMB, Throughput: %.1fMB/s",
		customConfig.MaxFileLoadSeconds,
		customConfig.MaxMemoryUsageMB,
		customConfig.MinThroughputMBps)
	t.Logf("Custom config - UI response: %dms, Scroll: %dms (%.1ffps)",
		customConfig.MaxUIResponseTimeMs,
		customConfig.MaxScrollLatencyMs,
		customConfig.MinScrollFPS)

	// Use custom configuration with validator
	validator := NewPerformanceValidator(customConfig)

	// Test stricter requirements
	if err := validator.ValidateMemoryUsage(60); err != nil {
		t.Logf("✓ Custom validation correctly failed: %v", err)
	} else {
		t.Error("Custom validation should have failed for 60MB usage")
	}

	if err := validator.ValidateScrollPerformance(15, 70.0); err != nil {
		t.Logf("✓ Custom scroll validation correctly failed: %v", err)
	} else {
		t.Error("Custom scroll validation should have failed")
	}

	// Test passing values
	if err := validator.ValidateMemoryUsage(40); err != nil {
		t.Errorf("Custom validation should have passed for 40MB: %v", err)
	} else {
		t.Log("✓ Custom validation passed for acceptable memory usage")
	}
}

// TestExamplePerformanceMeter demonstrates the PerformanceMeter usage
func TestExamplePerformanceMeter(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping example in CI environment")
	}

	t.Log("=== Example: Performance Meter ===")

	// Create and use a performance meter
	meter := NewPerformanceMeter()

	// Simulate processing
	for i := 0; i < 1000; i++ {
		if i == 0 {
			meter.RecordFirstLine()
		}

		// Simulate line processing
		lineContent := "This is a sample log line with some content"
		meter.RecordLineProcessed(len(lineContent))

		// Simulate some processing time
		if i%100 == 0 {
			time.Sleep(time.Millisecond)
		}

		// Simulate occasional errors
		if i%500 == 0 {
			meter.RecordError()
		}
	}

	// Get final metrics
	metrics := meter.Stop()

	t.Log("Performance Meter Results:")
	t.Logf("%s", metrics.String())

	// Validate the metrics make sense
	if metrics.LinesProcessed != 1000 {
		t.Errorf("Expected 1000 lines processed, got %d", metrics.LinesProcessed)
	}

	if metrics.ErrorCount != 2 { // Errors at i=0 and i=500
		t.Errorf("Expected 2 errors, got %d", metrics.ErrorCount)
	}

	if metrics.FirstLineLatency <= 0 {
		t.Error("First line latency should be positive")
	}

	if metrics.ThroughputMBps <= 0 {
		t.Error("Throughput should be positive")
	}

	t.Log("✓ Performance meter working correctly")
}

// TestExampleFileGeneration demonstrates test file generation with different patterns
func TestExampleFileGeneration(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping example in CI environment")
	}

	t.Log("=== Example: Test File Generation ===")

	generator := NewTestFileGenerator()

	// Generate files of different sizes
	testSizes := []int{1, 5, 10} // MB

	for _, sizeMB := range testSizes {
		t.Logf("Generating %dMB test file...", sizeMB)
		testFile := generator.CreateTestFile(sizeMB, t)

		// Verify file was created correctly
		stat, err := os.Stat(testFile)
		if err != nil {
			t.Errorf("Failed to stat generated file: %v", err)
			continue
		}

		actualSizeMB := float64(stat.Size()) / 1024 / 1024
		t.Logf("Generated file: %.1fMB actual size", actualSizeMB)

		// Quick validation - read first few lines
		file, err := os.Open(testFile)
		if err != nil {
			t.Errorf("Failed to open generated file: %v", err)
			os.Remove(testFile)
			continue
		}

		buffer := make([]byte, 512)
		n, err := file.Read(buffer)
		if err != nil && n == 0 {
			t.Errorf("Failed to read from generated file: %v", err)
		} else {
			preview := string(buffer[:min(n, 100)])
			t.Logf("File preview: %s...", preview)
		}

		file.Close()
		os.Remove(testFile)
	}

	t.Log("✓ Test file generation working correctly")
}

// helper function for minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestPerformanceRequirements validates that the current implementation meets all performance targets
func TestPerformanceRequirements(t *testing.T) {
	// This test validates the core performance requirements
	// It should fail if the implementation doesn't meet the specified targets

	config := DefaultBenchmarkConfig
	generator := NewTestFileGenerator()

	t.Run("CoreRequirements", func(t *testing.T) {
		// Generate a 100MB test file (streaming threshold)
		testFile := generator.CreateTestFile(config.LargeFileSizeMB, t)
		defer os.Remove(testFile)

		// Test FileReader performance
		reader := file.NewFileReader(
			file.WithStreamingThreshold(config.LargeFileSizeMB),
			file.WithBufferSize(64),
		)
		defer reader.Close()

		tester := NewFileReaderTester(config)
		metrics := tester.TestReadingPerformance(t, reader, testFile)

		// Verify core requirements
		if metrics.TotalDuration.Seconds() > config.MaxFileLoadSeconds {
			t.Errorf("REQUIREMENT FAILED: File load time %.2fs exceeds %.2fs target",
				metrics.TotalDuration.Seconds(), config.MaxFileLoadSeconds)
		} else {
			t.Logf("✅ File load time requirement met: %.2fs", metrics.TotalDuration.Seconds())
		}

		if metrics.ThroughputMBps < config.MinThroughputMBps {
			t.Errorf("REQUIREMENT FAILED: Throughput %.2fMB/s below %.2fMB/s target",
				metrics.ThroughputMBps, config.MinThroughputMBps)
		} else {
			t.Logf("✅ Throughput requirement met: %.2fMB/s", metrics.ThroughputMBps)
		}

		if metrics.FirstLineLatency.Milliseconds() > int64(config.MaxUIResponseTimeMs) {
			t.Errorf("REQUIREMENT FAILED: First line latency %dms exceeds %dms target",
				metrics.FirstLineLatency.Milliseconds(), config.MaxUIResponseTimeMs)
		} else {
			t.Logf("✅ UI responsiveness requirement met: %dms", metrics.FirstLineLatency.Milliseconds())
		}
	})

	t.Run("MemoryConstraints", func(t *testing.T) {
		// Test memory usage with buffer manager
		testFile := generator.CreateTestFile(config.LargeFileSizeMB, t)
		defer os.Remove(testFile)

		bufferManager := file.NewBufferManager(file.DefaultBufferConfig)
		defer bufferManager.Close()

		tester := NewBufferManagerTester(config)
		tester.TestMemoryEfficiency(t, bufferManager, testFile)

		memUsage := bufferManager.GetMemoryUsage() / 1024 / 1024
		if memUsage > int64(config.MaxMemoryUsageMB) {
			t.Errorf("REQUIREMENT FAILED: Memory usage %dMB exceeds %dMB limit",
				memUsage, config.MaxMemoryUsageMB)
		} else {
			t.Logf("✅ Memory usage requirement met: %dMB", memUsage)
		}
	})

	t.Run("ScrollingPerformance", func(t *testing.T) {
		// Test scrolling performance (60fps equivalent)
		testFile := generator.CreateTestFile(50, t) // Smaller file for scrolling test
		defer os.Remove(testFile)

		bufferManager := file.NewBufferManager(file.DefaultBufferConfig)
		defer bufferManager.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := bufferManager.OpenFile(ctx, testFile, nil)
		if err != nil {
			t.Fatalf("Failed to open file for scrolling test: %v", err)
		}

		tester := NewBufferManagerTester(config)
		tester.TestScrollingPerformance(t, bufferManager)

		t.Log("✅ Scrolling performance requirements validated")
	})
}
