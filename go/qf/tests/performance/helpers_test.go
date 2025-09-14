// Package performance provides helper utilities for performance benchmarks
// and validation of the qf Interactive Log Filter Composer.
package performance

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/sglavoie/dev-helpers/go/qf/internal/file"
)

// PerformanceValidator provides utilities for validating performance requirements
type PerformanceValidator struct {
	config BenchmarkConfig
	mu     sync.RWMutex
}

// NewPerformanceValidator creates a new performance validator with the given configuration
func NewPerformanceValidator(config BenchmarkConfig) *PerformanceValidator {
	return &PerformanceValidator{
		config: config,
	}
}

// ValidateFileLoadTime checks if file load time meets performance requirements
func (pv *PerformanceValidator) ValidateFileLoadTime(duration time.Duration, fileSizeMB int) error {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	maxDuration := time.Duration(pv.config.MaxFileLoadSeconds * float64(time.Second))
	if duration > maxDuration {
		return fmt.Errorf("file load time %.2fs exceeds target %.2fs for %dMB file",
			duration.Seconds(), maxDuration.Seconds(), fileSizeMB)
	}
	return nil
}

// ValidateMemoryUsage checks if memory usage stays within configured limits
func (pv *PerformanceValidator) ValidateMemoryUsage(memoryMB int64) error {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	if memoryMB > int64(pv.config.MaxMemoryUsageMB) {
		return fmt.Errorf("memory usage %dMB exceeds limit %dMB",
			memoryMB, pv.config.MaxMemoryUsageMB)
	}
	return nil
}

// ValidateThroughput checks if throughput meets minimum requirements
func (pv *PerformanceValidator) ValidateThroughput(throughputMBps float64) error {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	if throughputMBps < pv.config.MinThroughputMBps {
		return fmt.Errorf("throughput %.2fMB/s is below minimum %.2fMB/s",
			throughputMBps, pv.config.MinThroughputMBps)
	}
	return nil
}

// ValidateScrollPerformance checks if scroll performance meets 60fps equivalent
func (pv *PerformanceValidator) ValidateScrollPerformance(latencyMs int64, fps float64) error {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	if latencyMs > int64(pv.config.MaxScrollLatencyMs) {
		return fmt.Errorf("scroll latency %dms exceeds limit %dms (60fps = ~16ms)",
			latencyMs, pv.config.MaxScrollLatencyMs)
	}

	if fps < pv.config.MinScrollFPS {
		return fmt.Errorf("scroll FPS %.2f is below minimum %.2f",
			fps, pv.config.MinScrollFPS)
	}

	return nil
}

// ValidateUIResponsiveness checks if UI response time meets requirements
func (pv *PerformanceValidator) ValidateUIResponsiveness(responseTimeMs int64) error {
	pv.mu.RLock()
	defer pv.mu.RUnlock()

	if responseTimeMs > int64(pv.config.MaxUIResponseTimeMs) {
		return fmt.Errorf("UI response time %dms exceeds limit %dms",
			responseTimeMs, pv.config.MaxUIResponseTimeMs)
	}
	return nil
}

// MemoryMonitor provides real-time memory usage monitoring
type MemoryMonitor struct {
	samples  []MemorySample
	mu       sync.RWMutex
	interval time.Duration
	stopChan chan bool
	running  bool
}

// MemorySample represents a memory usage sample at a point in time
type MemorySample struct {
	Timestamp time.Time
	AllocMB   int64
	SysMB     int64
	NumGC     uint32
}

// NewMemoryMonitor creates a new memory monitor with specified sampling interval
func NewMemoryMonitor(interval time.Duration) *MemoryMonitor {
	return &MemoryMonitor{
		samples:  make([]MemorySample, 0),
		interval: interval,
		stopChan: make(chan bool),
	}
}

// Start begins memory monitoring in a separate goroutine
func (mm *MemoryMonitor) Start() {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if mm.running {
		return
	}

	mm.running = true
	mm.samples = mm.samples[:0] // Clear existing samples

	go mm.monitor()
}

// Stop ends memory monitoring and returns collected samples
func (mm *MemoryMonitor) Stop() []MemorySample {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if !mm.running {
		return mm.samples
	}

	mm.running = false
	mm.stopChan <- true

	// Return a copy of samples
	samples := make([]MemorySample, len(mm.samples))
	copy(samples, mm.samples)
	return samples
}

