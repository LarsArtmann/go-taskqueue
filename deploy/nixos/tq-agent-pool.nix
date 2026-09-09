# NixOS module for the tq agent pool (go-taskqueue) + read-only dashboard.
#
# Usage:
#   imports = [ inputs.go-taskqueue.nixosModules.default ];
#   services.tq-agent-pool = {
#     enable = true;
#     user = "lars";                     # a login user with the repos,
#                                         # git identity + crush config
#     dbPath = "/mnt/pool/services/tq/tq.db";
#     poolSettings = {
#       projects-dir = "/home/lars/projects";
#       yolo = "true";
#     };
#     serve.enable = true;               # tq serve dashboard
#   };
#
# Invariants that MUST survive any edit (mirrors deploy/systemd and
# cmd/tq/bootstrap.go's unit template):
#   - KillSignal=SIGINT, KillMode=process, TimeoutStopSec=45min: in-flight
#     agents finish and record their outcome; a shared drain deadline
#     strands tasks in `running` forever (AGENTS.md invariant).
#   - ProtectSystem=full (NOT strict) and NO ProtectHome restriction: the
#     pool execs headless agents that write and commit inside $HOME repos
#     and talk to the network. Sandbox harder and the pool is dead.
#   - The pool unit carries an explicit agent-toolchain PATH (agentPath):
#     systemd's default service PATH has no git/go/crush, so every scan,
#     preflight, agent exec and verify gate would fail without it.
#   - Restart=on-failure + RestartSec=30s, never Restart=always: a
#     config error (unknown pool.conf key) must rate-limit, not loop.
#
# pool.conf is a flat `key = value` file using the agent-pool FLAG names
# (projects-dir, interval, concurrency, …). Precedence: CLI flag > env >
# file. Unknown keys are a hard error at pool start (fail loudly).
# `services.tq-agent-pool.extraArgs` flags always win over poolSettings.
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.services.tq-agent-pool;

  # Flat key=value pool.conf, bootstrap's renderPoolConfig style. Keys
  # sorted for deterministic store paths.
  poolConf = pkgs.writeText "tq-pool.conf" (
    lib.concatMapStringsSep "\n" (k: "${k} = ${toString cfg.poolSettings.${k}}") (
      lib.naturalSort (lib.attrNames cfg.poolSettings)
    )
  );

  agentHome = config.users.users.${cfg.user}.home or "/home/${cfg.user}";

  # Agents exec git (dirty preflight + commits), go (.tq-verify gates)
  # and crush — none of which are on systemd's default service PATH.
  defaultAgentPath = lib.concatStringsSep ":" [
    (lib.makeBinPath [
      pkgs.git
      pkgs.go
    ])
    "/run/current-system/sw/bin"
    "/etc/profiles/per-user/${cfg.user}/bin"
    "${agentHome}/go/bin"
  ];
