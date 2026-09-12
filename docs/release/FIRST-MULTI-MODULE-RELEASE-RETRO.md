# First multi-module release retrospective — SCAFFOLD

Status: TEMPLATE. Fill this in after the next REAL release (v0.2.0 was the
round-2 flow's first live cut; this retro is for the first release cut
AFTER this scaffold landed). Trigger: the moment `--push` completes, the
release owner fills every `<fill>` below and drops the SCAFFOLD banner.
Source row: TODO_LIST "first multi-module release" (23:47 f50).

## What the gates caught (and what they missed)

<fill: for each gate in scripts/release.sh phase order — preconditions,
CHANGELOG, flake version sync, gate_gomod, ci-local, webui smoke — did it
fire on the real release? Anything the release survived WITHOUT a gate
covering it is a gap to route into TODO_LIST.>

## Time and friction

<fill: wall-clock per phase; where the flow made the owner wait; whether
the 30-minute CI poll cap was enough; proxy wait attempts used.>

## Module-tag reality vs the doc

<fill: did every `internal/<mod>/vX.Y.Z` cut cleanly (disk-derived list)?
Any module where require-bump and sub-tag got out of sync before the
release tag? Did the clean-room module-tag verification (RELEASE.md
"Sub-tag cutting") run, and how long did it take? Any `go get`-passes-
`go list`-fails surprise?>

## Queue↔git attribution during the release window

<fill: were release-window commits carrying `Task-Queue-ID` footers
attributed correctly (multi-commits per ID are the norm)? Any
`commitsForTask` note-degradations (missing repo, git log failure) that
hid real attribution?>

## Decisions to keep / overturn

<fill: anything RELEASE.md now documents that the live release proved
wrong, and anything done ad hoc that should become a gate. Cite the
CHANGELOG section of the release.>
