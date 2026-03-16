package httpserver

import "sync"

type rateCounter struct {
	mu     sync.Mutex
	counts map[string]int
}

func newRateCounter() *rateCounter {
	return &rateCounter{counts: map[string]int{}}
}

func (r *rateCounter) Increment(key string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts[key]++
	return r.counts[key]
}