// monitor runs the memory monitoring loop
func (mm *MemoryMonitor) monitor() {
	ticker := time.NewTicker(mm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			sample := MemorySample{
				Timestamp: time.Now(),
				AllocMB:   int64(m.Alloc) / 1024 / 1024,
				SysMB:     int64(m.Sys) / 1024 / 1024,
				NumGC:     m.NumGC,
			}

			mm.mu.Lock()
			mm.samples = append(mm.samples, sample)
			mm.mu.Unlock()

		case <-mm.stopChan:
			return
		}
	}
}

// GetStats returns statistics about memory usage from collected samples
func (mm *MemoryMonitor) GetStats(samples []MemorySample) MemoryStats {
	if len(samples) == 0 {
		return MemoryStats{}
	}

	var stats MemoryStats
	stats.SampleCount = len(samples)
	stats.StartTime = samples[0].Timestamp
	stats.EndTime = samples[len(samples)-1].Timestamp
	stats.Duration = stats.EndTime.Sub(stats.StartTime)

	// Calculate min, max, and average
	stats.MinAllocMB = samples[0].AllocMB
	stats.MaxAllocMB = samples[0].AllocMB
	stats.MinSysMB = samples[0].SysMB
	stats.MaxSysMB = samples[0].SysMB

	var totalAlloc, totalSys int64

	for _, sample := range samples {
		totalAlloc += sample.AllocMB
		totalSys += sample.SysMB

		if sample.AllocMB < stats.MinAllocMB {
			stats.MinAllocMB = sample.AllocMB
		}
		if sample.AllocMB > stats.MaxAllocMB {
			stats.MaxAllocMB = sample.AllocMB
		}
		if sample.SysMB < stats.MinSysMB {
			stats.MinSysMB = sample.SysMB
		}
		if sample.SysMB > stats.MaxSysMB {
			stats.MaxSysMB = sample.SysMB
		}
	}

	stats.AvgAllocMB = totalAlloc / int64(len(samples))
	stats.AvgSysMB = totalSys / int64(len(samples))

	// Calculate GC frequency
	if len(samples) > 1 {
		gcDiff := samples[len(samples)-1].NumGC - samples[0].NumGC
		stats.GCCount = int(gcDiff)
		if stats.Duration > 0 {
			stats.GCFrequencyPerSec = float64(gcDiff) / stats.Duration.Seconds()
		}
	}

	return stats
}

// MemoryStats contains statistical information about memory usage
type MemoryStats struct {
	SampleCount       int
	StartTime         time.Time
	EndTime           time.Time
	Duration          time.Duration
	MinAllocMB        int64
	MaxAllocMB        int64
	AvgAllocMB        int64
	MinSysMB          int64
	MaxSysMB          int64
	AvgSysMB          int64
	GCCount           int
	GCFrequencyPerSec float64
}

// String returns a formatted string representation of memory stats
func (ms MemoryStats) String() string {
	return fmt.Sprintf(
		"Memory Stats: %d samples over %v\n"+
			"  Alloc: min=%dMB, max=%dMB, avg=%dMB\n"+
			"  Sys: min=%dMB, max=%dMB, avg=%dMB\n"+
			"  GC: %d collections (%.2f/sec)",
		ms.SampleCount, ms.Duration,
		ms.MinAllocMB, ms.MaxAllocMB, ms.AvgAllocMB,
		ms.MinSysMB, ms.MaxSysMB, ms.AvgSysMB,
		ms.GCCount, ms.GCFrequencyPerSec)
}

// PerformanceMeter measures various performance aspects during operations
type PerformanceMeter struct {
	startTime      time.Time
	firstLineTime  time.Time
	endTime        time.Time
	linesProcessed int64
	bytesProcessed int64
	errors         int64
	mu             sync.RWMutex
}

// NewPerformanceMeter creates a new performance meter
func NewPerformanceMeter() *PerformanceMeter {
	return &PerformanceMeter{
		startTime: time.Now(),
	}
}

// RecordFirstLine records the time when the first line was processed
func (pm *PerformanceMeter) RecordFirstLine() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.firstLineTime.IsZero() {
		pm.firstLineTime = time.Now()
	}
}

// RecordLineProcessed increments the count of lines processed
func (pm *PerformanceMeter) RecordLineProcessed(lineLength int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.linesProcessed++
	pm.bytesProcessed += int64(lineLength)
}

// RecordError increments the error count
func (pm *PerformanceMeter) RecordError() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.errors++
}

