//go:build !race

package queue

// raceDetector reports whether the current binary was built with -race.
// The race detector's ~10x slowdown makes latency assertions meaningless,
// so the scale test skips itself under race.
const raceDetector = false
