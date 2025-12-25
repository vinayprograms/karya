package parallel

import (
	"sync/atomic"
	"testing"
)

func TestProcess(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	results := Process(items, func(n int) *int {
		doubled := n * 2
		return &doubled
	})

	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}

	// Check all values are doubled (order not guaranteed)
	sum := 0
	for _, r := range results {
		sum += *r
	}
	if sum != 30 { // 2+4+6+8+10 = 30
		t.Errorf("expected sum 30, got %d", sum)
	}
}

func TestProcess_NilResults(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	results := Process(items, func(n int) *int {
		if n%2 == 0 {
			return nil // Skip odd
		}
		return &n
	})

	if len(results) != 3 {
		t.Errorf("expected 3 results (1,3,5), got %d", len(results))
	}
}

func TestProcess_Empty(t *testing.T) {
	var items []int

	results := Process(items, func(n int) *int {
		return &n
	})

	if results != nil {
		t.Errorf("expected nil for empty input, got %v", results)
	}
}

func TestProcessWithErrors(t *testing.T) {
	items := []string{"a", "bb", "ccc"}

	results := ProcessWithErrors(items, func(s string) (*int, error) {
		l := len(s)
		return &l, nil
	})

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestCollect(t *testing.T) {
	items := []int{1, 2, 3, 4, 5}

	results := Collect(items, func(n int) (int, bool) {
		if n > 2 {
			return n * 10, true
		}
		return 0, false
	})

	if len(results) != 3 {
		t.Errorf("expected 3 results (30,40,50), got %d", len(results))
	}
}

func TestCalculateWorkers(t *testing.T) {
	tests := []struct {
		name     string
		numItems int
		taskType TaskType
		minWant  int
		maxWant  int
	}{
		{"zero items", 0, FileProcessing, 0, 0},
		{"one item", 1, FileProcessing, 1, 1},
		{"few items", 3, FileProcessing, 1, 3},
		{"many items file", 100, FileProcessing, 1, 100},
		{"many items cpu", 100, CPUBound, 1, 100},
		{"many items io", 100, IOBound, 1, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateWorkers(tt.numItems, tt.taskType)
			if got < tt.minWant || got > tt.maxWant {
				t.Errorf("CalculateWorkers(%d, %s) = %d, want between %d and %d",
					tt.numItems, tt.taskType, got, tt.minWant, tt.maxWant)
			}
		})
	}
}

func TestProcess_Concurrency(t *testing.T) {
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}

	var counter atomic.Int32

	results := Process(items, func(n int) *int {
		counter.Add(1)
		return &n
	})

	if len(results) != 100 {
		t.Errorf("expected 100 results, got %d", len(results))
	}

	if counter.Load() != 100 {
		t.Errorf("expected 100 items processed, got %d", counter.Load())
	}
}
