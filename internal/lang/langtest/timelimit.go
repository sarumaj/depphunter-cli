package langtest

import "time"

// TimeLimit scales a wall-clock bound on a plugin's work to the build: the race
// detector slows the code it instruments five to ten times, and CI runs
// `go test -race` on shared runners. The bound keeps guarding a real regression
// (a grammar or a scanner gone quadratic takes far longer than ten times).
func TimeLimit(limit time.Duration) time.Duration {
	return limit * raceFactor
}
