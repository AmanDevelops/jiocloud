package parallel

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRunAllSucceed(t *testing.T) {
	items := []int{1, 2, 3, 4, 5, 6, 7, 8}
	var sum int64
	err := Run(items, 4, func(n int) error {
		atomic.AddInt64(&sum, int64(n))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if sum != 36 {
		t.Errorf("sum = %d, want 36", sum)
	}
}

func TestRunRespectsWorkerLimit(t *testing.T) {
	var (
		mu       sync.Mutex
		inFlight int
		maxSeen  int
	)
	items := make([]int, 50)
	err := Run(items, 3, func(int) error {
		mu.Lock()
		inFlight++
		if inFlight > maxSeen {
			maxSeen = inFlight
		}
		mu.Unlock()
		// brief spin to encourage overlap
		for i := 0; i < 10000; i++ {
		}
		mu.Lock()
		inFlight--
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if maxSeen > 3 {
		t.Errorf("max concurrency = %d, want <= 3", maxSeen)
	}
}

func TestRunReturnsFirstError(t *testing.T) {
	sentinel := errors.New("boom")
	err := Run([]int{1, 2, 3}, 2, func(n int) error {
		if n == 2 {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want %v", err, sentinel)
	}
}