in
{
  options.services.tq-agent-pool = {
    enable = lib.mkEnableOption "tq agent pool (self-managing TODO_LIST harvest + headless crush agents)";

    package = lib.mkPackageOption pkgs "tq" { };

    user = lib.mkOption {
      type = lib.types.str;
      default = "tq";
      description = ''
        User the pool and dashboard run as. The realistic deployment is a
        login user (e.g. "lars"): agents write/commit inside that user's
        repos and need their git identity, ssh credentials and crush
        config. The synthetic "tq" default user is created automatically;
        any other value must already exist.
      '';
    };

    group = lib.mkOption {
      type = lib.types.str;
      default = "tq";
      description = "Group for the pool and dashboard units.";
    };

    agentPath = lib.mkOption {
      type = lib.types.str;
      default = defaultAgentPath;
      description = ''
        PATH for the pool unit. Agents exec git (dirty preflight,
        commits), go (.tq-verify gates), crush (the agents themselves)
        and repo tooling (templ, nix) — none of which are on systemd's
        default service PATH. Default: hermetic git+go, then the system
        profile, the per-user profile and the user's GOBIN.
      '';
    };

    dbPath = lib.mkOption {
      type = lib.types.path;
      default = "/var/lib/tq/tq.db";
      description = ''
        Path to the SQLite task journal shared by the pool and the
        dashboard. May point at a mounted pool (e.g.
        /mnt/pool/services/tq/tq.db) — both units then require that mount
        and skip the /var/lib StateDirectory.
      '';
    };

    poolSettings = lib.mkOption {
      type = lib.types.attrsOf lib.types.str;
      default = { };
      description = ''
        Settings for the generated pool.conf (flat key = value, flag
        spellings: projects-dir, repos, interval, concurrency, yolo,
        project-exclusive, daily-budget, max-per-tick, task-timeout,
        repo-interval, dlq-backoff, review, alert-url, log-dir,
        log-dir-max-age, log-dir-max-bytes, …).
        Precedence: extraArgs flags > environment > this file.
      '';
      example = {
        projects-dir = "/home/lars/projects";
        repos = "CV,SystemNix";
        yolo = "true";
      };
    };

    extraArgs = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ ];
      description = "Extra agent-pool CLI flags (win over poolSettings).";
      example = [ "--once" ];
    };

    renderedConfigFile = lib.mkOption {
      type = lib.types.path;
      readOnly = true;
      description = "The rendered pool.conf store path (for custom units or checks).";
    };

    serve = {
      enable = lib.mkEnableOption "the tq serve read-only dashboard";

      addr = lib.mkOption {
        type = lib.types.str;
        default = "127.0.0.1:8090";
        description = ''
          Listen address for the dashboard. tq serve refuses non-loopback
          binds without an auth token; front it with a reverse proxy (SSO)
          instead of exposing it directly.
        '';
      };

      authTokenFile = lib.mkOption {
        type = lib.types.nullOr lib.types.path;
        default = null;
        description = ''
          Optional EnvironmentFile providing TQ_SERVE_TOKEN=... Required
          for non-loopback --addr (KEY=VALUE lines, not a raw token).
        '';
      };
    };
  };

  config = lib.mkIf cfg.enable {
    services.tq-agent-pool.renderedConfigFile = poolConf;

    users.users.tq = lib.mkIf (cfg.user == "tq") {
      isSystemUser = true;
      inherit (cfg) group;
      home = dirOf (toString cfg.dbPath);
      createHome = false;
    };
    users.groups.tq = lib.mkIf (cfg.group == "tq") { };

    systemd.services.tq-agent-pool = {
      description = "tq agent-pool (self-managing TODO_LIST harvest + headless agents)";
      documentation = [ "https://github.com/larsartmann/go-taskqueue" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];

      # When dbPath is outside /var/lib (e.g. on a mounted pool), fail
      # loudly if the mount is missing instead of journaling onto the root
      # filesystem under the mountpoint. Mirrors the bank-sync module.
      unitConfig = lib.optionalAttrs (!(lib.hasPrefix "/var/lib/" (toString cfg.dbPath))) {
        RequiresMountsFor = [ (dirOf (toString cfg.dbPath)) ];
      };

      serviceConfig = {
        Type = "simple";
        ExecStart = "${lib.getExe' cfg.package "tq"} agent-pool --config ${poolConf} ${lib.escapeShellArgs cfg.extraArgs}";
        Environment = [
          "TQ_DB=${toString cfg.dbPath}"
          "PATH=${cfg.agentPath}"
        ];
        User = cfg.user;
        Group = cfg.group;
        WorkingDirectory = dirOf (toString cfg.dbPath);
        Restart = "on-failure";
        RestartSec = "30s";
        startLimitBurst = 5;
        startLimitIntervalSec = 300;

        # Graceful drain: agents may run for a long time; give them a
        # wide stop window. The pool survives the first SIGINT and
        # finishes in-flight tasks (see the header invariants).
        TimeoutStopSec = "45min";
        KillSignal = "SIGINT";
        KillMode = "process";

        # Deliberately conservative: the pool execs headless agents that
        # write/commit inside $HOME and talk to the network, so no
        # sandboxing below may break either. NoNewPrivileges and the
        # kernel/cgroup namespaces cost nothing; ProtectSystem=full keeps
        # /usr,/boot,/etc read-only while leaving $HOME writable for the
        # repos and the binary.
        NoNewPrivileges = true;
        ProtectSystem = "full";
        ProtectControlGroups = true;
        ProtectKernelModules = true;
        ProtectKernelTunables = true;
        ProtectKernelLogs = true;
        RestrictSUIDSGID = true;
        LockPersonality = true;
      }
      // lib.optionalAttrs (lib.hasPrefix "/var/lib/" (toString cfg.dbPath)) {
        StateDirectory = "tq";
      };
    };

    systemd.services.tq-serve = lib.mkIf cfg.serve.enable {
      description = "tq serve (read-only task-queue dashboard)";
      documentation = [ "https://github.com/larsartmann/go-taskqueue" ];
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];

      unitConfig = lib.optionalAttrs (!(lib.hasPrefix "/var/lib/" (toString cfg.dbPath))) {
        RequiresMountsFor = [ (dirOf (toString cfg.dbPath)) ];
      };

      serviceConfig = {
        Type = "simple";
        ExecStart = "${lib.getExe' cfg.package "tq"} serve --addr ${cfg.serve.addr}";
        Environment = [ "TQ_DB=${toString cfg.dbPath}" ];
        EnvironmentFile = lib.optional (cfg.serve.authTokenFile != null) cfg.serve.authTokenFile;
        User = cfg.user;
        Group = cfg.group;
        WorkingDirectory = dirOf (toString cfg.dbPath);
        Restart = "on-failure";
        RestartSec = "5s";
        startLimitBurst = 5;
        startLimitIntervalSec = 300;

        # The dashboard is a pure projection (ADR-0003): read-only DB
        # access apart from the SQLite WAL/SHM siblings, which require
        # write access to the DB directory even for readers.
        NoNewPrivileges = true;
        ProtectSystem = "full";
        PrivateTmp = true;
        ReadWritePaths = [ (dirOf (toString cfg.dbPath)) ];
      }
      // lib.optionalAttrs (lib.hasPrefix "/var/lib/" (toString cfg.dbPath)) {
        StateDirectory = "tq";
      };
    };
  };
}
