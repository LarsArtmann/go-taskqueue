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
	DepBumpExecutor    = internalexecutor.DepBumpExecutor
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
	DepBump           = internalexecutor.DepBump
	DepBumpRelease    = internalexecutor.DepBumpRelease
	DepBumpPayload    = internalexecutor.DepBumpPayload
	FailureEvidence   = internalexecutor.FailureEvidence
)

// Typed errors.
type (
	LeaseLostError       = internalexecutor.LeaseLostError
	PermanentError       = internalexecutor.PermanentError
	PreflightError       = internalexecutor.PreflightError
	QuestionPendingError = internalexecutor.QuestionPendingError
	RateLimitError       = internalexecutor.RateLimitError
	VerifyGateError      = internalexecutor.VerifyGateError
	VerifyGateClass      = internalexecutor.VerifyGateClass
)

// Verify-gate failure classes (see VerifyGateError).
const (
	VerifyGateDead          = internalexecutor.VerifyGateDead
	VerifyGateSlow          = internalexecutor.VerifyGateSlow
	VerifyGateEnvironmental = internalexecutor.VerifyGateEnvironmental
	VerifyGateEnvCode       = internalexecutor.VerifyGateEnvCode
)

// IsGateArtifactDeath classifies a stored verify failure as the
// unscoped-gofmt artifact (the vendor-gofmt death class).
var IsGateArtifactDeath = internalexecutor.IsGateArtifactDeath

// Verify-gate death classes stamped into FailureEvidence.VerifyStage (see
// verifyDeathStage).
const (
	VerifyStageGofmt      = internalexecutor.VerifyStageGofmt
	VerifyStageTest       = internalexecutor.VerifyStageTest
	VerifyStageE2ETimeout = internalexecutor.VerifyStageE2ETimeout
	VerifyStageRun        = internalexecutor.VerifyStageRun
)

// Retry-failure taxonomy (see FailureEvidence.Class): why a run failed,
// stamped by the worker so autopsies and forensics never re-derive it
// from output tails.
type FailureClass = internalexecutor.FailureClass

const (
	FailureClassTransient      = internalexecutor.FailureClassTransient
	FailureClassPermanent      = internalexecutor.FailureClassPermanent
	FailureClassProviderWindow = internalexecutor.FailureClassProviderWindow
	FailureClassEnvironment    = internalexecutor.FailureClassEnvironment
)

// ClassifyFailure maps a failed run's error onto the retry taxonomy.
var ClassifyFailure = internalexecutor.ClassifyFailure

// Sink carries failure/result detail through the context (error and
// result sinks).
type Sink = internalexecutor.Sink

// Commit-attribution scanner (gitscan.go): which commits carry a git
// footer `key: value`.
type (
	Commit         = internalexecutor.Commit
	GitScanner     = internalexecutor.GitScanner
	GitScannerFunc = internalexecutor.GitScannerFunc
	GitLogScanner  = internalexecutor.GitLogScanner
)

// Task-type keys.
const (
	TaskTypeAgent      = internalexecutor.TaskTypeAgent
	TaskTypeReview     = internalexecutor.TaskTypeReview
	TaskTypeStatus     = internalexecutor.TaskTypeStatus
	TaskTypeDLQFix     = internalexecutor.TaskTypeDLQFix
	TaskTypePrioritize = internalexecutor.TaskTypePrioritize
	TaskTypeDepBump    = internalexecutor.TaskTypeDepBump
)

// Defaults and knobs.
const (
	DefaultAgentBinary    = internalexecutor.DefaultAgentBinary
	DefaultCloseoutPrompt = internalexecutor.DefaultCloseoutPrompt
	EvidenceTailBytes     = internalexecutor.EvidenceTailBytes
	GoEnvExperiment       = internalexecutor.GoEnvExperiment
	RedactMarker          = internalexecutor.RedactMarker
	ScopedGofmtStage      = internalexecutor.ScopedGofmtStage
	TaskTrailer           = internalexecutor.TaskTrailer
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
	ErrDepBumpEmptyPayload        = internalexecutor.ErrDepBumpEmptyPayload
	ErrDepBumpNoWork              = internalexecutor.ErrDepBumpNoWork
	ErrDepBumpBadVersion          = internalexecutor.ErrDepBumpBadVersion
)

// Functions.
var (
	NewRegistry              = internalexecutor.NewRegistry
	NewCommandExecutor       = internalexecutor.NewCommandExecutor
	NewHTTPExecutor          = internalexecutor.NewHTTPExecutor
	NewAgentExecutor         = internalexecutor.NewAgentExecutor
	AgentVersion             = internalexecutor.AgentVersion
	CommandFromPayload       = internalexecutor.CommandFromPayload
	DetectRateLimit          = internalexecutor.DetectRateLimit
	DetectVerify             = internalexecutor.DetectVerify
	Excerpt                  = internalexecutor.Excerpt
	ExtractResultPayload     = internalexecutor.ExtractResultPayload
	ExtractSessionID         = internalexecutor.ExtractSessionID
	Permanent                = internalexecutor.Permanent
	QuestionPending          = internalexecutor.QuestionPending
	RateLimited              = internalexecutor.RateLimited
	ReadTQVerify             = internalexecutor.ReadTQVerify
	RenderAgentPayload       = internalexecutor.RenderAgentPayload
	ResultLine               = internalexecutor.ResultLine
	RedactSecrets            = internalexecutor.RedactSecrets
	SecretHits               = internalexecutor.SecretHits
	SetFailureEvidence       = internalexecutor.SetFailureEvidence
	SetVerifyFailureEvidence = internalexecutor.SetVerifyFailureEvidence
	SetResultDetail          = internalexecutor.SetResultDetail
	StaleVerifyReasons       = internalexecutor.StaleVerifyReasons
	SweepSidecars            = internalexecutor.SweepSidecars
	SweepSidecarsByBytes     = internalexecutor.SweepSidecarsByBytes
	ParseResult              = internalexecutor.ParseResult
	ParseDLQFixResult        = internalexecutor.ParseDLQFixResult
	ParsePrioritizeResult    = internalexecutor.ParsePrioritizeResult
	NewSink                  = internalexecutor.NewSink
	NewDepBumpExecutor       = internalexecutor.NewDepBumpExecutor
	IsStableSemver           = internalexecutor.IsStableSemver
)