// Stop stops the performance meter and returns final metrics
func (pm *PerformanceMeter) Stop() PerformanceMetrics {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.endTime = time.Now()

	duration := pm.endTime.Sub(pm.startTime)
	var firstLineLatency time.Duration
	if !pm.firstLineTime.IsZero() {
		firstLineLatency = pm.firstLineTime.Sub(pm.startTime)
	}

	var throughputMBps float64
	if duration > 0 {
		throughputMBps = float64(pm.bytesProcessed) / duration.Seconds() / 1024 / 1024
	}

	return PerformanceMetrics{
		TotalDuration:    duration,
		FirstLineLatency: firstLineLatency,
		LinesProcessed:   pm.linesProcessed,
		BytesProcessed:   pm.bytesProcessed,
		ThroughputMBps:   throughputMBps,
		ErrorCount:       pm.errors,
	}
}

// PerformanceMetrics contains performance measurement results
type PerformanceMetrics struct {
	TotalDuration    time.Duration
	FirstLineLatency time.Duration
	LinesProcessed   int64
	BytesProcessed   int64
	ThroughputMBps   float64
	ErrorCount       int64
}

// String returns a formatted string representation of performance metrics
func (pm PerformanceMetrics) String() string {
	return fmt.Sprintf(
		"Performance: %.2fs total, %.2fms first line\n"+
			"  Processed: %d lines, %d bytes (%.2f MB/s)\n"+
			"  Errors: %d",
		pm.TotalDuration.Seconds(),
		float64(pm.FirstLineLatency.Nanoseconds())/1e6,
		pm.LinesProcessed,
		pm.BytesProcessed,
		pm.ThroughputMBps,
		pm.ErrorCount)
}

// FileReaderTester provides utilities for testing FileReader implementations
type FileReaderTester struct {
	config    BenchmarkConfig
	validator *PerformanceValidator
}

// NewFileReaderTester creates a new FileReader tester
func NewFileReaderTester(config BenchmarkConfig) *FileReaderTester {
	return &FileReaderTester{
		config:    config,
		validator: NewPerformanceValidator(config),
	}
}

// TestStreamingThreshold tests that streaming mode is activated appropriately
func (frt *FileReaderTester) TestStreamingThreshold(t *testing.T, reader file.FileReader, filePath string, expectedStreaming bool) {
	isStreaming, err := reader.IsStreamingMode(filePath)
	if err != nil {
		t.Fatalf("IsStreamingMode failed: %v", err)
	}

	if isStreaming != expectedStreaming {
		stat, _ := os.Stat(filePath)
		sizeMB := float64(stat.Size()) / 1024 / 1024
		t.Errorf("Expected streaming=%v for %.1fMB file, got %v",
			expectedStreaming, sizeMB, isStreaming)
	}
}

