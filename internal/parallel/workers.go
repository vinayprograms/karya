package parallel

import (
	"runtime"
	"sync"
	"time"
)

// TaskType represents the type of processing workload
type TaskType string

const (
	// CPUBound tasks are limited by CPU processing power
	CPUBound TaskType = "cpu-bound"
	// IOBound tasks are limited by I/O operations (network, disk)
	IOBound TaskType = "io-bound"
	// FileProcessing tasks involve mixed I/O and CPU work
	FileProcessing TaskType = "file-processing"
)

// CalculateWorkers determines the optimal number of workers based on
// the number of items to process and the type of workload.
func CalculateWorkers(numItems int, taskType TaskType) int {
	return globalCalculator.calculate(numItems, taskType)
}

// calculator provides adaptive worker scaling with caching
type calculator struct {
	mu              sync.Mutex
	lastCalculation time.Time
	cachedWorkers   int
}

var globalCalculator = &calculator{}

func (c *calculator) calculate(numItems int, taskType TaskType) int {
	if numItems == 0 {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Cache results for 30 seconds to avoid repeated system probing
	if time.Since(c.lastCalculation) < 30*time.Second && c.cachedWorkers > 0 {
		adjusted := c.cachedWorkers
		if numItems < adjusted {
			adjusted = numItems
		}
		return adjusted
	}

	cpuCores := runtime.NumCPU()

	// Get memory statistics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	allocatedMB := float64(m.Alloc) / 1024 / 1024

	// Performance probe
	probeResult := probeSystem()

	var baseWorkers int
	switch taskType {
	case FileProcessing:
		baseWorkers = cpuCores * 2

		// Adjust based on memory usage
		if allocatedMB > 100 {
			baseWorkers = cpuCores
		}

		// Apply probe results
		if probeResult > cpuCores*2 {
			baseWorkers = min(baseWorkers+2, cpuCores*3)
		}

	case CPUBound:
		baseWorkers = cpuCores

	case IOBound:
		baseWorkers = min(cpuCores*4, 50)

	default:
		baseWorkers = cpuCores * 2
	}

	// Bounds checking
	if baseWorkers < 1 {
		baseWorkers = 1
	}
	if baseWorkers > numItems {
		baseWorkers = numItems
	}

	// Cache the result
	c.lastCalculation = time.Now()
	c.cachedWorkers = baseWorkers

	return baseWorkers
}

// probeSystem performs a quick performance probe
func probeSystem() int {
	startTime := time.Now()

	// Simple CPU probe
	iterations := 100000
	sum := 0
	for i := range iterations {
		sum += i * i
	}

	cpuProbeTime := time.Since(startTime)
	baseCores := runtime.NumCPU()

	if cpuProbeTime < 10*time.Millisecond {
		return baseCores * 3
	} else if cpuProbeTime > 50*time.Millisecond {
		return baseCores
	}
	return baseCores * 2
}
