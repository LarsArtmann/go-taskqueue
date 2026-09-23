//go:build !race

package cqrsqlite

// raceDetector reports whether the current binary was built with -race.
const raceDetector = false
