// Package conform holds the shared conformance suite for the three S1
// spike queue backends (sqlitev4, postgresv4, cqrsqlite). Every backend
// runs the SAME tests through a thin harness: the backend owns how a
// fresh isolated database is created (FreshDSN), how a Store attaches to
// a dsn (OpenOn), and how raw seeding SQL reaches the engine (Exec/Begin,
// dialed via Dialect). Capability knobs (Caps) carry the S1 divergences
// the suites previously encoded as per-backend skip edits — a Cap stays
// false until the upstream engine grows the surface (ADR-0019).
//
// The suites are sequential: StoreSuite registers no parallel subtests and
// the harness state (`active`) spans exactly one StoreSuite call.
package conform

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"testing"
	"time"

	"github.com/larsartmann/go-taskqueue/internal/journal"
	"github.com/larsartmann/go-taskqueue/internal/queue"
	"github.com/larsartmann/go-taskqueue/internal/queue/companion"
	"github.com/larsartmann/go-taskqueue/internal/task"
)

// Store is the exact surface the conformance suite exercises: the
// queue.Store contract plus the companion extras (facts reads, watermarks,
// priority scores, questions, archive, project counts). Every spike
// backend satisfies it.
type Store interface {
	AppendFact(ctx context.Context, f journal.Fact) error
	ArchiveFactsBefore(ctx context.Context, cutoff int64) (int64, error)
	ArchiveSummary(ctx context.Context) (companion.ArchiveStats, error)
	Cancel(ctx context.Context, id task.ID, reason string) error
	CancelOwned(ctx context.Context, id task.ID, claim queue.Claim) error
	CancelRequested(ctx context.Context, id task.ID) (bool, error)
	CancelRunning(ctx context.Context, id task.ID, reason string) error
	ClaimDue(ctx context.Context, owner string, lease time.Duration) (task.Task, queue.Claim, error)
	Close() error
	Complete(ctx context.Context, id task.ID, claim queue.Claim, result jsontext.Value) error
	CountFacts(ctx context.Context, ftype journal.FactType, since time.Time) (int64, error)
	CountTasks(ctx context.Context, f queue.Filter) (int, error)
	DeletePriorityScores(ctx context.Context, itemKeys []string) (int64, error)
	DismissDead(ctx context.Context, id task.ID, reason, by string) error
	Enqueue(ctx context.Context, n task.New) (task.Task, error)
	Facts(ctx context.Context, after int64, limit int) ([]journal.Fact, error)
	FactsForTask(ctx context.Context, id string, limit int) ([]journal.Fact, error)
	FactsSince(ctx context.Context, ftype journal.FactType, since time.Time, limit int) ([]journal.Fact, error)
	Fail(ctx context.Context, id task.ID, claim queue.Claim, errText string, backoff time.Duration, evidence jsontext.Value) error
	FailPermanent(ctx context.Context, id task.ID, claim queue.Claim, errText string, evidence jsontext.Value) error
	Get(ctx context.Context, id task.ID) (task.Task, error)
	HeadSeq(ctx context.Context) (int64, error)
	Heartbeat(ctx context.Context, id task.ID, claim queue.Claim, extend time.Duration) error
	LastFacts(ctx context.Context, limit int) ([]journal.Fact, error)
	List(ctx context.Context, f queue.Filter) ([]task.Task, error)
	MarkOrphaned(ctx context.Context, cutoff time.Time) (int, error)
	PriorityScore(ctx context.Context, itemKey string) (queue.PriorityScore, bool, error)
	PriorityScores(ctx context.Context) ([]queue.PriorityScore, error)
	ProjectCounts(ctx context.Context) (map[string]map[task.Status]int, error)
	RecordAnswer(ctx context.Context, id task.ID, ans queue.AnswerRecord) error
	Requeue(ctx context.Context, id task.ID, claim queue.Claim, errText string, delay time.Duration, resumeCloseout bool) error
	RescueDead(ctx context.Context, id task.ID, maxAttempts int) error
	SavePriorityScore(ctx context.Context, score queue.PriorityScore) error
	SaveWatermark(ctx context.Context, consumer string, seq int64) error
	StatusCounts(ctx context.Context) (map[task.Status]int, error)
	UpdatePendingPriority(ctx context.Context, id task.ID, newPriority int, source, reason string) error
	Watermark(ctx context.Context, consumer string) (int64, bool, error)
}

// Caps carries the S1 divergences as capabilities: a false Cap skips the
// matching tests with the divergence reason the backend's suite used to
// encode inline. Flip a Cap to true when the upstream engine grows the
// surface — the test bodies are already the contract pins.
type Caps struct {
	// Exclusivity: store-level per-project exclusivity is implemented
	// (WithProjectExclusivity option exists and is honored).
	Exclusivity bool
	// ResumeCloseout: Requeue carries the resume_closeout evidence key.
	ResumeCloseout bool
	// LegacyMigration: in-place migration from a pre-column/pre-table
	// database file (sqlite-flavored legacy fixtures).
	LegacyMigration bool
	// Archive: the fact archive (facts_archive/journal_meta) exists.
	// False everywhere today — hot-journal-only until upstream grows it.
	Archive bool
	// EnqueuedSnapshot: task.enqueued facts carry the full task snapshot
	// (priority/dedup_key). False everywhere today — upstream enrich,
	// gated on the M4 ratification memo.
	EnqueuedSnapshot bool
}

