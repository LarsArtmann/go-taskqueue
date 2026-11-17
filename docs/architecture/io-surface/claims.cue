// claims.cue — human judgment. Tooling never overwrites this file.
//
// Filled 2026-11-15 (io-surface TODO #140, judgment hour): every site carries
// resource + trigger; file-write sites carry durability; sites whose misbehavior
// breaks the queue contract carry criticality "critical". Rationales mark
// decided postures (why this durability/criticality is right), not narration.
package io_surface

sites: {
	"cmd/tq.appendRSSSample": claims: {
		resource: "RSS samples log (<unix-milli> <kb> lines, append-only)"
		trigger: "each pool sampler tick"
		durability: "ephemeral"
		rationale: "evidence logging; every failure is silent by design and a lost tail costs one sample"
	}
	"cmd/tq.applyPoolConfigFile": claims: {
		resource: "pool config env backing (TQ_* variables)"
		trigger: "pool config resolution when a key is neither flagged nor file-set"
	}
	"cmd/tq.applyPoolHealthConfigDefault": claims: {
		resource: "pool config file existence (~/.config/tq/pool.conf)"
		trigger: "pool-health default resolution at command start"
	}
	"cmd/tq.askDBPath": claims: {
		resource: "TQ_DB / TQ_ASK_DB env (ask journal path)"
		trigger: "tq ask db resolution"
	}
	"cmd/tq.bootstrapOptions.ensureCrushConfig": claims: {
		resource: "repo .crushrc (managed block rewrite)"
		trigger: "bootstrap when autonomy is requested"
		durability: "recoverable"
		rationale: "byte-identical short-circuit; the file is operator-owned config the bootstrap only extends inside its managed block"
	}
	"cmd/tq.bootstrapOptions.ensureRepos": claims: {
		resource: "repo .git directory presence"
		trigger: "bootstrap repo sanity pass"
	}
	"cmd/tq.bootstrapOptions.ensureTQVerify": claims: {
		resource: "repo .tq-verify pin file"
		trigger: "bootstrap when a verify command is detected or overridden"
		durability: "recoverable"
	}
	"cmd/tq.bootstrapOptions.installService": claims: {
		resource: "~/.config/tq/pool.conf, ~/.config/systemd/user/tq-agent-pool.service, systemctl --user units"
		trigger: "explicit bootstrap --install"
		durability: "durable"
		criticality: "critical"
		rationale: "the deployed service definition is the pool's whole control plane; a torn write here breaks the next daemon restart"
	}
	"cmd/tq.bootstrapOptions.resolveRepos": claims: {
		resource: "projects dir and repo checkout paths"
		trigger: "bootstrap spec expansion"
	}
	"cmd/tq.buildAgentConveniencePayload": claims: {
		resource: "prompt file contents (--prompt-file)"
		trigger: "agent convenience payload assembly when a prompt file is given"
	}
	"cmd/tq.cmdAPI": claims: {
		resource: "TCP API listener (default 127.0.0.1:8091, bearer-token auth)"
		trigger: "tq api command lifetime"
	}
	"cmd/tq.cmdAsk": claims: {
		resource: "task question file (TQ_QUESTION_FILE)"
		trigger: "tq ask recording a question/answer forward"
		durability: "recoverable"
	}
	"cmd/tq.cmdCalibration": claims: {
		resource: "TQ_LOG_DIR env"
		trigger: "tq calibration flag resolution"
	}
	"cmd/tq.cmdEnqueue": claims: {
		resource: "payload file contents (--payload-file) and TQ_* defaults"
		trigger: "tq enqueue flag resolution"
	}
	"cmd/tq.cmdHarvest": claims: {
		resource: "TQ_PROJECTS_DIR default"
		trigger: "tq harvest flag resolution"
	}
	"cmd/tq.cmdPoolHealth": claims: {
		resource: "TQ_POOL_CONFIG path default"
		trigger: "tq pool-health flag resolution"
	}
	"cmd/tq.cmdServe": claims: {
		resource: "TQ_* serve defaults"
		trigger: "tq serve flag resolution"
	}
	"cmd/tq.cmdShow": claims: {
		resource: "TQ_DB env fallback in error guidance"
		trigger: "tq show failure path when no explicit db"
	}
	"cmd/tq.cmdVerdict": claims: {
		resource: "task result file (TQ_RESULT_FILE or --file)"
		trigger: "tq verdict recording a verdict document"
		durability: "durable"
		criticality: "critical"
		rationale: "the verdict file is the queue↔task evidence contract; losing it fails the task's closeout"
	}
	"cmd/tq.cmdWorker": claims: {
		resource: "TQ_* worker defaults and per-worker env threading"
		trigger: "tq worker flag resolution and agent env inheritance"
	}
	"cmd/tq.commitsForTask": claims: {
		resource: "task repo git log (footer scan)"
		trigger: "task fold: cross-referencing landed commits against the task footer"
	}
	"cmd/tq.crushKeeper": claims: {
		resource: "crush server unix-socket HTTP (workspace liveness)"
		trigger: "each keeper tick while the pool holds agent slots"
	}
	"cmd/tq.crushRun": claims: {
		resource: "crush binary subprocess (CRUSH_SESSION_ID env)"
		trigger: "tq crush wrapper invocation"
	}
	"cmd/tq.crushServerRSSLogPath": claims: {
		resource: "TQ_DB env (RSS log path derivation)"
		trigger: "pool startup RSS-log placement"
	}
	"cmd/tq.defaultDB": claims: {
		resource: "TQ_DB env"
		trigger: "every command's default database path resolution"
	}
	"cmd/tq.defaultProjectsDir": claims: {
		resource: "TQ_PROJECTS_DIR env (else ~/projects)"
		trigger: "harvest/worker default projects dir resolution"
	}
	"cmd/tq.doctorCrushManagedBlock": claims: {
		resource: "repo .crushrc contents"
		trigger: "tq doctor crush-pin check"
	}
	"cmd/tq.doctorEnvironment": claims: {
		resource: "crush/git/go binaries on PATH"
		trigger: "tq doctor environment check"
	}
	"cmd/tq.doctorProjection": claims: {
		resource: "projection db file presence"
		trigger: "tq doctor projection check"
	}
	"cmd/tq.doctorProjectionDLQ": claims: {
		resource: "legacy + deployment DLQ file presence"
		trigger: "tq doctor DLQ layout check"
	}
	"cmd/tq.doctorRepoAutonomy": claims: {
		resource: "repo TODO_LIST.md and .crushrc presence"
		trigger: "tq doctor repo autonomy check"
	}
	"cmd/tq.doctorRepoCoverage": claims: {
		resource: "configured project dirs existence"
		trigger: "tq doctor coverage check"
	}
	"cmd/tq.doctorSQLiteChecks": claims: {
		resource: "task SQLite db (PRAGMA quick_check, journal mode)"
		trigger: "tq doctor database check"
	}
	"cmd/tq.doctorTagAncestry": claims: {
		resource: "git rev-parse in the target repo"
		trigger: "tq doctor tag-ancestry check"
	}
	"cmd/tq.doctorTreeGofmt": claims: {
		resource: "gofmt -l over the target tree"
		trigger: "tq doctor gofmt check"
	}
	"cmd/tq.doctorVerdictChannel": claims: {
		resource: "tq binary on PATH"
		trigger: "tq doctor verdict-channel check"
	}
	"cmd/tq.doctorVerifyPins": claims: {
		resource: "repo .tq-verify pin presence"
		trigger: "tq doctor verify-pin check"
	}
	"cmd/tq.ensureCommitted": claims: {
		resource: "task repo git index (git add/commit)"
		trigger: "task closeout: committing the agent's work before verdict"
	}
	"cmd/tq.executableFile": claims: {
		resource: "verify hook file metadata"
		trigger: "verify resolution: honoring an executable override file"
	}
	"cmd/tq.expandRepoSpecs": claims: {
		resource: "projects dir and repo paths"
		trigger: "repo spec expansion for delegated runs"
	}
	"cmd/tq.foreignTaskRefs": claims: {
		resource: "git log of the daemon fold repo"
		trigger: "fold scan: separating this task's commits from foreign footer commits"
	}
	"cmd/tq.gitChangedFiles": claims: {
		resource: "task repo git diff"
		trigger: "verify scoping: files touched by the task's commits"
	}
	"cmd/tq.gitChildCommit": claims: {
		resource: "task repo git log adjacency"
		trigger: "fold scan: attributing child commits to footer work"
	}
	"cmd/tq.gitCommitMeta": claims: {
		resource: "task repo git commit metadata"
		trigger: "fold scan: commit subject/author extraction"
	}
	"cmd/tq.goEnvBuild": claims: {
		resource: "go build ./... in the target module"
		trigger: "go env doctor probe"
	}
	"cmd/tq.loadPoolConfigFile": claims: {
		resource: "pool config file"
		trigger: "pool config load at worker/pool start"
	}
	"cmd/tq.parkBy": claims: {
		resource: "USER env (park-by identity)"
		trigger: "tq park default author resolution"
	}
	"cmd/tq.parseAgentPoolOptions": claims: {
		resource: "TQ_DISCOVERY_ADDR/TQ_* agent-pool env"
		trigger: "agent-pool flag resolution"
	}
	"cmd/tq.parseSystemdUnitEnv": claims: {
		resource: "systemd unit Environment= lines"
		trigger: "doctor: reading the deployed unit's env for drift"
	}
	"cmd/tq.printCQRSEvents": claims: {
		resource: "journal fact stream (stdout render)"
		trigger: "tq events print"
	}
	"cmd/tq.procRSSKB": claims: {
		resource: "/proc/<pid>/status VmRSS"
		trigger: "each RSS sample of a pool-managed process"
	}
	"cmd/tq.readCrushServerRSS": claims: {
		resource: "crush server RSS log file"
		trigger: "each keeper RSS tick"
	}
	"cmd/tq.repoDirty": claims: {
		resource: "git status --porcelain of the task repo"
		trigger: "pre-claim and pre-verify tree cleanliness gates"
	}
	"cmd/tq.resolveGofmt": claims: {
		resource: "gofmt binary on PATH"
		trigger: "doctor/verify gofmt resolution"
	}
	"cmd/tq.runCrushServerActor": claims: {
		resource: "agent runtime dir + crush server child process"
		trigger: "pool actor lifecycle: spawn, supervise, restart"
		durability: "ephemeral"
		rationale: "runtime scaffolding is recreated by the actor on every restart"
	}
	"cmd/tq.runDLQ": claims: {
		resource: "TQ_* DLQ defaults"
		trigger: "tq dlq flag resolution"
	}
	"cmd/tq.runGateBoard": claims: {
		resource: "repo scripts/ dir and gate board subprocess"
		trigger: "tq gate-board delegation when scripts exist"
	}
	"cmd/tq.runGoEnvProbe": claims: {
		resource: "scratch go.mod probe file in a temp dir"
		trigger: "go env probe bootstrap"
		durability: "ephemeral"
		rationale: "written to a temp probe dir and removed with it"
	}
	"cmd/tq.sessionBegin": claims: {
		resource: "CRUSH_SESSION_ID env and session registry"
		trigger: "tq session begin"
	}
	"cmd/tq.sessionClose": claims: {
		resource: "CRUSH_SESSION_ID env and session registry"
		trigger: "tq session close"
	}
	"cmd/tq.sessionPing": claims: {
		resource: "CRUSH_SESSION_ID env and session registry"
		trigger: "tq session ping"
	}
	"internal/bridge/cqa.Bridge.Collect": claims: {
		resource: "configured repo directory metadata"
		trigger: "cqa bridge collect when the repo dir resolves"
	}
	"internal/bridge/cqa.Bridge.getJSON": claims: {
		resource: "cqa API (BaseURL, GET JSON)"
		trigger: "each collect poll"
	}
	"internal/bridge/papdashboard.AnswerPoller.list": claims: {
		resource: "PapDashboard API answers endpoint"
		trigger: "each answer poll tick"
	}
	"internal/bridge/papdashboard.HealthRelay.handle": claims: {
		resource: "relay request body (health snapshot JSON, 1 MiB cap)"
		trigger: "each pool health POST to the relay"
	}
	"internal/bridge/papdashboard.NewHealthRelay": claims: {
		resource: "loopback TCP listener (127.0.0.1:0)"
		trigger: "health relay construction at pool start"
	}
	"internal/bridge/papdashboard.ingester.post": claims: {
		resource: "PapDashboard API ingest endpoint"
		trigger: "each alert/health push"
	}
	"internal/budget.Guard.runBudgetCmd": claims: {
		resource: "configured budget command via sh -c"
		trigger: "budget guard evaluation"
	}
	"internal/config.SyncPolicyFromEnv": claims: {
		resource: "TQ_SYNC_POLICY env"
		trigger: "config load"
	}
	"internal/config.parseSyncPolicy": claims: {
		resource: "TQ_SYNC_POLICY env string"
		trigger: "sync policy parse"
	}
	"internal/depsweep.DepgraphSource.Plan": claims: {
		resource: "depgraph binary subprocess"
		trigger: "each dep-sweep plan"
	}
	"internal/docsgates.binaryRunner": claims: {
		resource: "docs-gates binary subprocess (run --json)"
		trigger: "each docs-gate verification"
	}
	"internal/executor.AgentExecutor.Execute": claims: {
		resource: "agent payload/verify file metadata"
		trigger: "agent task execute preconditions"
	}
	"internal/executor.AgentExecutor.binary": claims: {
		resource: "agent binary env resolution"
		trigger: "agent executor build"
	}
	"internal/executor.AgentExecutor.runAgentTurn": claims: {
		resource: "agent stdin/stdout temp files + child process env (per turn)"
		trigger: "each agent task turn"
		durability: "ephemeral"
		rationale: "temp files are created and removed within the turn; loss costs nothing"
	}
	"internal/executor.AgentExecutor.runCloseoutTurn": claims: {
		resource: "closeout turn child process (question rail payload)"
		trigger: "agent task closeout turn"
	}
	"internal/executor.AgentVersion": claims: {
		resource: "agent binary on PATH (--version probe)"
		trigger: "agent version detection"
	}
	"internal/executor.CommandExecutor.Execute": claims: {
		resource: "task command via sh -c (line-limited)"
		trigger: "each sh-type task execution"
	}
	"internal/executor.DepBumpExecutor.Execute": claims: {
		resource: "target repo .git metadata"
		trigger: "dep-bump task precondition"
	}
	"internal/executor.DepBumpExecutor.commit": claims: {
		resource: "target repo paths to commit"
		trigger: "dep-bump commit step"
	}
	"internal/executor.DepBumpExecutor.quarantineGoWork": claims: {
		resource: "go.work file in the target repo"
		trigger: "dep-bump when a go.work must be quarantined"
	}
	"internal/executor.DepBumpExecutor.regenerateTempl": claims: {
		resource: "templ binary + repo *.templ walk"
		trigger: "dep-bump after go.mod/template changes"
	}
	"internal/executor.DepBumpExecutor.run": claims: {
		resource: "configured bump subprocess (env-inherited)"
		trigger: "each dep-bump tool invocation"
	}
	"internal/executor.DepBumpExecutor.syncGomod": claims: {
		resource: "buildflow binary on PATH"
		trigger: "dep-bump go.mod sync when buildflow is available"
	}
	"internal/executor.DepBumpExecutor.verify": claims: {
		resource: "verify scratch dir under os.TempDir"
		trigger: "dep-bump verification"
		durability: "ephemeral"
		rationale: "whole verify tree is a temp dir removed after the run"
	}
	"internal/executor.GitLogScanner.CommitsByTrailer": claims: {
		resource: "repo git log by trailer"
		trigger: "harvest footer scan"
	}
	"internal/executor.HTTPExecutor.Execute": claims: {
		resource: "configured HTTP webhook (POST body)"
		trigger: "each http-type task execution"
	}
	"internal/executor.HTTPExecutor.rateLimited": claims: {
		resource: "webhook response body (8 KiB cap)"
		trigger: "each webhook response read"
	}
	"internal/executor.ReadVerdict": claims: {
		resource: "task verdict file"
		trigger: "verdict read at task fold"
	}
	"internal/executor.RecordCalibrationFailure": claims: {
		resource: "calibration.jsonl (append under TQ_LOG_DIR)"
		trigger: "each recorded task failure kind"
		durability: "recoverable"
		rationale: "calibration evidence is re-derivable from journal facts; silent-drop by design"
	}
	"internal/executor.ScanWorktreeOrphans": claims: {
		resource: "worktree root directory listing"
		trigger: "sidecar sweep scan"
	}
	"internal/executor.SummarizeCalibration": claims: {
		resource: "calibration.jsonl (read)"
		trigger: "calibration summary reads"
	}
	"internal/executor.SweepSidecars": claims: {
		resource: "orphan sidecar directory (os.Remove)"
		trigger: "sidecar sweep of one orphan"
		durability: "ephemeral"
		rationale: "sidecars are run-scoped scratch; removal is the point"
	}
	"internal/executor.SweepSidecarsByBytes": claims: {
		resource: "orphan sidecar directory over byte budget (os.Remove)"
		trigger: "byte-budget sidecar sweep"
		durability: "ephemeral"
	}
	"internal/executor.TrimCalibrationDataset": claims: {
		resource: "calibration.jsonl (trim: read, temp write, rename)"
		trigger: "dataset over byte budget"
		durability: "recoverable"
		rationale: "temp+rename swap keeps the dataset whole; loss costs calibration evidence, not queue state"
	}
	"internal/executor.acquireAgentSlot": claims: {
		resource: "agent slot dir (0o1777) + slot lock file"
		trigger: "each agent task claiming a concurrency slot"
		durability: "ephemeral"
		rationale: "slot locks are lease-scoped; stale slots are reclaimed by the sweeper"
	}
	"internal/executor.agentSlotDir": claims: {
		resource: "TQ_AGENT_SLOT_DIR env (else temp)"
		trigger: "slot dir resolution"
	}
	"internal/executor.assertCleanTree": claims: {
		resource: "task repo git status"
		trigger: "verify precondition: tree must be clean"
	}
	"internal/executor.autoDetectVerify": claims: {
		resource: "repo go.mod discovery inside the verify shell"
		trigger: "verify auto-detection"
	}
	"internal/executor.baselineVerify": claims: {
		resource: "baseline verify scratch tree + cached evidence dir"
		trigger: "verify when the baseline contract holds"
		durability: "ephemeral"
	}
	"internal/executor.buildflowCovered": claims: {
		resource: "repo .buildflow.yml presence"
		trigger: "verify routing: buildflow-covered repos take the baseline contract"
	}
	"internal/executor.checkGitVersion": claims: {
		resource: "repo git --version"
		trigger: "verify git capability probe"
	}
	"internal/executor.checkWorktreeRails": claims: {
		resource: "repo rail files (.buildflow.yml & co)"
		trigger: "worktree guard rails check"
	}
	"internal/executor.collectSidecars": claims: {
		resource: "sidecar root directory listing"
		trigger: "sidecar sweep collection"
	}
	"internal/executor.createWorktree": claims: {
		resource: "worktree path (stale remove + git worktree add)"
		trigger: "each agent task worktree creation"
		durability: "ephemeral"
		rationale: "worktrees are task-scoped and removed at closeout; loss costs the task's scratch only"
	}
	"internal/executor.crushServerClient.getJSON": claims: {
		resource: "crush server unix HTTP (GET)"
		trigger: "agent supervision reads"
	}
	"internal/executor.crushServerClient.postJSON": claims: {
		resource: "crush server unix HTTP (POST)"
		trigger: "agent control commands (interrupt/closeout)"
	}
	"internal/executor.defaultVerify": claims: {
		resource: "repo default verify command metadata"
		trigger: "verify resolution: repo default path"
	}
	"internal/executor.filesInCommits": claims: {
		resource: "repo git show --name-only for task commits"
		trigger: "verify scoping: commit file lists"
	}
	"internal/executor.gitRun": claims: {
		resource: "task repo git subprocess"
		trigger: "executor git plumbing"
	}
	"internal/executor.gofmtFlagged": claims: {
		resource: "gofmt -l over the task repo"
		trigger: "verify: gofmt drift probe"
	}
	"internal/executor.newCrushServerClient": claims: {
		resource: "crush server unix socket dial"
		trigger: "crush client construction"
	}
	"internal/executor.prepareRepo": claims: {
		resource: "repo .git metadata"
		trigger: "task precondition: repo is a checkout"
	}
	"internal/executor.questionPendingFrom": claims: {
		resource: "task question file"
		trigger: "closeout: reading the pending question rail"
	}
	"internal/executor.randomClientID": claims: {
		resource: "crypto/rand"
		trigger: "crush client id generation"
	}
	"internal/executor.readTQVerify": claims: {
		resource: "repo .tq-verify pin file"
		trigger: "verify resolution at claim time"
	}
	"internal/executor.recordCalibration": claims: {
		resource: "calibration.jsonl dataset (append)"
		trigger: "each calibrated task completion"
		durability: "recoverable"
		rationale: "calibration data is re-derivable from journal facts; append-only loss costs one datapoint"
	}
	"internal/executor.redactActive": claims: {
		resource: "TQ_REDACT env"
		trigger: "log redaction decision"
	}
	"internal/executor.removeWorktree": claims: {
		resource: "task worktree dir (os.RemoveAll)"
		trigger: "task closeout worktree teardown"
		durability: "ephemeral"
	}
	"internal/executor.requireRepoAutonomy": claims: {
		resource: "repo autonomy markers (TODO_LIST, .crushrc, crush.json)"
		trigger: "agent task autonomy precondition"
	}
	"internal/executor.requireReportFile": claims: {
		resource: "repo closeout report file"
		trigger: "closeout: report presence gate"
	}
	"internal/executor.resolveRepoDir": claims: {
		resource: "repo path metadata"
		trigger: "repo dir resolution for payloads"
	}
	"internal/executor.retrySession": claims: {
		resource: "retry sidecar file + TQ_* env"
		trigger: "agent session retry with saved session id"
	}
	"internal/executor.runVerify": claims: {
		resource: "repo verify subprocess (.tq-verify / default / auto-detect)"
		trigger: "each task verify"
	}
	"internal/executor.vendorGofmtSignature": claims: {
		resource: "vendor dir + gofmt state signature"
		trigger: "verify: vendor/fmt stage gate"
	}
	"internal/executor.writeFailureSidecar": claims: {
		resource: "redacted failure sidecar under TQ_LOG_DIR (session-resume ladder evidence)"
		trigger: "network-outage agent run failure"
		durability: "recoverable"
		rationale: "best-effort redacted evidence; absent TQ_LOG_DIR or a write error degrades to the fresh-run fallback — loss costs the resume ladder and the diagnosis trail, never task state"
	}
	"internal/executor.writeOutputSidecar": claims: {
		resource: "task output sidecar dir (TQ sidecar root)"
		trigger: "each finished task writing its output sidecar"
		durability: "recoverable"
	}
	"internal/executor.writeVerifyEvidence": claims: {
		resource: "verify evidence dir under TQ_LOG_DIR"
		trigger: "each verify writing its evidence"
		durability: "recoverable"
	}
	"internal/harvest.DiscoverRepos": claims: {
		resource: "projects dir repo listing (TODO_LIST.md markers)"
		trigger: "harvest discovery pass"
	}
	"internal/harvest.DiscoverReposDaemon": claims: {
		resource: "discovery daemon HTTP (TQ_DISCOVERY_ADDR)"
		trigger: "daemon-mode harvest discovery"
	}
	"internal/harvest.EffortCache.Load": claims: {
		resource: "effort cache JSON"
		trigger: "harvest effort lookup"
	}
	"internal/harvest.EffortCache.Save": claims: {
		resource: "effort cache JSON"
		trigger: "harvest effort update"
		durability: "ephemeral"
		rationale: "cache: recomputed from the source repos on loss"
	}
	"internal/harvest.Harvester.donePreflightRepoDir": claims: {
		resource: "repo directory metadata"
		trigger: "done-preflight before enqueueing work"
	}
	"internal/harvest.ParseRepoAll": claims: {
		resource: "repo TODO_LIST.md"
		trigger: "harvest parse of a repo's task list"
	}
	"internal/harvest.ReadImportance": claims: {
		resource: "repo .config/metadata.yaml"
		trigger: "harvest importance weighting"
	}
	"internal/harvest.RepoPurpose": claims: {
		resource: "repo .config/metadata.yaml"
		trigger: "harvest purpose extraction"
	}
	"internal/harvest.Watcher.streamOnce": claims: {
		resource: "watch URL HTTP stream (SSE/long-poll)"
		trigger: "each watcher stream cycle"
	}
	"internal/harvest.claimLive": claims: {
		resource: ".tq/claimed/<id> marker"
		trigger: "harvest claim liveness check"
	}
	"internal/harvest.closeoutReportExists": claims: {
		resource: "repo closeout report glob"
		trigger: "done-preflight closeout presence"
	}
	"internal/harvest.gitOutputs": claims: {
		resource: "repo git subprocess output"
		trigger: "harvest git evidence collection"
	}
	"internal/harvest.gitSucceeds": claims: {
		resource: "repo git subprocess exit status"
		trigger: "harvest git capability probes"
	}
	"internal/harvest.isDangling": claims: {
		resource: "git merge-base --is-ancestor in the repo"
		trigger: "harvest dangling-candidate check"
	}
	"internal/harvest.jevClient.ask": claims: {
		resource: "System One (jev) HTTP API /v1/systemone"
		trigger: "each harvest importance question"
	}
	"internal/harvest.mapDaemonProjects": claims: {
		resource: "repo TODO_LIST.md metadata"
		trigger: "daemon project mapping"
	}
	"internal/harvest.unixSocketClient": claims: {
		resource: "discovery unix socket dial"
		trigger: "harvest daemon transport setup"
	}
	"internal/harvest.verdictBlock": claims: {
		resource: ".tq/verdicts/<id> file"
		trigger: "harvest verdict-block scan"
	}
	"internal/httpapi.Server.ListenAndServe": claims: {
		resource: "HTTP API listener"
		trigger: "tq serve lifetime"
	}
	"internal/httpapi.Server.handleEnqueue": claims: {
		resource: "enqueue request body (1 MiB cap)"
		trigger: "each API enqueue POST"
	}
	"internal/httpapi.Server.handleReportError": claims: {
		resource: "error-report request body (1 MiB cap)"
		trigger: "each API error report POST"
	}
	"internal/queue/companion.ClaimCounts": claims: {
		resource: "companion SQLite (claim counts)"
		trigger: "dashboard/status reads"
	}
	"internal/queue/companion.ClaimDue": claims: {
		resource: "companion SQLite (due claim CAS)"
		trigger: "each worker claim attempt"
		criticality: "critical"
		rationale: "the claim CAS is the lease contract; a wrong write double-runs tasks"
	}
	"internal/queue/companion.CountFacts": claims: {
		resource: "companion SQLite (fact count)"
		trigger: "status reads"
	}
	"internal/queue/companion.CountTasks": claims: {
		resource: "companion SQLite (task count by status)"
		trigger: "status reads"
	}
	"internal/queue/companion.DeletePriorityScores": claims: {
		resource: "companion SQLite (priority scores)"
		trigger: "priority invalidation"
	}
	"internal/queue/companion.FactsSince": claims: {
		resource: "companion SQLite (facts after seq)"
		trigger: "projection catch-up reads"
	}
	"internal/queue/companion.HeadSeq": claims: {
		resource: "companion SQLite (max fact seq)"
		trigger: "projection head check"
	}
	"internal/queue/companion.LastFacts": claims: {
		resource: "companion SQLite (last N facts)"
		trigger: "tail reads"
	}
	"internal/queue/companion.List": claims: {
		resource: "companion SQLite (task list query)"
		trigger: "list reads"
	}
	"internal/queue/companion.ListWatermarks": claims: {
		resource: "companion SQLite (consumer watermarks)"
		trigger: "watermark reads"
	}
	"internal/queue/companion.Migrate": claims: {
		resource: "companion SQLite schema (CompanionSchema DDL)"
		trigger: "companion open when schema is absent"
		criticality: "critical"
		rationale: "schema bootstrap must be atomic or the read model is corrupt from birth"
	}
	"internal/queue/companion.Park": claims: {
		resource: "companion SQLite (park CAS)"
		trigger: "tq park / auto-park"
		criticality: "critical"
	}
	"internal/queue/companion.PriorityScore": claims: {
		resource: "companion SQLite (one priority score)"
		trigger: "priority lookups"
	}
	"internal/queue/companion.PriorityScores": claims: {
		resource: "companion SQLite (priority scores)"
		trigger: "priority listings"
	}
	"internal/queue/companion.ProjectCounts": claims: {
		resource: "companion SQLite (project/status counts)"
		trigger: "dashboard reads"
	}
	"internal/queue/companion.RecordAnswer": claims: {
		resource: "companion SQLite (question answer upsert)"
		trigger: "each recorded answer"
		criticality: "critical"
		rationale: "answers gate question-forward dedup; a lost write re-asks agents"
	}
	"internal/queue/companion.Requeue": claims: {
		resource: "companion SQLite (requeue CAS)"
		trigger: "lease expiry / explicit requeue"
		criticality: "critical"
	}
	"internal/queue/companion.SavePriorityScore": claims: {
		resource: "companion SQLite (priority score upsert)"
		trigger: "scorer writes"
	}
	"internal/queue/companion.SaveWatermark": claims: {
		resource: "companion SQLite (watermark upsert)"
		trigger: "projection checkpoint"
		criticality: "critical"
		rationale: "watermark loss replays or skips facts"
	}
	"internal/queue/companion.SetWatermark": claims: {
		resource: "companion SQLite (watermark set)"
		trigger: "projection checkpoint set"
		criticality: "critical"
	}
	"internal/queue/companion.StatusCounts": claims: {
		resource: "companion SQLite (status group counts)"
		trigger: "status reads"
	}
	"internal/queue/companion.TokenFor": claims: {
		resource: "companion SQLite (API token lookup)"
		trigger: "API auth"
	}
	"internal/queue/companion.Watermark": claims: {
		resource: "companion SQLite (one watermark)"
		trigger: "projection resume"
	}
	"internal/queue/companion.WithTx": claims: {
		resource: "companion SQLite transaction"
		trigger: "multi-statement companion writes"
		criticality: "critical"
	}
	"internal/queue/companion.appendFact": claims: {
		resource: "companion SQLite (facts append)"
		trigger: "each journaled fact"
		criticality: "critical"
		rationale: "the fact log is the system of record"
	}
	"internal/queue/companion.cancelRequestedReasonTx": claims: {
		resource: "companion SQLite (cancel reasons)"
		trigger: "cancel scan in tx"
	}
	"internal/queue/companion.cancelRequestedTx": claims: {
		resource: "companion SQLite (cancel request state)"
		trigger: "cancel flow in tx"
	}
	"internal/queue/companion.dialed.ExecContext": claims: {
		resource: "companion SQLite (dialed query wrapper)"
		trigger: "every dialed write"
	}
	"internal/queue/companion.dialed.QueryContext": claims: {
		resource: "companion SQLite (dialed query wrapper)"
		trigger: "every dialed read"
	}
	"internal/queue/companion.dialed.QueryRowContext": claims: {
		resource: "companion SQLite (dialed query wrapper)"
		trigger: "every dialed single-row read"
	}
	"internal/queue/companion.factDetailRefs": claims: {
		resource: "companion SQLite (fact detail refs)"
		trigger: "fold detail reads"
	}
	"internal/queue/companion.leaseErr": claims: {
		resource: "companion SQLite (task status for lease errors)"
		trigger: "lease failure diagnosis"
	}
	"internal/queue/companion.loadTask": claims: {
		resource: "companion SQLite (task row)"
		trigger: "task loads"
	}
	"internal/queue/companion.unblockParkedTask": claims: {
		resource: "companion SQLite (parked task unblock)"
		trigger: "unpark flow"
	}
	"internal/queue/companion/conform.TestEnqueueClaimBaseline10k": claims: {
		resource: "TQ_* env in conformance harness"
		trigger: "conformance baseline test"
	}
	"internal/queue/companion/conform.TestLoadSnapshotScaleAt100k": claims: {
		resource: "conformance SQLite (100k snapshot scale)"
		trigger: "conformance scale test"
	}
	"internal/queue/companion/conform.openLegacyStore": claims: {
		resource: "conformance legacy SQLite store"
		trigger: "conformance migration tests"
	}
	"internal/queue/postgresv4.Open": claims: {
		resource: "PostgreSQL (pgx pool DSN)"
		trigger: "postgres store open"
	}
	"internal/queue/postgresv4.OpenWithPool": claims: {
		resource: "PostgreSQL (existing pool DSN)"
		trigger: "postgres store open from pool"
	}
	"internal/queue/postgresv4.Store.Complete": claims: {
		resource: "PostgreSQL tasks table (complete CAS)"
		trigger: "task completion"
		criticality: "critical"
	}
	"internal/queue/sqlitev4.Store.Complete": claims: {
		resource: "task SQLite (complete CAS)"
		trigger: "task completion"
		criticality: "critical"
	}
	"internal/queue/sqlitev4.openSharedDB": claims: {
		resource: "task SQLite shared DSN (WAL + busy timeout)"
		trigger: "store open in every process"
	}
	"internal/queue/sqlitev4/migration.Probe": claims: {
		resource: "legacy SQLite read-only probe"
		trigger: "migration preflight"
	}
	"internal/queue/sqlitev4/migration.UpgradeIfNeeded": claims: {
		resource: "TQ_NO_AUTO_UPGRADE env"
		trigger: "store open upgrade gate"
	}
	"internal/queue/sqlitev4/migration.copyDeps": claims: {
		resource: "legacy deps table → v4 (in tx)"
		trigger: "migration data copy"
		criticality: "critical"
	}
	"internal/queue/sqlitev4/migration.copyLegacyTable": claims: {
		resource: "legacy table → v4 (in tx)"
		trigger: "migration data copy"
		criticality: "critical"
	}
	"internal/queue/sqlitev4/migration.copyQueriedRows": claims: {
		resource: "legacy rows → v4 (in tx)"
		trigger: "migration data copy"
		criticality: "critical"
	}
	"internal/queue/sqlitev4/migration.copyTasks": claims: {
		resource: "legacy tasks → v4 (in tx)"
		trigger: "migration data copy"
		criticality: "critical"
	}
	"internal/queue/sqlitev4/migration.migrateInto": claims: {
		resource: "migration destination SQLite + source snapshot (tx)"
		trigger: "v3→v4 upgrade"
		criticality: "critical"
		rationale: "the migration is the only whole-database rewrite in the system; torn state here loses the queue"
	}
	"internal/queue/sqlitev4/migration.oldAllFacts": claims: {
		resource: "legacy facts table read"
		trigger: "migration verification reads"
	}
	"internal/queue/sqlitev4/migration.oldListStatus": claims: {
		resource: "legacy task list read"
		trigger: "migration verification reads"
	}
	"internal/queue/sqlitev4/migration.oldStatusCounts": claims: {
		resource: "legacy status counts read"
		trigger: "migration verification reads"
	}
	"internal/queue/sqlitev4/migration.openSource": claims: {
		resource: "legacy SQLite read-only DSN"
		trigger: "migration source open"
	}
	"internal/queue/sqlitev4/migration.removeSQLiteSidecars": claims: {
		resource: "SQLite -wal/-shm sidecars of the legacy db (os.Remove)"
		trigger: "migration swap cleanup"
		durability: "durable"
		criticality: "critical"
		rationale: "removing the wrong sidecars at the wrong moment corrupts the legacy db; gated on converge success"
	}
	"internal/queue/sqlitev4/migration.restoreOnConvergeFailure": claims: {
		resource: "pre-migration backup file (read + restore)"
		trigger: "migration rollback on converge failure"
		durability: "durable"
		criticality: "critical"
		rationale: "the backup restore is the only undo path for a failed migration"
	}
	"internal/queue/sqlitev4/migration.snapshotLegacy": claims: {
		resource: "legacy SQLite backup snapshot"
		trigger: "migration start"
		criticality: "critical"
	}
	"internal/queue/sqlitev4/migration.sourceTableExists": claims: {
		resource: "legacy schema introspection"
		trigger: "migration table probe"
	}
	"internal/queue/sqlitev4/migration.verifyHeadSeq": claims: {
		resource: "legacy facts head seq read"
		trigger: "migration converge verification"
		criticality: "critical"
	}
	"internal/queue/sqlitev4/migration.verifyPriorityScores": claims: {
		resource: "legacy priority scores read"
		trigger: "migration converge verification"
		criticality: "critical"
	}
	"internal/queue/sqlitev4/migration.verifyProjectCounts": claims: {
		resource: "legacy project counts read"
		trigger: "migration converge verification"
		criticality: "critical"
	}
	"internal/queue/sqlitev4/migration.verifyWatermarks": claims: {
		resource: "legacy watermarks read"
		trigger: "migration converge verification"
		criticality: "critical"
	}
	"internal/readmodel.ConvergeProjectionHome": claims: {
		resource: "projection home dir metadata"
		trigger: "projection convergence check"
	}
	"internal/readmodel.NewProjectionHost": claims: {
		resource: "worker-failed marker file (os.Remove)"
		trigger: "projection host start clearing a stale marker"
		durability: "recoverable"
	}
	"internal/readmodel.OpenDeadLetters": claims: {
		resource: "DLQ SQLite (read model)"
		trigger: "dead-letter reads"
	}
	"internal/readmodel.ReadWorkerFailedMarker": claims: {
		resource: "worker-failed marker file"
		trigger: "projection resume diagnosis"
	}
	"internal/readmodel.probeMissingColumns": claims: {
		resource: "projection SQLite read-only column probe"
		trigger: "schema drift check"
	}
	"internal/readmodel.tryFoldLock": claims: {
		resource: "fold lock file (O_CREATE|O_RDWR flock)"
		trigger: "each fold attempt: single-folder fold contract"
		criticality: "critical"
		rationale: "the fold lock is the split-brain guard between folders; it is content-free — flock owns the mutual exclusion, the file bytes are never parsed, so a torn creation is harmless"
		durability: "ephemeral"
	}
	"internal/readmodel.writeWorkerFailedMarkerAt": claims: {
		resource: "worker-failed marker (tmp+rename + os.Remove)"
		trigger: "projection worker failure record"
		durability: "recoverable"
	}
	"internal/retryx.RenameTransient": claims: {
		resource: "retry helper around HTTP POST + os.Rename"
		trigger: "transient-rename flows (publish swaps)"
		durability: "recoverable"
	}
	"internal/session.LoadRegistry": claims: {
		resource: "session registry file"
		trigger: "session listing/loads"
	}
	"internal/session.PgrepOwner.Owns": claims: {
		resource: "pgrep subprocess"
		trigger: "registry liveness: does the owning pid live"
	}
	"internal/session.PingRegistry": claims: {
		resource: "session registry file (touch/open)"
		trigger: "session ping"
		durability: "ephemeral"
		rationale: "registry is liveness metadata; loss costs session tracking, not data"
	}
	"internal/session.RegistryPath": claims: {
		resource: "TQ_SESSION_REGISTRY env"
		trigger: "registry path resolution"
	}
	"internal/session.RewriteRegistry": claims: {
		resource: "session registry file rewrite"
		trigger: "session register/unregister"
		durability: "ephemeral"
	}
	"internal/status.closeoutReportPath": claims: {
		resource: "closeout report glob"
		trigger: "status closeout detection"
	}
	"internal/task.NewID": claims: {
		resource: "crypto/rand"
		trigger: "every task id mint"
	}
	"internal/testastguard.Parse": claims: {
		resource: "AST guard source file"
		trigger: "test guard parse"
	}
	"internal/webui.Server.Run": claims: {
		resource: "web UI HTTP listener"
		trigger: "tq web lifetime"
	}
	"internal/webui.attachWorkspace": claims: {
		resource: "crush server workspace attach HTTP"
		trigger: "web UI workspace attach"
	}
	"internal/webui.crushUnixClient": claims: {
		resource: "crush server unix socket dial"
		trigger: "web UI crush transport"
	}
	"internal/webui.discoverWorkspaces": claims: {
		resource: "crush server /v1/workspaces"
		trigger: "web UI workspace discovery"
	}
	"internal/webui.fetchWorkspaceBusy": claims: {
		resource: "crush server workspace sessions"
		trigger: "web UI busy-state poll"
	}
	"internal/webui.newNonce": claims: {
		resource: "crypto/rand"
		trigger: "web UI session nonce mint"
	}
	"internal/webui.staticFS": claims: {
		resource: "embedded static assets (go:embed)"
		trigger: "web UI asset serving"
	}
}
