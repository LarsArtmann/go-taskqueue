package executor

import (
	"errors"
)

// FailureClass is the retry taxonomy for one failed attempt: WHY the run
// failed, in the vocabulary the retry ladder already acts on. The worker
// stamps it onto the failure evidence (and DLQ autopsies read it back) so
// forensics never re-derive the class from the output tail (paperclip's
// provider-failure classification, M12).
type FailureClass string

const (
	// FailureClassTransient: the identical retry may succeed (flake,
	// contention, kernel ETXTBSY). A plain retry with backoff — the
	// store's default failure treatment — is already the right ladder.
	FailureClassTransient FailureClass = "transient"
	// FailureClassPermanent: the identical retry fails identically (bad
	// payload, missing repo, an introduced verify failure). Dead-letter
	// material; a retry only burns money.
	FailureClassPermanent FailureClass = "permanent"
	// FailureClassProviderWindow: the model provider behind the run
	// refused (429 / usage window); the identical retry fails until the
	// window resets. Requeue WITHOUT burning an attempt, parked on the
	// parsed window.
	FailureClassProviderWindow FailureClass = "provider-window"
	// FailureClassEnvironment: the task's environment refused to run or
	// to prove the run (dirty tree, missing autonomy, dead or slow verify
	// gate). Requeue WITHOUT burning an attempt on the escalation ladder.
	FailureClassEnvironment FailureClass = "environment"
)

// ClassifyFailure maps a failed run's error onto the retry taxonomy. It
// lives here (not inline in the worker) so the taxonomy has ONE definition,
// but it is CALLED by the worker: only the worker sees the final error
// wrapper chain (a verify gate wraps a plain failure and changes its
// class). An unrecognized error classifies transient — a plain retry is
// already the default treatment, so a wrong guess degrades to today's
// behavior, never worse.
func ClassifyFailure(err error) FailureClass {
	if _, ok := errors.AsType[*RateLimitError](err); ok {
		return FailureClassProviderWindow
	}

	if _, ok := errors.AsType[*PermanentError](err); ok {
		return FailureClassPermanent
	}

	if _, ok := errors.AsType[*PreflightError](err); ok {
		return FailureClassEnvironment
	}

	if _, ok := errors.AsType[*VerifyGateError](err); ok {
		return FailureClassEnvironment
	}

	return FailureClassTransient
}