// TestReadingPerformance tests file reading performance and validates against targets
func (frt *FileReaderTester) TestReadingPerformance(t *testing.T, reader file.FileReader, filePath string) PerformanceMetrics {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start memory monitoring
	memMonitor := NewMemoryMonitor(100 * time.Millisecond)
	memMonitor.Start()

	// Start performance measurement
	perfMeter := NewPerformanceMeter()

	// Begin reading
	lineChan, err := reader.ReadFile(ctx, filePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// Process lines
	for line := range lineChan {
		if perfMeter.linesProcessed == 0 {
			perfMeter.RecordFirstLine()
		}
		perfMeter.RecordLineProcessed(len(line.Content))

		// Stop after processing reasonable amount for test
		if perfMeter.linesProcessed > 10000 {
			break
		}
	}

	// Stop measurements
	perfMetrics := perfMeter.Stop()
	memSamples := memMonitor.Stop()
	memStats := memMonitor.GetStats(memSamples)

	// Get file size for validation
	stat, _ := os.Stat(filePath)
	sizeMB := int(stat.Size() / 1024 / 1024)

	// Validate performance
	if err := frt.validator.ValidateFileLoadTime(perfMetrics.TotalDuration, sizeMB); err != nil {
		t.Errorf("File load performance: %v", err)
	}

	if err := frt.validator.ValidateMemoryUsage(memStats.MaxAllocMB); err != nil {
		t.Errorf("Memory usage: %v", err)
	}

	if err := frt.validator.ValidateThroughput(perfMetrics.ThroughputMBps); err != nil {
		t.Errorf("Throughput: %v", err)
	}

	if err := frt.validator.ValidateUIResponsiveness(perfMetrics.FirstLineLatency.Milliseconds()); err != nil {
		t.Errorf("UI responsiveness: %v", err)
	}

	// Log detailed results
	t.Logf("Performance test results for %dMB file:", sizeMB)
	t.Logf("%s", perfMetrics.String())
	t.Logf("%s", memStats.String())

	return perfMetrics
}

// BufferManagerTester provides utilities for testing BufferManager implementations
type BufferManagerTester struct {
	config    BenchmarkConfig
	validator *PerformanceValidator
}

// NewBufferManagerTester creates a new BufferManager tester
func NewBufferManagerTester(config BenchmarkConfig) *BufferManagerTester {
	return &BufferManagerTester{
		config:    config,
		validator: NewPerformanceValidator(config),
	}
}

// TestScrollingPerformance tests scrolling performance against 60fps target
func (bmt *BufferManagerTester) TestScrollingPerformance(t *testing.T, bufferManager *file.BufferManager) {
	scrollOps := 0
	totalLatency := time.Duration(0)
	maxLatency := time.Duration(0)

	testDuration := time.Duration(bmt.config.ScrollTestDurationSec) * time.Second
	start := time.Now()

	for time.Since(start) < testDuration {
		scrollStart := time.Now()

		// Simulate scrolling to different positions
		lineNumber := 1 + (scrollOps*50)%5000
		_, err := bufferManager.GetLine(lineNumber)
		if err != nil {
			// Line might not be available, continue testing
		}

		latency := time.Since(scrollStart)
		totalLatency += latency
		if latency > maxLatency {
			maxLatency = latency
		}
		scrollOps++

		// Maintain 60fps timing
		targetFrameTime := time.Second / 60
		if latency < targetFrameTime {
			time.Sleep(targetFrameTime - latency)
		}
	}

	actualDuration := time.Since(start)
	avgLatency := totalLatency / time.Duration(scrollOps)
	actualFPS := float64(scrollOps) / actualDuration.Seconds()

	// Validate performance
	if err := bmt.validator.ValidateScrollPerformance(avgLatency.Milliseconds(), actualFPS); err != nil {
		t.Errorf("Scroll performance: %v", err)
	}

	t.Logf("Scroll performance: %d ops in %.2fs (%.2f FPS)", scrollOps, actualDuration.Seconds(), actualFPS)
	t.Logf("Latency: avg=%.2fms, max=%.2fms",
		float64(avgLatency.Nanoseconds())/1e6,
		float64(maxLatency.Nanoseconds())/1e6)
}

// TestMemoryEfficiency tests buffer manager memory usage efficiency
func (bmt *BufferManagerTester) TestMemoryEfficiency(t *testing.T, bufferManager *file.BufferManager, filePath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start memory monitoring
	memMonitor := NewMemoryMonitor(50 * time.Millisecond)
	memMonitor.Start()

	// Open file in buffer manager
	err := bufferManager.OpenFile(ctx, filePath, nil)
	if err != nil {
		t.Fatalf("Failed to open file in buffer manager: %v", err)
	}

	// Simulate various buffer operations
	for i := 0; i < 100; i++ {
		lineNum := 1 + i*10
		_, err := bufferManager.GetLine(lineNum)
		if err != nil {
			// Continue on error
		}
	}

	// Load some context
	_, err = bufferManager.LoadContext(ctx, 500, 20)
	if err != nil {
		// Continue on error
	}

	// Stop monitoring and analyze
	memSamples := memMonitor.Stop()
	memStats := memMonitor.GetStats(memSamples)
	bufferMemory := bufferManager.GetMemoryUsage()

	// Validate memory usage
	if err := bmt.validator.ValidateMemoryUsage(memStats.MaxAllocMB); err != nil {
		t.Errorf("Buffer manager memory usage: %v", err)
	}

	t.Logf("Buffer manager memory efficiency:")
	t.Logf("  Buffer reported: %d MB", bufferMemory/1024/1024)
	t.Logf("%s", memStats.String())

	// Check buffer statistics
	stats := bufferManager.GetStats()
	if bufferSize, ok := stats["circular_buffer_size"].(int); ok {
		t.Logf("  Circular buffer size: %d", bufferSize)
	}
	if memoryUsage, ok := stats["memory_usage_mb"].(int64); ok {
		t.Logf("  Reported memory usage: %d MB", memoryUsage)
	}
}