// Suite is the backend harness: how to get an isolated database, how to
// attach stores to it, and how the suite's direct-SQL seams reach the
// engine. Exec/Begin receive the suite's sqlite-style "?" SQL and apply
// the dialect themselves (identity for sqlite, pgq for postgres).
type Suite struct {
	// FreshDSN returns a fresh, isolated database location for one test
	// (sqlite: temp-file path; postgres: per-test schema-scoped dsn with
	// drop-cascade cleanup already registered).
	FreshDSN func(t *testing.T) string
	// OpenOn attaches a Store to dsn and registers its Close via
	// t.Cleanup. opts carries companion store options (exclusivity).
	OpenOn func(t *testing.T, dsn string, opts ...companion.StoreOption) Store
	// Exec runs direct SQL against the store's engine database
	// (test-only seeding/backdating seam, same dialect as production).
	Exec func(ctx context.Context, s Store, query string, args ...any) (sql.Result, error)
	// Begin opens a transaction on the store's engine database (bulk
	// seeding seam).
	Begin func(ctx context.Context, s Store) (*sql.Tx, error)
	// Dialect rewrites the suite's "?"-placeholder SQL for prepared
	// statements inside Begin transactions. Defaults to companion.SQLite.
	Dialect companion.Dialect
	// Caps selects which divergence-gated tests run (see Caps).
	Caps Caps
}

// active spans exactly one StoreSuite call (the suites are sequential).
var active Suite

// StoreSuite runs the full conformance suite against the backend harness.
func StoreSuite(t *testing.T, s Suite) {
	t.Helper()

	if s.FreshDSN == nil || s.OpenOn == nil || s.Exec == nil || s.Begin == nil {
		t.Fatal("conform: incomplete Suite — FreshDSN, OpenOn, Exec and Begin are required")
	}

	if s.Dialect == nil {
		s.Dialect = companion.SQLite
	}

	prev := active
	active = s
	t.Cleanup(func() { active = prev })

	for _, tt := range conformanceTests {
		t.Run(tt.name, tt.run)
	}
}

func freshDSN(t *testing.T) string {
	t.Helper()

	return active.FreshDSN(t)
}

func openOn(t *testing.T, dsn string, opts ...companion.StoreOption) Store {
	t.Helper()

	return active.OpenOn(t, dsn, opts...)
}

func dbExec(s Store, ctx context.Context, query string, args ...any) (sql.Result, error) {
	return active.Exec(ctx, s, query, args...)
}

func dbBegin(s Store, ctx context.Context) (*sql.Tx, error) {
	return active.Begin(ctx, s)
}

func dial(query string) string {
	return active.Dialect(query)
}

