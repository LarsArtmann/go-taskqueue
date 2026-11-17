package executor

import (
	"fmt"
)

// NetworkOutageCode marks connectivity-loss deaths in errors and journal
// facts: an agent run whose provider (or any upstream host) was
// unreachable — DNS resolution failure, no route — while the machine's
// network was down or degraded. The session the run opened is alive
// server-side (the provider call failed, not the conversation); these
// deaths park the task WITHOUT burning an attempt so the retry resumes
// the SAME crush session instead of minting a fresh one per attempt (the
// DNS-outage session spam, observed 2026-11-17: 24+ sessions in minutes
// across dead tasks' attempts and their autopsies).
const NetworkOutageCode = "network"

// networkOutageMarkers are the observed connectivity-loss signatures.
// Deliberately NARROW, evidence-first (the strings below are the exact
// resolver/routing errors observed in the 2026-11-17 outage facts and
// crush session records):
//   - "temporary failure in name resolution" — glibc/musl EAI_AGAIN
//     (observed: `dial tcp: lookup api.z.ai: Temporary failure in name
//     resolution` riding a Bad Gateway finish in the session).
//   - "no such host" — the resolver answered ENOTFOUND for a host that
//     exists (DNS servers unreachable while cache expired).
//   - "network is unreachable" / "no route to host" — routing loss.
//
// "connection refused" is deliberately NOT a marker: a refused TCP port
// usually names a LOCAL service being down (the llamacpp discovery probe
// against 127.0.0.1), not an internet outage — park-without-burn would
// then hide a provider that is simply gone. Revisit with live evidence
// only.
var networkOutageMarkers = []string{
	"temporary failure in name resolution",
	"no such host",
	"network is unreachable",
	"no route to host",
}

// DetectNetworkOutage reports whether a run's output or error text
// carries the connectivity-loss signature. Both surfaces are scanned:
// the provider error often only rides the OUTPUT (crush exits non-zero
// with the resolver error in the tail) and the wrapped error carries the
// excerpted tail on the rc-0 paths.
func DetectNetworkOutage(output, errText string) bool {
	for _, marker := range networkOutageMarkers {
		if containsFold(output, marker) || containsFold(errText, marker) {
			return true
		}
	}

	return false
}

// NetworkOutageError is the typed outcome for an agent turn that died on
// connectivity loss: the worker requeues it WITHOUT burning an attempt
// (RequeueClassNetwork) on the escalating preflight ladder, exempt from
// the env-streak breaker — the identical retry fails identically until
// the network returns, and the executor's session-retry ladder resumes
// the SAME crush session on the next claim (no fresh session per
// attempt). ResumeCloseout marks a close-out turn severed by the outage:
// the work turn already succeeded, so the re-claim resumes at close-out.
type NetworkOutageError struct {
	Cause error
	// ResumeCloseout marks an outage-severed CLOSE-OUT turn: the work
	// turn succeeded and its session is alive; the re-claim resumes at
	// close-out instead of re-paying the work turn (same contract as
	// RateLimitError.ResumeCloseout).
	ResumeCloseout bool
}

func (e *NetworkOutageError) Error() string {
	cause := "<run lost connectivity>"
	if e.Cause != nil {
		cause = e.Cause.Error()
	}

	return fmt.Sprintf(
		"network unreachable during the agent run [%s]: %s — the task parks until the network returns; the retry resumes the same session without attempt burn",
		NetworkOutageCode,
		cause,
	)
}

func (e *NetworkOutageError) Unwrap() error { return e.Cause }
