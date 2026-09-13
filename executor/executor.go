// Package executor is the public facade over the pluggable-execution
// module (ADR-0016): the Executor interface, Registry, and the built-in
// executors (sh, HTTP, headless agent, review, status, dlqfix,
// prioritize). The implementation stays internal; this package pins the
// names.
package executor

import (
	internalexecutor "github.com/larsartmann/go-taskqueue/internal/executor"
)

// Executor contract.
type (
	Executor = internalexecutor.Executor
	Func     = internalexecutor.Func
	Registry = internalexecutor.Registry
)

// Built-in executors.
type (
	CommandExecutor    = internalexecutor.CommandExecutor
	HTTPExecutor       = internalexecutor.HTTPExecutor
	AgentExecutor      = internalexecutor.AgentExecutor
	ReviewExecutor     = internalexecutor.ReviewExecutor
	StatusExecutor     = internalexecutor.StatusExecutor
	DLQFixExecutor     = internalexecutor.DLQFixExecutor
	PrioritizeExecutor = internalexecutor.PrioritizeExecutor
)

// Payloads, results, and verdicts.
type (
	AgentPayload      = internalexecutor.AgentPayload
	AgentResult       = internalexecutor.AgentResult
	ReviewPayload     = internalexecutor.ReviewPayload
	ReviewResult      = internalexecutor.ReviewResult
	ReviewVerdict     = internalexecutor.ReviewVerdict
	ReviewFinding     = internalexecutor.ReviewFinding
	StatusPayload     = internalexecutor.StatusPayload
	StatusResult      = internalexecutor.StatusResult
	StatusCompletion  = internalexecutor.StatusCompletion
	DLQFixPayload     = internalexecutor.DLQFixPayload
	DLQFixResult      = internalexecutor.DLQFixResult
	DLQFixVerdict     = internalexecutor.DLQFixVerdict
	PrioritizePayload = internalexecutor.PrioritizePayload
	PrioritizeItem    = internalexecutor.PrioritizeItem
	PrioritizeResult  = internalexecutor.PrioritizeResult
	PrioritizeVerdict = internalexecutor.PrioritizeVerdict
	FailureEvidence   = internalexecutor.FailureEvidence
)

// Typed errors.
type (
	LeaseLostError = internalexecutor.LeaseLostError
	PermanentError = internalexecutor.PermanentError
	PreflightError = internalexecutor.PreflightError
	RateLimitError = internalexecutor.RateLimitError
)

// Error sinks (context-carried failure/result detail).
type Sink = internalexecutor.Sink

// Task-type keys.
const (
	TaskTypeAgent      = internalexecutor.TaskTypeAgent
	TaskTypeReview     = internalexecutor.TaskTypeReview
	TaskTypeStatus     = internalexecutor.TaskTypeStatus
	TaskTypeDLQFix     = internalexecutor.TaskTypeDLQFix
	TaskTypePrioritize = internalexecutor.TaskTypePrioritize
)

// Defaults and knobs.
const (
	DefaultAgentBinary    = internalexecutor.DefaultAgentBinary
	DefaultCloseoutPrompt = internalexecutor.DefaultCloseoutPrompt
	EvidenceTailBytes     = internalexecutor.EvidenceTailBytes
	GoEnvExperiment       = internalexecutor.GoEnvExperiment
)

// Verdicts.
const (
	VerdictApprove        = internalexecutor.VerdictApprove
	VerdictRequestChanges = internalexecutor.VerdictRequestChanges
	VerdictFixed          = internalexecutor.VerdictFixed
	VerdictWontFix        = internalexecutor.VerdictWontFix
)

// Sentinel errors.
var (
	ErrUnknownType                = internalexecutor.ErrUnknownType
	ErrDLQFixSummaryMissing       = internalexecutor.ErrDLQFixSummaryMissing
	ErrPrioritizeEmptyPayload     = internalexecutor.ErrPrioritizeEmptyPayload
	ErrPrioritizeSparsePayload    = internalexecutor.ErrPrioritizeSparsePayload
	ErrPrioritizeUnknownItem      = internalexecutor.ErrPrioritizeUnknownItem
	ErrPrioritizeDuplicateVerdict = internalexecutor.ErrPrioritizeDuplicateVerdict
	ErrPrioritizeMissingVerdict   = internalexecutor.ErrPrioritizeMissingVerdict
	ErrPrioritizeScoreRange       = internalexecutor.ErrPrioritizeScoreRange
)

// Functions.
var (
	NewRegistry           = internalexecutor.NewRegistry
	NewCommandExecutor    = internalexecutor.NewCommandExecutor
	NewHTTPExecutor       = internalexecutor.NewHTTPExecutor
	NewAgentExecutor      = internalexecutor.NewAgentExecutor
	AgentVersion          = internalexecutor.AgentVersion
	CommandFromPayload    = internalexecutor.CommandFromPayload
	DetectRateLimit       = internalexecutor.DetectRateLimit
	DetectVerify          = internalexecutor.DetectVerify
	ExtractResultPayload  = internalexecutor.ExtractResultPayload
	ExtractSessionID      = internalexecutor.ExtractSessionID
	Permanent             = internalexecutor.Permanent
	RateLimited           = internalexecutor.RateLimited
	ReadTQVerify          = internalexecutor.ReadTQVerify
	RenderAgentPayload    = internalexecutor.RenderAgentPayload
	ResultLine            = internalexecutor.ResultLine
	SetFailureEvidence    = internalexecutor.SetFailureEvidence
	SetResultDetail       = internalexecutor.SetResultDetail
	SweepSidecars         = internalexecutor.SweepSidecars
	SweepSidecarsByBytes  = internalexecutor.SweepSidecarsByBytes
	ParseResult           = internalexecutor.ParseResult
	ParseDLQFixResult     = internalexecutor.ParseDLQFixResult
	ParsePrioritizeResult = internalexecutor.ParsePrioritizeResult
	NewSink               = internalexecutor.NewSink
)