var conformanceTests = []struct {
	name string
	run  func(*testing.T)
}{
	{"TestEnqueueAndClaim", TestEnqueueAndClaim},
	{"TestCompleteVerifiesLease", TestCompleteVerifiesLease},
	{"TestCompleteResetsLastError", TestCompleteResetsLastError},
	{"TestFailRetriesThenDeadLetters", TestFailRetriesThenDeadLetters},
	{"TestDismissDead", TestDismissDead},
	{"TestLeaseExpiryAllowsReclaim", TestLeaseExpiryAllowsReclaim},
	{"TestDepsBlockUntilCompleted", TestDepsBlockUntilCompleted},
	{"TestPriorityOrdersClaims", TestPriorityOrdersClaims},
	{"TestClaimAgingFlipsOrder", TestClaimAgingFlipsOrder},
	{"TestClaimAgingBonusCapped", TestClaimAgingBonusCapped},
	{"TestClaimAgingRespectsNotBefore", TestClaimAgingRespectsNotBefore},
	{"TestClaimAgingKeyedOnCreatedAtAcrossRequeue", TestClaimAgingKeyedOnCreatedAtAcrossRequeue},
	{"TestClaimAgingAccruesPerWindow", TestClaimAgingAccruesPerWindow},
	{"TestUpdatePendingPriority", TestUpdatePendingPriority},
	{"TestUpdatePendingPriorityIdempotent", TestUpdatePendingPriorityIdempotent},
	{"TestUpdatePendingPriorityRefusesNonPending", TestUpdatePendingPriorityRefusesNonPending},
	{"TestUpdatePendingPriorityFlipsClaimOrder", TestUpdatePendingPriorityFlipsClaimOrder},
	{"TestNotBeforeDelays", TestNotBeforeDelays},
	{"TestHeartbeatExtendsLease", TestHeartbeatExtendsLease},
	{"TestCancelPendingOnly", TestCancelPendingOnly},
	{"TestListFilters", TestListFilters},
	{"TestGetNotFound", TestGetNotFound},
	{"TestEmptyTypeRejected", TestEmptyTypeRejected},
	{"TestEnqueueDedupKey", TestEnqueueDedupKey},
	{"TestEnqueueWithoutDedupKeyIndependent", TestEnqueueWithoutDedupKeyIndependent},
	{"TestMigrateAddsDedupKeyToOldDatabase", TestMigrateAddsDedupKeyToOldDatabase},
	{"TestWatermarkAbsentReturnsZero", TestWatermarkAbsentReturnsZero},
	{"TestWatermarkSaveAndReadRoundtrip", TestWatermarkSaveAndReadRoundtrip},
	{"TestWatermarkMonotonicGuard", TestWatermarkMonotonicGuard},
	{"TestMigrateAddsWatermarksTable", TestMigrateAddsWatermarksTable},
	{"TestFailPermanentDeadLettersImmediately", TestFailPermanentDeadLettersImmediately},
	{"TestProjectExclusivitySerializesPerProject", TestProjectExclusivitySerializesPerProject},
	{"TestProjectExclusivityAcrossStoreHandles", TestProjectExclusivityAcrossStoreHandles},
	{"TestParkedRequeueNotResurrectableByStaleLease", TestParkedRequeueNotResurrectableByStaleLease},
	{"TestParkedFilter", TestParkedFilter},
	{"TestRequeueDoesNotBurnAttempts", TestRequeueDoesNotBurnAttempts},
	{"TestRequeueFactCarriesResumeCloseout", TestRequeueFactCarriesResumeCloseout},
	{"TestFactsCursorBounded", TestFactsCursorBounded},
	{"TestLastFactsReturnsTailInAscendingOrder", TestLastFactsReturnsTailInAscendingOrder},
	{"TestHeadSeq", TestHeadSeq},
	{"TestFactsForTaskFiltersAndBounds", TestFactsForTaskFiltersAndBounds},
	{"TestCountFactsByTypeSince", TestCountFactsByTypeSince},
	{"TestFactsSinceByType", TestFactsSinceByType},
	{"TestListQueryPushdown", TestListQueryPushdown},
	{"TestListQueryLikeEscaping", TestListQueryLikeEscaping},
	{"TestListOffsetPagination", TestListOffsetPagination},
	{"TestStatusCountsAndProjectCounts", TestStatusCountsAndProjectCounts},
	{"TestListSeverityOrder", TestListSeverityOrder},
	{"TestCountTasksMatchesList", TestCountTasksMatchesList},
	{"TestLoadSnapshotScaleAt100k", TestLoadSnapshotScaleAt100k},
	{"TestCancelRunningRequestAndHonour", TestCancelRunningRequestAndHonour},
	{"TestReclaimFinalizesCancelRequest", TestReclaimFinalizesCancelRequest},
	{"TestCancelReasonStoredInFactDetail", TestCancelReasonStoredInFactDetail},
	{"TestMarkOrphanedRecordsStrandedTasks", TestMarkOrphanedRecordsStrandedTasks},
	{"TestEnqueueClaimBaseline10k", TestEnqueueClaimBaseline10k},
	{"TestArchiveFactsBeforeKeepsProjections", TestArchiveFactsBeforeKeepsProjections},
	{"TestListSortAllowlist", TestListSortAllowlist},
	{"TestListSinceFilter", TestListSinceFilter},
	{"TestPriorityScoreRoundtrip", TestPriorityScoreRoundtrip},
	{"TestPriorityScoresListAndDelete", TestPriorityScoresListAndDelete},
	{"TestBandFilter", TestBandFilter},
	{"TestEnqueueFactDetailCarriesIdentity", TestEnqueueFactDetailCarriesIdentity},
	{"TestRescueDeadEmitsRescueEnqueue", TestRescueDeadEmitsRescueEnqueue},
	{"TestRecordAnswerUnblocksParkedTask", TestRecordAnswerUnblocksParkedTask},
	{"TestRecordAnswerReplayIsNoop", TestRecordAnswerReplayIsNoop},
	{"TestRecordAnswerSecondQuestionAppends", TestRecordAnswerSecondQuestionAppends},
	{"TestRecordAnswerOnRunningTaskFactOnly", TestRecordAnswerOnRunningTaskFactOnly},
	{"TestRecordAnswerRawPayloadFactOnly", TestRecordAnswerRawPayloadFactOnly},
	{"TestRecordAnswerValidation", TestRecordAnswerValidation},
	{"TestAppendFactRecordsNonTaskFact", TestAppendFactRecordsNonTaskFact},
	{"TestAppendFactDoesNotMaterializeTaskRows", TestAppendFactDoesNotMaterializeTaskRows},
}
