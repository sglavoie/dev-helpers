# Makefile for qf Performance Benchmarks
# Provides convenient targets for running various performance tests

.PHONY: help bench bench-all bench-streaming bench-memory bench-scrolling bench-real-world \
        bench-concurrent profile-cpu profile-mem profile-trace clean examples validate

# Default target
help:
	@echo "qf Performance Benchmarks"
	@echo "=========================="
	@echo ""
	@echo "Available targets:"
	@echo "  bench           - Run all performance benchmarks"
	@echo "  bench-all       - Run all benchmarks with extended timeout"
	@echo "  bench-streaming - Run streaming-specific benchmarks"
	@echo "  bench-memory    - Run memory usage benchmarks"
	@echo "  bench-scrolling - Run scrolling performance benchmarks"
	@echo "  bench-real-world- Run real-world scenario benchmarks"
	@echo "  bench-concurrent- Run concurrent access benchmarks"
	@echo "  examples        - Run example usage tests"
	@echo "  validate        - Run performance requirements validation"
	@echo ""
	@echo "Profiling targets:"
	@echo "  profile-cpu     - Run benchmarks with CPU profiling"
	@echo "  profile-mem     - Run benchmarks with memory profiling"
	@echo "  profile-trace   - Run benchmarks with execution tracing"
	@echo ""
	@echo "Utility targets:"
	@echo "  clean          - Clean up generated files"
	@echo ""
	@echo "Environment variables:"
	@echo "  BENCH_TIME     - Benchmark duration (default: 10s)"
	@echo "  BENCH_COUNT    - Number of benchmark runs (default: 3)"
	@echo "  FILE_SIZE      - Test file size in MB (default: 100)"

# Configuration
BENCH_TIME ?= 10s
BENCH_COUNT ?= 3
FILE_SIZE ?= 100
TIMEOUT ?= 10m
PKG = ./tests/performance/

# Run all benchmarks
bench:
	@echo "Running performance benchmarks..."
	go test -bench=. -benchmem -benchtime=$(BENCH_TIME) -count=$(BENCH_COUNT) -timeout=$(TIMEOUT) $(PKG)

# Run all benchmarks with extended timeout for large files
bench-all:
	@echo "Running extended performance benchmarks..."
	go test -bench=. -benchmem -benchtime=30s -count=5 -timeout=15m $(PKG)

# Run streaming-specific benchmarks
bench-streaming:
	@echo "Running streaming performance benchmarks..."
	go test -bench=BenchmarkFileReader -benchmem -benchtime=$(BENCH_TIME) -timeout=$(TIMEOUT) $(PKG)

# Run memory usage benchmarks
bench-memory:
	@echo "Running memory usage benchmarks..."
	go test -bench=BenchmarkMemory -benchmem -benchtime=$(BENCH_TIME) -timeout=$(TIMEOUT) $(PKG)

# Run scrolling performance benchmarks (60fps target)
bench-scrolling:
	@echo "Running scrolling performance benchmarks..."
	go test -bench=BenchmarkScrolling -benchmem -benchtime=$(BENCH_TIME) -timeout=$(TIMEOUT) $(PKG)

# Run real-world scenario benchmarks
bench-real-world:
	@echo "Running real-world scenario benchmarks..."
	go test -bench=BenchmarkRealWorld -benchmem -benchtime=$(BENCH_TIME) -timeout=$(TIMEOUT) $(PKG)

# Run concurrent access benchmarks
bench-concurrent:
	@echo "Running concurrent access benchmarks..."
	go test -bench=BenchmarkConcurrent -benchmem -benchtime=$(BENCH_TIME) -timeout=$(TIMEOUT) $(PKG)

# Run buffer manager benchmarks
bench-buffer:
	@echo "Running buffer manager benchmarks..."
	go test -bench=BenchmarkBufferManager -benchmem -benchtime=$(BENCH_TIME) -timeout=$(TIMEOUT) $(PKG)

# CPU profiling
profile-cpu:
	@echo "Running benchmarks with CPU profiling..."
	go test -bench=BenchmarkFileReaderThroughput -cpuprofile=cpu.prof -benchtime=$(BENCH_TIME) $(PKG)
	@echo "CPU profile saved to cpu.prof"
	@echo "Analyze with: go tool pprof cpu.prof"

# Memory profiling
profile-mem:
	@echo "Running benchmarks with memory profiling..."
	go test -bench=BenchmarkMemoryUsage -memprofile=mem.prof -benchtime=$(BENCH_TIME) $(PKG)
	@echo "Memory profile saved to mem.prof"
	@echo "Analyze with: go tool pprof mem.prof"

