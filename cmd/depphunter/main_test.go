package main

import (
	"sync"
	"testing"
	"time"
)

func TestLatestCoalesces(t *testing.T) {
	var mu sync.Mutex
	var seen []int
	release := make(chan struct{})
	l := newLatest(func(v int) {
		if v == 1 {
			<-release // hold the first run while more values arrive
		}
		mu.Lock()
		seen = append(seen, v)
		mu.Unlock()
	})
	done := make(chan struct{})
	go func() { l.Run(1); close(done) }()
	time.Sleep(20 * time.Millisecond)
	for v := 2; v <= 5; v++ {
		l.Run(v) // returns at once: a run is in progress
	}
	close(release)
	<-done
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 5 {
		t.Errorf("runs %v, want [1 5]", seen)
	}
}
