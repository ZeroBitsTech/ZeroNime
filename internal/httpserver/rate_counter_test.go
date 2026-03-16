package httpserver

import (
	"fmt"
	"sync"
	"testing"
)

func TestRateCounterIncrementIsConcurrentSafe(t *testing.T) {
	t.Parallel()

	counter := newRateCounter()
	const total = 200

	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter.Increment("127.0.0.1:minute")
		}()
	}
	wg.Wait()

	if got := counter.Increment("127.0.0.1:minute"); got != total+1 {
		t.Fatalf("Increment() = %d, want %d", got, total+1)
	}
}

func TestRateCounterSeparatesKeys(t *testing.T) {
	t.Parallel()

	counter := newRateCounter()
	for index := 0; index < 5; index++ {
		key := fmt.Sprintf("127.0.0.%d", index)
		if got := counter.Increment(key); got != 1 {
			t.Fatalf("first increment for %s = %d, want 1", key, got)
		}
	}
}
