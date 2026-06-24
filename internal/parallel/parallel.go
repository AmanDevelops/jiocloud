// Package parallel provides a tiny bounded-concurrency helper used to run
// independent transfers (uploads/downloads) over a fixed pool of workers.
package parallel

import "sync"

// Run applies fn to each item using at most `workers` concurrent goroutines.
// It returns the first error encountered; once any task fails, no further tasks
// are scheduled (in-flight tasks still run to completion). With workers <= 1 it
// runs sequentially.
func Run[T any](items []T, workers int, fn func(T) error) error {
	if workers < 1 {
		workers = 1
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)
	sem := make(chan struct{}, workers)

	for _, it := range items {
		mu.Lock()
		stop := firstErr != nil
		mu.Unlock()
		if stop {
			break
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(it T) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := fn(it); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(it)
	}

	wg.Wait()
	return firstErr
}