# Execution tracing
profile-trace:
	@echo "Running benchmarks with execution tracing..."
	go test -bench=BenchmarkScrollingPerformance -trace=trace.out -benchtime=$(BENCH_TIME) $(PKG)
	@echo "Trace saved to trace.out"
	@echo "Analyze with: go tool trace trace.out"

# Block profiling (for concurrency analysis)
profile-block:
	@echo "Running benchmarks with block profiling..."
	go test -bench=BenchmarkConcurrentAccess -blockprofile=block.prof -benchtime=$(BENCH_TIME) $(PKG)
	@echo "Block profile saved to block.prof"
	@echo "Analyze with: go tool pprof block.prof"

# Run example usage tests
examples:
	@echo "Running example usage tests..."
	go test -run=Example -v -timeout=5m $(PKG)

# Run performance requirements validation
validate:
	@echo "Validating performance requirements..."
	go test -run=TestPerformanceRequirements -v -timeout=$(TIMEOUT) $(PKG)

# Quick validation for CI
validate-quick:
	@echo "Running quick performance validation..."
	go test -run=TestPerformanceRequirements -v -timeout=2m $(PKG) -benchtime=100ms

# Run benchmarks with different file sizes
bench-sizes:
	@echo "Running benchmarks with various file sizes..."
	@for size in 50 100 200 300; do \
		echo "Testing with $${size}MB files..."; \
		FILE_SIZE=$${size} go test -bench=BenchmarkFileReaderThroughput -benchmem -benchtime=5s $(PKG) || true; \
	done

# Run memory stress test
stress-memory:
	@echo "Running memory stress test..."
	go test -bench=BenchmarkMemoryUsage -benchmem -benchtime=60s -timeout=5m $(PKG)

# Run concurrent stress test
stress-concurrent:
	@echo "Running concurrent access stress test..."
	go test -bench=BenchmarkConcurrentAccess -benchmem -benchtime=30s -timeout=3m $(PKG)

# Generate benchmark comparison
compare:
	@echo "Running benchmark comparison..."
	@if [ ! -f benchmark_baseline.txt ]; then \
		echo "Creating baseline benchmarks..."; \
		make bench > benchmark_baseline.txt; \
	fi
	@echo "Running current benchmarks..."
	make bench > benchmark_current.txt
	@echo "Comparing results..."
	@if command -v benchcmp >/dev/null 2>&1; then \
		benchcmp benchmark_baseline.txt benchmark_current.txt; \
	else \
		echo "Install benchcmp for comparison: go get golang.org/x/tools/cmd/benchcmp"; \
		echo "Baseline results in: benchmark_baseline.txt"; \
		echo "Current results in: benchmark_current.txt"; \
	fi

# Continuous benchmarking (for development)
watch:
	@echo "Starting continuous benchmarking (Ctrl+C to stop)..."
	@while true; do \
		echo "=== $(shell date) ==="; \
		make bench-streaming; \
		sleep 10; \
	done

# Clean up generated files
clean:
	@echo "Cleaning up generated files..."
	rm -f *.prof *.out benchmark_*.txt
	rm -f /tmp/qf_*test*.log /tmp/qf_*bench*.log
	@echo "Clean complete."

# Display system information for benchmark context
sysinfo:
	@echo "System Information for Benchmarking:"
	@echo "====================================="
	@echo "Go version: $(shell go version)"
	@echo "OS: $(shell uname -s)"
	@echo "Architecture: $(shell uname -m)"
	@echo "CPU cores: $(shell nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 'unknown')"
	@echo "Memory: $(shell free -h 2>/dev/null | grep Mem | awk '{print $$2}' || vm_stat 2>/dev/null | head -1 || echo 'unknown')"
	@echo "Disk space: $(shell df -h . | tail -1 | awk '{print $$4}')"
	@echo ""

# Run full performance suite with system info
bench-full: sysinfo bench validate examples
	@echo ""
	@echo "Performance benchmarking complete!"
	@echo "Check results above for any failures."

# CI-friendly benchmark run
bench-ci:
	@echo "Running CI-friendly performance benchmarks..."
	go test -bench=. -benchmem -benchtime=1s -count=1 -timeout=3m $(PKG) | tee benchmark_ci.txt
	@if grep -q "FAIL" benchmark_ci.txt; then \
		echo "❌ Performance benchmarks failed!"; \
		exit 1; \
	else \
		echo "✅ All performance benchmarks passed!"; \
	fi

# Development helper - run quick benchmarks after code changes
dev-check:
	@echo "Quick development check..."
	make validate-quick
	make bench-streaming BENCH_TIME=2s
	@echo "✅ Development check complete"