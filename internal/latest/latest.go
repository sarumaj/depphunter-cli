// Package latest runs a function on the newest of the values it is handed, one call at
// a time: the background loaders of the served map (git history, references,
// findings) are re-run after every analysis in watch mode, and a burst of analyses
// needs one follow-up run with the last graph, not one run per graph.
package latest

import "sync"

// Runner runs function one call at a time with the newest value it was given: values
// that arrive during a run are coalesced into a single follow-up run.
type Runner[T any] struct {
	function func(T)
	mu       sync.Mutex
	running  bool
	next     *T
}

// New returns a Runner for function.
func New[T any](function func(T)) *Runner[T] { return &Runner[T]{function: function} }

// Run calls the function with v, or, while a call is already in progress, leaves v
// for that caller's loop to pick up and returns at once.
func (l *Runner[T]) Run(v T) {
	l.mu.Lock()
	l.next = &v
	if l.running {
		l.mu.Unlock()
		return // the running loop picks v up
	}
	l.running = true
	for l.next != nil {
		v := *l.next
		l.next = nil
		l.mu.Unlock()
		l.function(v)
		l.mu.Lock()
	}
	l.running = false
	l.mu.Unlock()
}
