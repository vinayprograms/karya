// Package parallel provides generic parallel processing utilities
// for file and data processing workloads.
package parallel

import (
	"sync"
)

// ProcessFunc is a function that processes an item and returns a result.
// It should handle its own errors internally and return nil for failed items.
type ProcessFunc[T any, R any] func(item T) R

// Process executes the given function on each item in parallel using a worker pool.
// It returns all non-nil results. The order of results is not guaranteed.
func Process[T any, R any](items []T, fn ProcessFunc[T, *R]) []*R {
	if len(items) == 0 {
		return nil
	}

	numWorkers := CalculateWorkers(len(items), FileProcessing)

	jobs := make(chan T, len(items))
	results := make(chan *R, len(items))
	var wg sync.WaitGroup

	// Start workers
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				if result := fn(item); result != nil {
					results <- result
				}
			}
		}()
	}

	// Send jobs
	go func() {
		for _, item := range items {
			jobs <- item
		}
		close(jobs)
	}()

	// Close results when workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var collected []*R
	for result := range results {
		collected = append(collected, result)
	}

	return collected
}

// ProcessWithError is similar to Process but allows the processing function
// to return an error. Items that error are skipped in the results.
type ProcessWithErrorFunc[T any, R any] func(item T) (*R, error)

// ProcessWithErrors executes the given function on each item in parallel.
// Items that return errors are skipped. Returns all successful results.
func ProcessWithErrors[T any, R any](items []T, fn ProcessWithErrorFunc[T, R]) []*R {
	if len(items) == 0 {
		return nil
	}

	numWorkers := CalculateWorkers(len(items), FileProcessing)

	jobs := make(chan T, len(items))
	results := make(chan *R, len(items))
	var wg sync.WaitGroup

	// Start workers
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				if result, err := fn(item); err == nil && result != nil {
					results <- result
				}
			}
		}()
	}

	// Send jobs
	go func() {
		for _, item := range items {
			jobs <- item
		}
		close(jobs)
	}()

	// Close results when workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var collected []*R
	for result := range results {
		collected = append(collected, result)
	}

	return collected
}

// Collect is a convenience function that processes items and collects
// results into a slice using a simple transform function.
func Collect[T any, R any](items []T, fn func(item T) (R, bool)) []R {
	if len(items) == 0 {
		return nil
	}

	numWorkers := CalculateWorkers(len(items), FileProcessing)

	type result struct {
		value R
		ok    bool
	}

	jobs := make(chan T, len(items))
	results := make(chan result, len(items))
	var wg sync.WaitGroup

	// Start workers
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				val, ok := fn(item)
				results <- result{value: val, ok: ok}
			}
		}()
	}

	// Send jobs
	go func() {
		for _, item := range items {
			jobs <- item
		}
		close(jobs)
	}()

	// Close results when workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var collected []R
	for r := range results {
		if r.ok {
			collected = append(collected, r.value)
		}
	}

	return collected
}
