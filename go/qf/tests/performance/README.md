# Performance Benchmarks for qf

This directory contains comprehensive performance benchmarks for the qf interactive log filter composer, focusing on regex caching, pattern compilation, and filtering throughput.

## Performance Targets

The benchmarks validate these performance targets:
- **Cache Hit Rate**: >80% for typical usage patterns
- **Pattern Compilation**: <20ms average compilation time
- **Filtering Throughput**: >100K lines/sec for simple patterns

## Benchmark Categories

### 1. PatternManager LRU Cache Benchmarks (`cache_test.go`)

#### Cache Performance
- `BenchmarkPatternManagerCacheHit`: Tests cache hit performance with pre-populated cache
- `BenchmarkPatternManagerCacheMiss`: Tests compilation performance on cache misses
- `BenchmarkPatternManagerConcurrentAccess`: Tests thread safety and concurrent performance
- `BenchmarkPatternManagerLRUEviction`: Tests LRU eviction performance

#### Pattern Compilation
- `BenchmarkPatternCompilation`: Tests regex compilation speed for simple and complex patterns
- `BenchmarkPatternValidation`: Tests Pattern.Validate() performance

#### Filtering Throughput
- `BenchmarkFilteringThroughputSimple`: Tests filtering performance with simple patterns
- `BenchmarkFilteringThroughputComplex`: Tests filtering performance with complex regex patterns
- `BenchmarkFilteringWithMixedPatterns`: Tests mixed include/exclude pattern scenarios

#### Cache Efficiency Metrics
- `BenchmarkCacheEfficiencyMetrics`: Tests comprehensive cache performance with realistic access patterns
- `BenchmarkCacheMemoryPressure`: Tests cache behavior under memory constraints

#### Stress Testing
- `BenchmarkHighVolumeFiltering`: Tests performance with large datasets (100K lines)

### 2. Test Function
- `TestCachePerformanceTargets`: Validates that performance targets are met

## Running the Benchmarks

### Run All Cache Benchmarks
```bash
go test -bench=. -benchmem ./tests/performance/
```

### Run Specific Benchmark Categories
```bash
# Cache hit performance
go test -bench=BenchmarkPatternManagerCache -benchmem ./tests/performance/

# Pattern compilation performance
go test -bench=BenchmarkPatternCompilation -benchmem ./tests/performance/

# Filtering throughput
go test -bench=BenchmarkFilteringThroughput -benchmem ./tests/performance/

# Cache efficiency
go test -bench=BenchmarkCacheEfficiency -benchmem ./tests/performance/
```

### Run Performance Target Validation
```bash
go test -run=TestCachePerformanceTargets -v ./tests/performance/
```

### Advanced Benchmarking

#### Extended Benchmark Time
```bash
go test -bench=BenchmarkHighVolumeFiltering -benchtime=5s ./tests/performance/
```

#### CPU Profiling
```bash
go test -bench=BenchmarkFilteringThroughputSimple -cpuprofile=cpu.prof ./tests/performance/
go tool pprof cpu.prof
```

#### Memory Profiling
```bash
go test -bench=BenchmarkPatternManagerCache -memprofile=mem.prof ./tests/performance/
go tool pprof mem.prof
```

## Understanding Benchmark Output

### Metrics Reported
- **hit_rate_%**: Cache hit rate as percentage
- **cache_size**: Current number of cached patterns
- **lines_per_sec**: Filtering throughput in lines per second
- **processing_time_ms**: Total processing time in milliseconds
- **avg_compilation_ms**: Average pattern compilation time

### Sample Output
```
BenchmarkPatternManagerCacheHit-10       493683    241.9 ns/op    40.00 cache_size    100.0 hit_rate_%    0 B/op    0 allocs/op
```

This shows:
- 493,683 operations completed
- 241.9 nanoseconds per operation
- 40 patterns in cache
- 100% cache hit rate
- 0 bytes allocated per operation
- 0 allocations per operation

## Test Data Generation

The benchmarks use realistic test data:

### Patterns
- Common log levels (ERROR, WARN, INFO, DEBUG)
- Date/time patterns
- IP addresses and email addresses
- HTTP status codes and API endpoints
- UUID and duration patterns

### Log Lines
- Realistic log message templates
- Varied timestamps, user IDs, and error messages
- Representative data sizes and content distribution

## Performance Optimization Tips

Based on benchmark results:

1. **Cache Size**: Tune cache size based on pattern diversity
2. **Pattern Complexity**: Simple patterns perform 2-3x faster than complex regex
3. **Concurrency**: PatternManager scales well with concurrent access
4. **Memory**: LRU eviction keeps memory usage bounded

## Continuous Performance Monitoring

Add these benchmarks to CI/CD pipeline:
```bash
# Performance regression testing
go test -bench=. -benchmem ./tests/performance/ > bench_results.txt
# Compare with previous results using benchcmp or similar tools
```