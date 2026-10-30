#!/usr/bin/env bash
# ONE pin source for tool versions consumed by gates and CI (the
# GOSEC_VERSION pattern). Bash consumers source this file; ci.yml derives
# its install lines with sed (no bash source needed on the runner).
# Bump a version HERE and nowhere else — a second literal is the drift
# this file exists to kill (2026-10-15 parity-sweep §e5: golangci-lint
# v2.13.2 was pasted into two ci.yml steps).
#
# Format contract (pinned by ci-local's lint-pin step): lines of
# NAME=value with no spaces, values matching [A-Za-z0-9._-]+ — ci.yml
# sed-derives every value from here (the check-gosec.sh --self-test evals
# the workflow's exact sed lines against this file).
# shellcheck disable=SC2034  # consumed by check-gosec.sh and sed-derived by ci.yml
GOSEC_VERSION=v2.29.0
# shellcheck disable=SC2034
GOSEC_EXCLUDES=-exclude=G104,G115,G118,G124,G202,G204,G301,G302,G304,G306,G404,G702,G703,G710
# shellcheck disable=SC2034
GOLANGCI_LINT_VERSION=v2.13.2
# shellcheck disable=SC2034  # consumed by ci.yml's erraudit step
ERRAUDIT_VERSION=v0.7.0
