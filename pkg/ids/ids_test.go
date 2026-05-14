package ids

import (
	"sync"
	"testing"
)

func TestNewOrderID_isValidULID(t *testing.T) {
	id := NewOrderID()
	if len(id) != 26 {
		t.Fatalf("expected 26 chars, got %d: %q", len(id), id)
	}
	if !ValidateOrderID(id) {
		t.Fatalf("NewOrderID produced invalid ULID: %q", id)
	}
}

func TestNewOrderID_isUnique(t *testing.T) {
	const n = 1000
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		id := NewOrderID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID generated: %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestNewOrderID_isMonotonic(t *testing.T) {
	prev := NewOrderID()
	for i := 0; i < 100; i++ {
		next := NewOrderID()
		if next < prev {
			t.Fatalf("IDs not monotonically increasing: %q >= %q", prev, next)
		}
		prev = next
	}
}

func TestNewOrderID_concurrentSafe(t *testing.T) {
	const goroutines = 50
	const perGoroutine = 100

	results := make(chan string, goroutines*perGoroutine)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				results <- NewOrderID()
			}
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[string]struct{}, goroutines*perGoroutine)
	for id := range results {
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID under concurrency: %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestValidateOrderID_acceptsValidULID(t *testing.T) {
	id := NewOrderID()
	if !ValidateOrderID(id) {
		t.Fatalf("expected valid, got invalid for %q", id)
	}
}

func TestValidateOrderID_rejectsInvalidStrings(t *testing.T) {
	cases := []string{
		"",
		"not-a-ulid",
		"01ARZ3NDEKTSV4RRFFQ69G5FA",  // 25 chars (too short)
		"01ARZ3NDEKTSV4RRFFQ69G5FAVX", // 27 chars (too long)
		"01ARZ3NDEKTSV4RRFFQ69G5FAU",  // valid length but invalid char (U)
	}
	for _, tc := range cases {
		if ValidateOrderID(tc) {
			t.Errorf("expected invalid, got valid for %q", tc)
		}
	}
}
