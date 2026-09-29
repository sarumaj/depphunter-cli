//go:build !race

package langtest

// raceFactor is 1 without the race detector: the bounds hold as written.
const raceFactor = 1
