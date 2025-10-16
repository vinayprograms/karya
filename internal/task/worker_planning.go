package task

import (
	"runtime"
	"time"
)

// calculateOptimalWorkers determines the optimal number of worker threads
// based on system resources and workload characteristics
func calculateOptimalWorkers(numFiles int, taskType string) int {
	// Get system info
	cpuCores := runtime.NumCPU()

	var maxWorkers int

	switch taskType {
	case "cpu-bound":
		// For CPU-intensive tasks, limit to CPU cores
		maxWorkers = cpuCores

	case "io-bound":
		// For I/O-bound tasks, can use more workers
		// Rule of thumb: 2-4x CPU cores for I/O bound tasks
		maxWorkers = cpuCores * 3

		// Cap at reasonable limits to avoid resource exhaustion
		if maxWorkers > 50 {
			maxWorkers = 50
		}

	case "file-processing":
		// For file processing (mixed I/O and CPU), use a balanced approach
		maxWorkers = cpuCores * 2

		// Dynamic adjustment based on available memory estimation
		// Each worker might use ~1-2MB for file processing
		estimatedMemoryPerWorker := 2 * 1024 * 1024 // 2MB

		// Get memory stats (simplified heuristic)
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// Use only a fraction of available memory for workers
		availableForWorkers := (512 * 1024 * 1024) // Conservative 512MB limit
		memoryBasedLimit := availableForWorkers / estimatedMemoryPerWorker

		if memoryBasedLimit < maxWorkers {
			maxWorkers = memoryBasedLimit
		}

		// Cap between reasonable bounds
		if maxWorkers > 25 {
			maxWorkers = 25
		}

	default:
		// Default conservative approach
		maxWorkers = cpuCores
	}

	// Never use more workers than files to process
	if numFiles < maxWorkers {
		maxWorkers = numFiles
	}

	// Minimum of 1 worker
	if maxWorkers < 1 {
		maxWorkers = 1
	}

	return maxWorkers
}

// probeSystemCapabilities performs a quick performance probe to adjust worker count
func probeSystemCapabilities() int {
	startTime := time.Now()

	// Simple CPU probe: measure time to perform basic operations
	iterations := 100000
	sum := 0
	for i := 0; i < iterations; i++ {
		sum += i * i
	}

	cpuProbeTime := time.Since(startTime)

	// Heuristic: if system is fast (< 10ms), can handle more workers
	// if system is slow (> 50ms), reduce workers
	baseCores := runtime.NumCPU()

	if cpuProbeTime < 10*time.Millisecond {
		// Fast system - can handle 3x cores for I/O tasks
		return baseCores * 3
	} else if cpuProbeTime > 50*time.Millisecond {
		// Slower system - be conservative
		return baseCores
	} else {
		// Average system - 2x cores
		return baseCores * 2
	}
}

// adaptiveWorkerCalculator provides dynamic worker scaling with system monitoring
type adaptiveWorkerCalculator struct {
	lastCalculation time.Time
	cachedWorkers   int
	systemLoad      float64
}

var globalWorkerCalculator = &adaptiveWorkerCalculator{}

// calculateAdaptiveWorkers performs intelligent worker calculation with caching and load monitoring
func calculateAdaptiveWorkers(numFiles int, taskType string) int {
	calc := globalWorkerCalculator

	// Cache results for 30 seconds to avoid repeated system probing
	if time.Since(calc.lastCalculation) < 30*time.Second && calc.cachedWorkers > 0 {
		// Adjust cached value based on current workload
		adjusted := calc.cachedWorkers
		if numFiles < adjusted {
			adjusted = numFiles
		}
		return adjusted
	}

	// Perform new calculation
	cpuCores := runtime.NumCPU()

	// Get memory statistics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Calculate available memory (simplified)
	allocatedMB := float64(m.Alloc) / 1024 / 1024

	// Performance probe
	probeResult := probeSystemCapabilities()

	// Base calculation on task type
	var baseWorkers int
	switch taskType {
	case "file-processing":
		baseWorkers = cpuCores * 2

		// Adjust based on memory usage
		if allocatedMB > 100 { // If using > 100MB, be more conservative
			baseWorkers = cpuCores
		}

		// Apply probe results
		if probeResult > cpuCores*2 {
			baseWorkers = min(baseWorkers+2, cpuCores*3)
		}

	case "cpu-bound":
		baseWorkers = cpuCores

	case "io-bound":
		baseWorkers = min(cpuCores*4, 50)

	default:
		baseWorkers = cpuCores * 2
	}

	// Final bounds checking
	if baseWorkers < 1 {
		baseWorkers = 1
	}
	if baseWorkers > numFiles {
		baseWorkers = numFiles
	}

	// Cache the result
	calc.lastCalculation = time.Now()
	calc.cachedWorkers = baseWorkers

	return baseWorkers
}

// min helper function since Go doesn't have a built-in min for int
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
