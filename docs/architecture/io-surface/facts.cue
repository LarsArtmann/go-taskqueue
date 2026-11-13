// facts.cue — extracted from code by `io-surface init`.
//
// Mechanical file: regenerate with `io-surface init --force` after an
// I/O-adding change; never hand-edit locations. Judgment lives in claims.cue.
package io_surface

audit: {
	commit: "a5cd73d0"
	date:   "2026-11-13"
	claims: 361
	method: "io-surface extract (AST, stdlib-only)"
}

enforcement: "gate"

scope: {
	in: [
		"file system (reads, writes, embeds)",
		"databases (SQLite/engine handles)",
		"network listeners",
		"outbound network",
		"stream copies",
		"entropy",
		"subprocess / local IPC",
		"environment reads",
	]
	out: [
		"timers and tickers (scheduling)",
		"memory pooling",
		"kernel-internal I/O (epoll, page cache)",
		"process stdio (stdout/stderr — user feedback, not durable state; DECIDED 2026-11-08, TODO #49)",
	]
}

sites: {
	"cmd/tq.appendRSSSample": {
		location: [
			"cmd/tq/agentpool.go:1286",
		]
		family: "file-write"
		facts: {
			call: ["os.OpenFile"]
		}
	}
	"cmd/tq.applyPoolConfigFile": {
		location: [
			"cmd/tq/poolconfig.go:114",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"cmd/tq.applyPoolHealthConfigDefault": {
		location: [
			"cmd/tq/poolhealth.go:451",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"cmd/tq.bootstrapOptions.ensureCrushConfig": {
		location: [
			"cmd/tq/bootstrap.go:569",
			"cmd/tq/bootstrap.go:591",
			"cmd/tq/bootstrap.go:599",
		]
		family: "file-write"
		facts: {
			call: ["os.ReadFile", "os.WriteFile"]
		}
	}
	"cmd/tq.bootstrapOptions.ensureRepos": {
		location: [
			"cmd/tq/bootstrap.go:424",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"cmd/tq.bootstrapOptions.ensureTQVerify": {
		location: [
			"cmd/tq/bootstrap.go:537",
			"cmd/tq/bootstrap.go:553",
		]
		family: "file-write"
		facts: {
			call: ["os.WriteFile"]
		}
	}
	"cmd/tq.bootstrapOptions.installService": {
		location: [
			"cmd/tq/bootstrap.go:753",
			"cmd/tq/bootstrap.go:758",
			"cmd/tq/bootstrap.go:763",
			"cmd/tq/bootstrap.go:768",
			"cmd/tq/bootstrap.go:777",
		]
		family: "file-write"
		facts: {
			call: ["exec.Command", "os.MkdirAll", "os.WriteFile"]
		}
	}
	"cmd/tq.bootstrapOptions.resolveRepos": {
		location: [
			"cmd/tq/bootstrap.go:379",
			"cmd/tq/bootstrap.go:390",
			"cmd/tq/bootstrap.go:400",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"cmd/tq.buildAgentConveniencePayload": {
		location: [
			"cmd/tq/main.go:598",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"cmd/tq.cmdAPI": {
		location: [
			"cmd/tq/main.go:4003",
			"cmd/tq/main.go:4056",
		]
		family: "listener"
		facts: {
			call: [".ListenAndServe", "os.Getenv"]
		}
	}
	"cmd/tq.cmdAsk": {
		location: [
			"cmd/tq/ask.go:187",
			"cmd/tq/ask.go:202",
		]
		family: "file-write"
		facts: {
			call: ["os.Getenv", "os.WriteFile"]
		}
	}
	"cmd/tq.cmdEnqueue": {
		location: [
			"cmd/tq/main.go:418",
			"cmd/tq/main.go:465",
		]
		family: "env"
		facts: {
			call: ["os.Getenv", "os.ReadFile"]
		}
	}
	"cmd/tq.cmdHarvest": {
		location: [
			"cmd/tq/main.go:839",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"cmd/tq.cmdPoolHealth": {
		location: [
			"cmd/tq/poolhealth.go:492",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"cmd/tq.cmdServe": {
		location: [
			"cmd/tq/main.go:4250",
			"cmd/tq/main.go:4255",
			"cmd/tq/main.go:4269",
			"cmd/tq/main.go:4279",
			"cmd/tq/main.go:4284",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"cmd/tq.cmdVerdict": {
		location: [
			"cmd/tq/verdict.go:51",
			"cmd/tq/verdict.go:83",
			"cmd/tq/verdict.go:94",
		]
		family: "file-write"
		facts: {
			call: ["io.ReadAll", "os.Getenv", "os.WriteFile"]
		}
	}
	"cmd/tq.cmdWorker": {
		location: [
			"cmd/tq/main.go:650",
			"cmd/tq/main.go:653",
			"cmd/tq/main.go:662",
			"cmd/tq/main.go:675",
		]
		family: "env"
		facts: {
			call: ["os.Getenv", "os.Setenv"]
		}
	}
	"cmd/tq.commitsForTask": {
		location: [
			"cmd/tq/main.go:3071",
			"cmd/tq/main.go:3075",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.Command", "os.Stat"]
		}
	}
	"cmd/tq.crushKeeper": {
		location: [
			"cmd/tq/agentpool.go:1224",
			"cmd/tq/agentpool.go:1241",
			"cmd/tq/agentpool.go:1246",
			"cmd/tq/agentpool.go:1248",
		]
		family: "outbound"
		facts: {
			call: [".DialContext", ".Do", "http.NewRequestWithContext", "io.Copy", "io.LimitReader"]
		}
	}
	"cmd/tq.crushRun": {
		location: [
			"cmd/tq/crush.go:49",
			"cmd/tq/crush.go:73",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.Command", "os.Getenv"]
		}
	}
	"cmd/tq.crushServerRSSLogPath": {
		location: [
			"cmd/tq/agentpool.go:1270",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"cmd/tq.defaultDB": {
		location: [
			"cmd/tq/main.go:179",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"cmd/tq.defaultProjectsDir": {
		location: [
			"cmd/tq/main.go:785",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"cmd/tq.doctorCrushManagedBlock": {
		location: [
			"cmd/tq/doctor.go:2227",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"cmd/tq.doctorEnvironment": {
		location: [
			"cmd/tq/doctor.go:1600",
			"cmd/tq/doctor.go:1616",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.LookPath"]
		}
	}
	"cmd/tq.doctorProjection": {
		location: [
			"cmd/tq/doctor.go:1281",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"cmd/tq.doctorProjectionDLQ": {
		location: [
			"cmd/tq/doctor.go:1378",
			"cmd/tq/doctor.go:1379",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"cmd/tq.doctorRepoAutonomy": {
		location: [
			"cmd/tq/doctor.go:2194",
			"cmd/tq/doctor.go:2207",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"cmd/tq.doctorRepoCoverage": {
		location: [
			"cmd/tq/doctor.go:831",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"cmd/tq.doctorSQLiteChecks": {
		location: [
			"cmd/tq/doctor.go:321",
			"cmd/tq/doctor.go:328",
			"cmd/tq/doctor.go:339",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext", "sql.Open"]
		}
	}
	"cmd/tq.doctorTagAncestry": {
		location: [
			"cmd/tq/doctor.go:2149",
			"cmd/tq/doctor.go:2154",
			"cmd/tq/doctor.go:2167",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.Command"]
		}
	}
	"cmd/tq.doctorTreeGofmt": {
		location: [
			"cmd/tq/doctor.go:1718",
			"cmd/tq/doctor.go:1736",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"cmd/tq.doctorVerdictChannel": {
		location: [
			"cmd/tq/doctor.go:1669",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.LookPath"]
		}
	}
	"cmd/tq.doctorVerifyPins": {
		location: [
			"cmd/tq/doctor.go:1063",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"cmd/tq.ensureCommitted": {
		location: [
			"cmd/tq/bootstrap.go:648",
			"cmd/tq/bootstrap.go:655",
			"cmd/tq/bootstrap.go:668",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.Command"]
		}
	}
	"cmd/tq.executableFile": {
		location: [
			"cmd/tq/doctor.go:2030",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"cmd/tq.expandRepoSpecs": {
		location: [
			"cmd/tq/main.go:326",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"cmd/tq.foreignTaskRefs": {
		location: [
			"cmd/tq/main.go:3037",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.Command"]
		}
	}
	"cmd/tq.gitChangedFiles": {
		location: [
			"cmd/tq/main.go:2924",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.Command"]
		}
	}
	"cmd/tq.gitChildCommit": {
		location: [
			"cmd/tq/main.go:2964",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.Command"]
		}
	}
	"cmd/tq.gitCommitMeta": {
		location: [
			"cmd/tq/main.go:2944",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.Command"]
		}
	}
	"cmd/tq.goEnvBuild": {
		location: [
			"cmd/tq/doctor.go:2094",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"cmd/tq.loadPoolConfigFile": {
		location: [
			"cmd/tq/poolconfig.go:27",
		]
		family: "file-read"
		facts: {
			call: ["os.Open"]
		}
	}
	"cmd/tq.parkBy": {
		location: [
			"cmd/tq/park.go:105",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"cmd/tq.parseAgentPoolOptions": {
		location: [
			"cmd/tq/agentpool.go:127",
			"cmd/tq/agentpool.go:166",
			"cmd/tq/agentpool.go:171",
			"cmd/tq/agentpool.go:238",
			"cmd/tq/agentpool.go:241",
			"cmd/tq/agentpool.go:242",
			"cmd/tq/agentpool.go:331",
			"cmd/tq/agentpool.go:334",
			"cmd/tq/agentpool.go:378",
			"cmd/tq/agentpool.go:393",
			"cmd/tq/agentpool.go:398",
			"cmd/tq/agentpool.go:471",
			"cmd/tq/agentpool.go:475",
			"cmd/tq/agentpool.go:480",
			"cmd/tq/agentpool.go:482",
			"cmd/tq/agentpool.go:488",
		]
		family: "file-write"
		facts: {
			call: ["os.Getenv", "os.MkdirAll", "os.Setenv"]
		}
	}
	"cmd/tq.parseSystemdUnitEnv": {
		location: [
			"cmd/tq/doctor.go:1935",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"cmd/tq.printCQRSEvents": {
		location: [
			"cmd/tq/main.go:3786",
		]
		family: "stream-copy"
		facts: {
			call: [".ReadAll"]
		}
	}
	"cmd/tq.procRSSKB": {
		location: [
			"cmd/tq/agentpool.go:1304",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"cmd/tq.readCrushServerRSS": {
		location: [
			"cmd/tq/poolhealth.go:126",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"cmd/tq.repoDirty": {
		location: [
			"cmd/tq/bootstrap.go:499",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.Command"]
		}
	}
	"cmd/tq.resolveGofmt": {
		location: [
			"cmd/tq/doctor.go:1787",
			"cmd/tq/doctor.go:1791",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.Command", "exec.LookPath"]
		}
	}
	"cmd/tq.runCrushServerActor": {
		location: [
			"cmd/tq/agentpool.go:1141",
			"cmd/tq/agentpool.go:1161",
		]
		family: "file-write"
		facts: {
			call: ["exec.CommandContext", "os.MkdirAll"]
		}
	}
	"cmd/tq.runDLQ": {
		location: [
			"cmd/tq/main.go:3246",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"cmd/tq.runGateBoard": {
		location: [
			"cmd/tq/doctor.go:608",
			"cmd/tq/doctor.go:622",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext", "os.Stat"]
		}
	}
	"cmd/tq.runGoEnvProbe": {
		location: [
			"cmd/tq/doctor.go:2074",
			"cmd/tq/doctor.go:2078",
		]
		family: "file-write"
		facts: {
			call: ["os.WriteFile"]
		}
	}
	"cmd/tq.sessionBegin": {
		location: [
			"cmd/tq/session.go:192",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"cmd/tq.sessionClose": {
		location: [
			"cmd/tq/session.go:234",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"cmd/tq.sessionPing": {
		location: [
			"cmd/tq/session.go:50",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"internal/bridge/cqa.Bridge.Collect": {
		location: [
			"internal/bridge/cqa/cqa.go:123",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/bridge/cqa.Bridge.getJSON": {
		location: [
			"internal/bridge/cqa/cqa.go:277",
			"internal/bridge/cqa/cqa.go:286",
			"internal/bridge/cqa/cqa.go:292",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.NewRequestWithContext", "io.LimitReader", "io.ReadAll"]
		}
	}
	"internal/bridge/papdashboard.AnswerPoller.list": {
		location: [
			"internal/bridge/papdashboard/answers.go:294",
			"internal/bridge/papdashboard/answers.go:308",
			"internal/bridge/papdashboard/answers.go:316",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.NewRequestWithContext", "io.LimitReader", "io.ReadAll"]
		}
	}
	"internal/bridge/papdashboard.HealthRelay.handle": {
		location: [
			"internal/bridge/papdashboard/healthrelay.go:185",
		]
		family: "stream-copy"
		facts: {
			call: ["io.LimitReader"]
		}
	}
	"internal/bridge/papdashboard.NewHealthRelay": {
		location: [
			"internal/bridge/papdashboard/healthrelay.go:152",
			"internal/bridge/papdashboard/healthrelay.go:172",
		]
		family: "listener"
		facts: {
			call: [".Serve", "net.Listen"]
		}
	}
	"internal/bridge/papdashboard.ingester.post": {
		location: [
			"internal/bridge/papdashboard/healthrelay.go:60",
			"internal/bridge/papdashboard/healthrelay.go:70",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.NewRequestWithContext"]
		}
	}
	"internal/budget.Guard.runBudgetCmd": {
		location: [
			"internal/budget/budget.go:160",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/config.SyncPolicyFromEnv": {
		location: [
			"internal/config/config.go:114",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"internal/config.parseSyncPolicy": {
		location: [
			"internal/config/config.go:128",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"internal/depsweep.DepgraphSource.Plan": {
		location: [
			"internal/depsweep/plan.go:128",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/docsgates.binaryRunner": {
		location: [
			"internal/docsgates/bridge.go:140",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/executor.AgentExecutor.Execute": {
		location: [
			"internal/executor/agent.go:302",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.AgentExecutor.binary": {
		location: [
			"internal/executor/agent.go:211",
			"internal/executor/agent.go:215",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"internal/executor.AgentExecutor.runAgent": {
		location: [
			"internal/executor/agent.go:691",
			"internal/executor/agent.go:696",
			"internal/executor/agent.go:703",
			"internal/executor/agent.go:708",
			"internal/executor/agent.go:810",
			"internal/executor/agent.go:817",
		]
		family: "file-write"
		facts: {
			call: ["exec.CommandContext", "os.CreateTemp", "os.Environ", "os.Remove"]
		}
	}
	"internal/executor.AgentExecutor.runCloseoutTurn": {
		location: [
			"internal/executor/agent.go:1037",
			"internal/executor/agent.go:1040",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext", "os.Environ"]
		}
	}
	"internal/executor.AgentVersion": {
		location: [
			"internal/executor/agent.go:236",
			"internal/executor/agent.go:244",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext", "exec.LookPath"]
		}
	}
	"internal/executor.CommandExecutor.Execute": {
		location: [
			"internal/executor/command.go:49",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/executor.DepBumpExecutor.Execute": {
		location: [
			"internal/executor/depbump.go:216",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.DepBumpExecutor.commit": {
		location: [
			"internal/executor/depbump.go:439",
			"internal/executor/depbump.go:444",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.DepBumpExecutor.quarantineGoWork": {
		location: [
			"internal/executor/depbump.go:584",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.DepBumpExecutor.regenerateTempl": {
		location: [
			"internal/executor/depbump.go:375",
			"internal/executor/depbump.go:402",
			"internal/executor/depbump.go:410",
			"internal/executor/depbump.go:418",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext", "exec.LookPath", "filepath.WalkDir", "os.Environ"]
		}
	}
	"internal/executor.DepBumpExecutor.run": {
		location: [
			"internal/executor/depbump.go:766",
			"internal/executor/depbump.go:777",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext", "os.Environ"]
		}
	}
	"internal/executor.DepBumpExecutor.syncGomod": {
		location: [
			"internal/executor/depbump.go:704",
			"internal/executor/depbump.go:720",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.LookPath", "os.Stat"]
		}
	}
	"internal/executor.DepBumpExecutor.verify": {
		location: [
			"internal/executor/depbump.go:286",
			"internal/executor/depbump.go:294",
		]
		family: "file-write"
		facts: {
			call: ["os.RemoveAll", "os.TempDir"]
		}
	}
	"internal/executor.GitLogScanner.CommitsByTrailer": {
		location: [
			"internal/executor/gitscan.go:146",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/executor.HTTPExecutor.Execute": {
		location: [
			"internal/executor/http.go:66",
			"internal/executor/http.go:73",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.NewRequestWithContext"]
		}
	}
	"internal/executor.HTTPExecutor.rateLimited": {
		location: [
			"internal/executor/http.go:106",
		]
		family: "stream-copy"
		facts: {
			call: ["io.LimitReader", "io.ReadAll"]
		}
	}
	"internal/executor.ReadVerdict": {
		location: [
			"internal/executor/result.go:389",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"internal/executor.ScanWorktreeOrphans": {
		location: [
			"internal/executor/worktreereap.go:38",
			"internal/executor/worktreereap.go:46",
			"internal/executor/worktreereap.go:78",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadDir"]
		}
	}
	"internal/executor.SweepSidecars": {
		location: [
			"internal/executor/sidecar.go:82",
		]
		family: "file-write"
		facts: {
			call: ["os.Remove"]
		}
	}
	"internal/executor.SweepSidecarsByBytes": {
		location: [
			"internal/executor/sidecar.go:124",
		]
		family: "file-write"
		facts: {
			call: ["os.Remove"]
		}
	}
	"internal/executor.acquireAgentSlot": {
		location: [
			"internal/executor/agentlock_unix.go:36",
			"internal/executor/agentlock_unix.go:44",
		]
		family: "file-write"
		facts: {
			call: ["os.MkdirAll", "os.OpenFile"]
		}
	}
	"internal/executor.agentSlotDir": {
		location: [
			"internal/executor/agentlock_unix.go:18",
			"internal/executor/agentlock_unix.go:22",
		]
		family: "env"
		facts: {
			call: ["os.Getenv", "os.TempDir"]
		}
	}
	"internal/executor.assertCleanTree": {
		location: [
			"internal/executor/agent.go:543",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/executor.autoDetectVerify": {
		location: [
			"internal/executor/agent.go:1417",
			"internal/executor/agent.go:1421",
			"internal/executor/agent.go:1425",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.baselineVerify": {
		location: [
			"internal/executor/verifygate.go:191",
			"internal/executor/verifygate.go:199",
			"internal/executor/verifygate.go:205",
		]
		family: "file-write"
		facts: {
			call: ["exec.CommandContext", "os.RemoveAll", "os.TempDir"]
		}
	}
	"internal/executor.buildflowCovered": {
		location: [
			"internal/executor/depbump.go:740",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.checkGitVersion": {
		location: [
			"internal/executor/gitscan.go:74",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/executor.checkWorktreeRails": {
		location: [
			"internal/executor/worktree.go:206",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.collectSidecars": {
		location: [
			"internal/executor/sidecar.go:27",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadDir"]
		}
	}
	"internal/executor.createWorktree": {
		location: [
			"internal/executor/worktree.go:68",
			"internal/executor/worktree.go:89",
		]
		family: "file-write"
		facts: {
			call: ["os.RemoveAll", "os.Stat"]
		}
	}
	"internal/executor.crushServerClient.getJSON": {
		location: [
			"internal/executor/crushserver.go:171",
			"internal/executor/crushserver.go:176",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.NewRequestWithContext"]
		}
	}
	"internal/executor.crushServerClient.postJSON": {
		location: [
			"internal/executor/crushserver.go:195",
			"internal/executor/crushserver.go:202",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.NewRequestWithContext"]
		}
	}
	"internal/executor.defaultVerify": {
		location: [
			"internal/executor/agent.go:1389",
			"internal/executor/agent.go:1402",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.filesInCommits": {
		location: [
			"internal/executor/outcome.go:89",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/executor.gitRun": {
		location: [
			"internal/executor/verifygate.go:402",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.Command"]
		}
	}
	"internal/executor.gofmtFlagged": {
		location: [
			"internal/executor/verifygate.go:374",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/executor.newCrushServerClient": {
		location: [
			"internal/executor/crushserver.go:126",
		]
		family: "outbound"
		facts: {
			call: [".DialContext"]
		}
	}
	"internal/executor.prepareRepo": {
		location: [
			"internal/executor/preflight.go:21",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.questionPendingFrom": {
		location: [
			"internal/executor/question.go:76",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"internal/executor.randomClientID": {
		location: [
			"internal/executor/crushserver.go:385",
		]
		family: "entropy"
		facts: {
			call: ["rand.Read"]
		}
	}
	"internal/executor.readTQVerify": {
		location: [
			"internal/executor/agent.go:1334",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"internal/executor.recordCalibration": {
		location: [
			"internal/executor/calibration.go:126",
			"internal/executor/calibration.go:92",
		]
		family: "file-write"
		facts: {
			call: ["os.Getenv", "os.OpenFile"]
		}
	}
	"internal/executor.redactActive": {
		location: [
			"internal/executor/redact.go:104",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"internal/executor.removeWorktree": {
		location: [
			"internal/executor/worktree.go:237",
		]
		family: "file-write"
		facts: {
			call: ["os.RemoveAll"]
		}
	}
	"internal/executor.requireRepoAutonomy": {
		location: [
			"internal/executor/agent.go:1300",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.requireReportFile": {
		location: [
			"internal/executor/status.go:223",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.resolveRepoDir": {
		location: [
			"internal/executor/agent.go:509",
			"internal/executor/agent.go:522",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.retrySession": {
		location: [
			"internal/executor/agent.go:665",
			"internal/executor/agent.go:670",
		]
		family: "env"
		facts: {
			call: ["os.Getenv", "os.ReadFile"]
		}
	}
	"internal/executor.runVerify": {
		location: [
			"internal/executor/agent.go:1188",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/executor.vendorGofmtSignature": {
		location: [
			"internal/executor/verifygate.go:246",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/executor.writeOutputSidecar": {
		location: [
			"internal/executor/agent.go:450",
			"internal/executor/agent.go:455",
			"internal/executor/agent.go:466",
		]
		family: "file-write"
		facts: {
			call: ["os.Getenv", "os.MkdirAll", "os.WriteFile"]
		}
	}
	"internal/executor.writeVerifyEvidence": {
		location: [
			"internal/executor/sidecar.go:144",
			"internal/executor/sidecar.go:150",
			"internal/executor/sidecar.go:156",
		]
		family: "file-write"
		facts: {
			call: ["os.Getenv", "os.MkdirAll", "os.WriteFile"]
		}
	}
	"internal/harvest.DiscoverRepos": {
		location: [
			"internal/harvest/harvest.go:1273",
			"internal/harvest/harvest.go:1286",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadDir", "os.Stat"]
		}
	}
	"internal/harvest.DiscoverReposDaemon": {
		location: [
			"internal/harvest/discovery.go:61",
			"internal/harvest/discovery.go:74",
			"internal/harvest/discovery.go:81",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.NewRequestWithContext", "io.LimitReader", "io.ReadAll"]
		}
	}
	"internal/harvest.EffortCache.Load": {
		location: [
			"internal/harvest/effort.go:509",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"internal/harvest.EffortCache.Save": {
		location: [
			"internal/harvest/effort.go:540",
		]
		family: "file-write"
		facts: {
			call: ["os.WriteFile"]
		}
	}
	"internal/harvest.Harvester.donePreflightRepoDir": {
		location: [
			"internal/harvest/donepreflight.go:134",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/harvest.ParseRepoAll": {
		location: [
			"internal/harvest/harvest.go:1328",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"internal/harvest.ReadImportance": {
		location: [
			"internal/harvest/priority.go:144",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"internal/harvest.RepoPurpose": {
		location: [
			"internal/harvest/priority.go:194",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"internal/harvest.Watcher.streamOnce": {
		location: [
			"internal/harvest/watch.go:164",
			"internal/harvest/watch.go:171",
			"internal/harvest/watch.go:178",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.NewRequestWithContext", "io.LimitReader", "io.ReadAll"]
		}
	}
	"internal/harvest.claimLive": {
		location: [
			"internal/harvest/verdicts.go:142",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/harvest.closeoutReportExists": {
		location: [
			"internal/harvest/redispatch.go:205",
		]
		family: "file-read"
		facts: {
			call: ["filepath.Glob"]
		}
	}
	"internal/harvest.gitOutputs": {
		location: [
			"internal/harvest/donepreflight.go:306",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/harvest.gitSucceeds": {
		location: [
			"internal/harvest/donepreflight.go:300",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/harvest.isDangling": {
		location: [
			"internal/harvest/citation.go:48",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext"]
		}
	}
	"internal/harvest.jevClient.ask": {
		location: [
			"internal/harvest/effort.go:216",
			"internal/harvest/effort.go:224",
			"internal/harvest/effort.go:235",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.MaxBytesReader", "http.NewRequestWithContext", "io.ReadAll"]
		}
	}
	"internal/harvest.mapDaemonProjects": {
		location: [
			"internal/harvest/discovery.go:124",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/harvest.unixSocketClient": {
		location: [
			"internal/harvest/discovery.go:200",
		]
		family: "outbound"
		facts: {
			call: [".DialContext"]
		}
	}
	"internal/harvest.verdictBlock": {
		location: [
			"internal/harvest/verdicts.go:124",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"internal/httpapi.Server.ListenAndServe": {
		location: [
			"internal/httpapi/httpapi.go:541",
		]
		family: "listener"
		facts: {
			call: [".ListenAndServe"]
		}
	}
	"internal/httpapi.Server.handleEnqueue": {
		location: [
			"internal/httpapi/httpapi.go:330",
		]
		family: "stream-copy"
		facts: {
			call: ["http.MaxBytesReader"]
		}
	}
	"internal/httpapi.Server.handleReportError": {
		location: [
			"internal/httpapi/httpapi.go:418",
		]
		family: "stream-copy"
		facts: {
			call: ["http.MaxBytesReader"]
		}
	}
	"internal/queue/companion.ClaimCounts": {
		location: [
			"internal/queue/companion/reads.go:467",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/companion.ClaimDue": {
		location: [
			"internal/queue/companion/claims.go:121",
			"internal/queue/companion/claims.go:176",
			"internal/queue/companion/claims.go:207",
		]
		family: "database"
		facts: {
			call: [".ExecContext", ".QueryRowContext"]
		}
	}
	"internal/queue/companion.CountFacts": {
		location: [
			"internal/queue/companion/reads.go:233",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/companion.CountTasks": {
		location: [
			"internal/queue/companion/reads.go:180",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/companion.DeletePriorityScores": {
		location: [
			"internal/queue/companion/reads.go:421",
		]
		family: "database"
		facts: {
			call: [".ExecContext"]
		}
	}
	"internal/queue/companion.FactsSince": {
		location: [
			"internal/queue/companion/reads.go:259",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/companion.HeadSeq": {
		location: [
			"internal/queue/companion/reads.go:224",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/companion.LastFacts": {
		location: [
			"internal/queue/companion/reads.go:200",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/companion.List": {
		location: [
			"internal/queue/companion/reads.go:154",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/companion.ListWatermarks": {
		location: [
			"internal/queue/companion/reads.go:304",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/companion.Migrate": {
		location: [
			"internal/queue/companion/schema.go:50",
			"internal/queue/companion/schema.go:54",
		]
		family: "database"
		facts: {
			call: [".ExecContext"]
		}
	}
	"internal/queue/companion.Park": {
		location: [
			"internal/queue/companion/parks.go:34",
			"internal/queue/companion/parks.go:52",
		]
		family: "database"
		facts: {
			call: [".ExecContext", ".QueryRowContext"]
		}
	}
	"internal/queue/companion.PriorityScore": {
		location: [
			"internal/queue/companion/reads.go:361",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/companion.PriorityScores": {
		location: [
			"internal/queue/companion/reads.go:382",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/companion.ProjectCounts": {
		location: [
			"internal/queue/companion/reads.go:498",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/companion.RecordAnswer": {
		location: [
			"internal/queue/companion/questions.go:50",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/companion.Requeue": {
		location: [
			"internal/queue/companion/claims.go:268",
			"internal/queue/companion/claims.go:279",
		]
		family: "database"
		facts: {
			call: [".ExecContext", ".QueryRowContext"]
		}
	}
	"internal/queue/companion.SavePriorityScore": {
		location: [
			"internal/queue/companion/reads.go:341",
		]
		family: "database"
		facts: {
			call: [".ExecContext"]
		}
	}
	"internal/queue/companion.SaveWatermark": {
		location: [
			"internal/queue/companion/reads.go:287",
		]
		family: "database"
		facts: {
			call: [".ExecContext"]
		}
	}
	"internal/queue/companion.SetWatermark": {
		location: [
			"internal/queue/companion/reads.go:329",
		]
		family: "database"
		facts: {
			call: [".ExecContext"]
		}
	}
	"internal/queue/companion.StatusCounts": {
		location: [
			"internal/queue/companion/reads.go:436",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/companion.TokenFor": {
		location: [
			"internal/queue/companion/claims.go:24",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/companion.Watermark": {
		location: [
			"internal/queue/companion/reads.go:272",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/companion.WithTx": {
		location: [
			"internal/queue/companion/core.go:97",
		]
		family: "database"
		facts: {
			call: [".BeginTx"]
		}
	}
	"internal/queue/companion.appendFact": {
		location: [
			"internal/queue/companion/claims.go:322",
		]
		family: "database"
		facts: {
			call: [".ExecContext"]
		}
	}
	"internal/queue/companion.cancelRequestedReasonTx": {
		location: [
			"internal/queue/companion/claims.go:341",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/companion.cancelRequestedTx": {
		location: [
			"internal/queue/companion/claims.go:333",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/companion.dialed.ExecContext": {
		location: [
			"internal/queue/companion/core.go:83",
		]
		family: "database"
		facts: {
			call: [".ExecContext"]
		}
	}
	"internal/queue/companion.dialed.QueryContext": {
		location: [
			"internal/queue/companion/core.go:87",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/companion.dialed.QueryRowContext": {
		location: [
			"internal/queue/companion/core.go:91",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/companion.factDetailRefs": {
		location: [
			"internal/queue/companion/questions.go:132",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/companion.leaseErr": {
		location: [
			"internal/queue/companion/claims.go:379",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/companion.loadTask": {
		location: [
			"internal/queue/companion/reads.go:530",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/companion.unblockParkedTask": {
		location: [
			"internal/queue/companion/questions.go:120",
		]
		family: "database"
		facts: {
			call: [".ExecContext"]
		}
	}
	"internal/queue/companion/conform.TestEnqueueClaimBaseline10k": {
		location: [
			"internal/queue/companion/conform/tests.go:2789",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"internal/queue/companion/conform.TestLoadSnapshotScaleAt100k": {
		location: [
			"internal/queue/companion/conform/tests.go:2393",
			"internal/queue/companion/conform/tests.go:2401",
			"internal/queue/companion/conform/tests.go:2411",
			"internal/queue/companion/conform/tests.go:2415",
		]
		family: "database"
		facts: {
			call: [".ExecContext", ".PrepareContext"]
		}
	}
	"internal/queue/companion/conform.openLegacyStore": {
		location: [
			"internal/queue/companion/conform/tests.go:912",
		]
		family: "database"
		facts: {
			call: ["sql.Open"]
		}
	}
	"internal/queue/postgresv4.Open": {
		location: [
			"internal/queue/postgresv4/adapter.go:125",
		]
		family: "database"
		facts: {
			call: ["sql.Open"]
		}
	}
	"internal/queue/postgresv4.OpenWithPool": {
		location: [
			"internal/queue/postgresv4/adapter.go:152",
		]
		family: "database"
		facts: {
			call: ["sql.Open"]
		}
	}
	"internal/queue/postgresv4.Store.Complete": {
		location: [
			"internal/queue/postgresv4/adapter.go:212",
		]
		family: "database"
		facts: {
			call: [".ExecContext"]
		}
	}
	"internal/queue/sqlitev4.Store.Complete": {
		location: [
			"internal/queue/sqlitev4/adapter.go:228",
		]
		family: "database"
		facts: {
			call: [".ExecContext"]
		}
	}
	"internal/queue/sqlitev4.openSharedDB": {
		location: [
			"internal/queue/sqlitev4/adapter.go:107",
		]
		family: "database"
		facts: {
			call: ["sql.Open"]
		}
	}
	"internal/queue/sqlitev4/migration.Probe": {
		location: [
			"internal/queue/sqlitev4/migration/upgrade.go:159",
		]
		family: "database"
		facts: {
			call: ["sql.Open"]
		}
	}
	"internal/queue/sqlitev4/migration.UpgradeIfNeeded": {
		location: [
			"internal/queue/sqlitev4/migration/upgrade.go:108",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"internal/queue/sqlitev4/migration.copyDeps": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:325",
			"internal/queue/sqlitev4/migration/migrate.go:340",
		]
		family: "database"
		facts: {
			call: [".ExecContext", ".QueryContext"]
		}
	}
	"internal/queue/sqlitev4/migration.copyLegacyTable": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:417",
			"internal/queue/sqlitev4/migration/migrate.go:427",
		]
		family: "database"
		facts: {
			call: [".ExecContext", ".QueryRowContext"]
		}
	}
	"internal/queue/sqlitev4/migration.copyQueriedRows": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:441",
			"internal/queue/sqlitev4/migration/migrate.go:468",
		]
		family: "database"
		facts: {
			call: [".ExecContext", ".QueryContext"]
		}
	}
	"internal/queue/sqlitev4/migration.copyTasks": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:269",
			"internal/queue/sqlitev4/migration/migrate.go:300",
		]
		family: "database"
		facts: {
			call: [".ExecContext", ".QueryContext"]
		}
	}
	"internal/queue/sqlitev4/migration.migrateInto": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:194",
			"internal/queue/sqlitev4/migration/migrate.go:201",
		]
		family: "database"
		facts: {
			call: [".BeginTx", "sql.Open"]
		}
	}
	"internal/queue/sqlitev4/migration.oldAllFacts": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:919",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/sqlitev4/migration.oldListStatus": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:962",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/sqlitev4/migration.oldStatusCounts": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:528",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/sqlitev4/migration.openSource": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:144",
		]
		family: "database"
		facts: {
			call: ["sql.Open"]
		}
	}
	"internal/queue/sqlitev4/migration.removeSQLiteSidecars": {
		location: [
			"internal/queue/sqlitev4/migration/upgrade.go:246",
		]
		family: "file-write"
		facts: {
			call: ["os.Remove"]
		}
	}
	"internal/queue/sqlitev4/migration.restoreOnConvergeFailure": {
		location: [
			"internal/queue/sqlitev4/migration/upgrade.go:226",
			"internal/queue/sqlitev4/migration/upgrade.go:233",
		]
		family: "file-write"
		facts: {
			call: ["os.ReadFile", "os.WriteFile"]
		}
	}
	"internal/queue/sqlitev4/migration.snapshotLegacy": {
		location: [
			"internal/queue/sqlitev4/migration/upgrade.go:202",
			"internal/queue/sqlitev4/migration/upgrade.go:212",
		]
		family: "database"
		facts: {
			call: [".ExecContext", "sql.Open"]
		}
	}
	"internal/queue/sqlitev4/migration.sourceTableExists": {
		location: [
			"internal/queue/sqlitev4/migration/helpers.go:22",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/sqlitev4/migration.verifyHeadSeq": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:848",
		]
		family: "database"
		facts: {
			call: [".QueryRowContext"]
		}
	}
	"internal/queue/sqlitev4/migration.verifyPriorityScores": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:730",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/sqlitev4/migration.verifyProjectCounts": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:581",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/queue/sqlitev4/migration.verifyWatermarks": {
		location: [
			"internal/queue/sqlitev4/migration/migrate.go:674",
		]
		family: "database"
		facts: {
			call: [".QueryContext"]
		}
	}
	"internal/readmodel.ConvergeProjectionHome": {
		location: [
			"internal/readmodel/schema.go:39",
		]
		family: "file-read"
		facts: {
			call: ["os.Stat"]
		}
	}
	"internal/readmodel.NewProjectionHost": {
		location: [
			"internal/readmodel/host.go:272",
		]
		family: "file-write"
		facts: {
			call: ["os.Remove"]
		}
	}
	"internal/readmodel.OpenDeadLetters": {
		location: [
			"internal/readmodel/dlq.go:73",
		]
		family: "database"
		facts: {
			call: ["sql.Open"]
		}
	}
	"internal/readmodel.ReadWorkerFailedMarker": {
		location: [
			"internal/readmodel/workerfailed.go:52",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"internal/readmodel.probeMissingColumns": {
		location: [
			"internal/readmodel/schema.go:74",
			"internal/readmodel/schema.go:86",
			"internal/readmodel/schema.go:98",
		]
		family: "database"
		facts: {
			call: [".QueryContext", ".QueryRowContext", "sql.Open"]
		}
	}
	"internal/readmodel.tryFoldLock": {
		location: [
			"internal/readmodel/foldlock.go:80",
		]
		family: "file-write"
		facts: {
			call: ["os.OpenFile"]
		}
	}
	"internal/readmodel.writeWorkerFailedMarkerAt": {
		location: [
			"internal/readmodel/workerfailed.go:78",
			"internal/readmodel/workerfailed.go:83",
		]
		family: "file-write"
		facts: {
			call: ["os.Remove", "os.WriteFile"]
		}
	}
	"internal/retryx.RenameTransient": {
		location: [
			"internal/retryx/rename.go:24",
			"internal/retryx/rename.go:30",
		]
		family: "file-write"
		facts: {
			call: [".Do", "os.Rename"]
		}
	}
	"internal/session.LoadRegistry": {
		location: [
			"internal/session/registry.go:98",
		]
		family: "file-read"
		facts: {
			call: ["os.Open"]
		}
	}
	"internal/session.PgrepOwner.Owns": {
		location: [
			"internal/session/registry.go:184",
			"internal/session/registry.go:188",
		]
		family: "ipc-subprocess"
		facts: {
			call: ["exec.CommandContext", "exec.LookPath"]
		}
	}
	"internal/session.PingRegistry": {
		location: [
			"internal/session/registry.go:72",
			"internal/session/registry.go:81",
		]
		family: "file-write"
		facts: {
			call: ["os.MkdirAll", "os.OpenFile"]
		}
	}
	"internal/session.RegistryPath": {
		location: [
			"internal/session/registry.go:48",
		]
		family: "env"
		facts: {
			call: ["os.Getenv"]
		}
	}
	"internal/session.RewriteRegistry": {
		location: [
			"internal/session/registry.go:146",
			"internal/session/registry.go:162",
		]
		family: "file-write"
		facts: {
			call: ["os.MkdirAll", "os.WriteFile"]
		}
	}
	"internal/status.closeoutReportPath": {
		location: [
			"internal/status/sweep.go:484",
		]
		family: "file-read"
		facts: {
			call: ["filepath.Glob"]
		}
	}
	"internal/task.NewID": {
		location: [
			"internal/task/task.go:38",
		]
		family: "entropy"
		facts: {
			call: ["rand.Read"]
		}
	}
	"internal/testastguard.Parse": {
		location: [
			"internal/testastguard/testastguard.go:28",
		]
		family: "file-read"
		facts: {
			call: ["os.ReadFile"]
		}
	}
	"internal/webui.Server.Run": {
		location: [
			"internal/webui/webui.go:579",
		]
		family: "listener"
		facts: {
			call: [".ListenAndServe"]
		}
	}
	"internal/webui.attachWorkspace": {
		location: [
			"internal/webui/agentfeed.go:337",
			"internal/webui/agentfeed.go:344",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.NewRequestWithContext"]
		}
	}
	"internal/webui.crushUnixClient": {
		location: [
			"internal/webui/agentfeed.go:80",
		]
		family: "outbound"
		facts: {
			call: [".DialContext"]
		}
	}
	"internal/webui.discoverWorkspaces": {
		location: [
			"internal/webui/agentfeed.go:105",
			"internal/webui/agentfeed.go:110",
			"internal/webui/agentfeed.go:120",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.NewRequestWithContext", "io.LimitReader", "io.ReadAll"]
		}
	}
	"internal/webui.fetchWorkspaceBusy": {
		location: [
			"internal/webui/agentfeed.go:195",
			"internal/webui/agentfeed.go:200",
			"internal/webui/agentfeed.go:210",
		]
		family: "outbound"
		facts: {
			call: [".Do", "http.NewRequestWithContext", "io.LimitReader", "io.ReadAll"]
		}
	}
	"internal/webui.newNonce": {
		location: [
			"internal/webui/webui.go:481",
		]
		family: "entropy"
		facts: {
			call: ["rand.Read"]
		}
	}
	"internal/webui.staticFS": {
		location: [
			"internal/webui/static.go:9",
		]
		family: "embed"
		facts: {
			call: ["go:embed static"]
		}
	}
}
