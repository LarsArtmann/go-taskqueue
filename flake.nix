{
  description = "Projects-aware task work queue / worker pool for Go";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts = {
      url = "github:hercules-ci/flake-parts";
      inputs.nixpkgs-lib.follows = "nixpkgs";
    };

    go-nix-helpers = {
      # github: (HTTPS) not git+ssh: keyless CI runners cannot fetch SSH inputs.
      url = "github:LarsArtmann/go-nix-helpers/master";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    inputs@{
      flake-parts,
      ...
    }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [ inputs.go-nix-helpers.flakeModules.go-standard ];

      # NixOS module for running the agent pool + read-only dashboard as
      # system services. Import into a NixOS system with:
      #   imports = [ inputs.go-taskqueue.nixosModules.default ];
      #   services.tq-agent-pool = { enable = true; user = "lars"; … };
      # (bank-sync deploy/nixos pattern; usage header in the module file)
      flake.nixosModules.default = import ./deploy/nixos/tq-agent-pool.nix;

      go-standard = {
        pname = "go-taskqueue";
        version = "0.1.0";
        vendorHash = "sha256-DCc5Liv61GG1fhcBKCMwWgxhVXN67wNIPhX4CZhbp64=";
        description = "Projects-aware task work queue: embedded SQLite journal, lease-based claims, DAG deps, DLQ, pluggable executors";
        subPackages = [ "cmd/tq" ];
        # nixpkgs 26.11 dropped x86_64-darwin; the go-standard default system
        # list still carries it and fails `nix flake check --all-systems` at
        # eval time. Pin the supported systems explicitly.
        systems = [
          "x86_64-linux"
          "aarch64-linux"
          "aarch64-darwin"
        ];
        # templ fmt gates the .templ sources in the treefmt check (the
        # generated *_templ.go output is excluded below — it is not
        # gofumpt/goimports-clean and regenerating would undo any rewrite).
        enableTempl = true;

        extraBuildAttrs = {
          env = {
            CGO_ENABLED = "0";
            # go-sse uses encoding/json/v2 (Go 1.26 default experiment; nixpkgs
            # builds the toolchain without it enabled).
            GOEXPERIMENT = "jsonv2";
          };
          preBuild = ''
            export HOME=$TMPDIR
          '';
          # `tq version` reports the release, not "dev" (round-5 M25/F133).
          # Keep in sync with go-standard.version above.
          buildFlagsArray = [ "-ldflags=-X main.version=0.1.0" ];
        };

        shellExtraEnv = {
          CGO_ENABLED = "0";
          GOEXPERIMENT = "jsonv2";
        };

        devShellExtraPackages = pkgs: [
          pkgs.git
          pkgs.gotools
          pkgs.gofumpt
          # dprint is available but deliberately NOT gated in treefmt: docs
          # formatting stays manual. Autoformatting *.md would rewrap the
          # machine-consumed TODO_LIST.md (one checkbox item per line — the
          # harvester's contract) and mass-rewrite the docs baseline.
          pkgs.dprint
          pkgs.tailwindcss_4
        ];
      };

      perSystem =
        {
          config,
          pkgs,
          lib,
          ...
        }:
        {
          # Generated templ output never satisfies gofumpt/goimports; the
          # .templ sources carry the formatting contract via templ fmt.
          treefmt.settings.excludes = [ "*_templ.go" ];

          # Fast vendorHash drift gate: realizes ONLY the go-modules FOD so a
          # go.mod/go.sum change fails in seconds with the hash mismatch.
          checks.vendor-hash = pkgs.runCommand "vendor-hash" { } ''
            echo "vendor hash verified: ${config.packages.default.goModules}"
            touch $out
          '';

          # Swallowed-build guard (18:41/19:33 reports f4): a green build can
          # still produce an empty store path (the GOEXPERIMENT=jsonv2 failure
          # mode), so execute the nix-built binary and require non-empty
          # --help output. References the package directly — not the `result`
          # symlink — so `nix flake check` covers it in CI and sandboxes.
          checks.binary-runs = pkgs.runCommand "binary-runs" { } ''
            ${config.packages.default}/bin/tq --help > help.txt
            if [ ! -s help.txt ]; then
              echo "error: nix-built tq binary produced empty --help output" >&2
              exit 1
            fi
            cp help.txt $out
          '';

          # Module eval check (ROUND8 A6, bank-sync nixos-module-eval
          # pattern): instantiates the NixOS module so option typos /
          # rendering regressions fail `nix flake check`. Evals BOTH the
          # default /var/lib path AND the deployment shape (login user +
          # pool dbPath + poolSettings + serve) — an option branch that is
          # never evaluated is untested code. Linux only (needs nixpkgs
          # nixosSystem).
          checks.module-eval = lib.mkIf pkgs.stdenv.hostPlatform.isLinux (
            pkgs.runCommand "nixos-module-eval" { } (
              let
                inherit (inputs) nixpkgs;
                nixosModule = import ./deploy/nixos/tq-agent-pool.nix;
                evalFull =
                  extra:
                  (nixpkgs.lib.nixosSystem {
                    system = pkgs.stdenv.hostPlatform.system;
                    modules = [
                      {
                        nixpkgs.overlays = [
                          (_final: _prev: {
                            tq = config.packages.default;
                          })
                        ];
                      }
                      nixosModule
                      { services.tq-agent-pool.enable = true; }
                      extra
                    ];
                  }).config;
                eval = extra: (evalFull extra).systemd.services;
                defaultUnits = eval { };
                # Third branch: an unknown poolSettings key must flow into the
                # rendered pool.conf (NixOS cannot know tq's flag set — the
                # LOUD rejection is tq's applyPoolConfigFile at unit start,
                # which this check's runtime smoke exercises via the binary's
                # own config parsing). If the key silently VANISHED instead,
                # typos would run with defaults — the exact failure this
                # branch guards against.
                unknownKeyConfig = evalFull {
                  services.tq-agent-pool = {
                    poolSettings = {
                      projects-dir = "/home/alice/projects";
                      typo-key-that-tq-rejects = "true";
                    };
                  };
                };
                # The rendered config is a store file — pure eval cannot
                # read it; the check script greps it at build time.
                unknownKeyConfPath = unknownKeyConfig.services.tq-agent-pool.renderedConfigFile;
                tokenUnits = eval {
                  services.tq-agent-pool = {
                    serve = {
                      enable = true;
                      addr = "127.0.0.1:8100";
                      authTokenFile = "/run/tq-token";
                    };
                  };
                };
                deployedUnits = eval {
                  services.tq-agent-pool = {
                    user = "alice";
                    group = "users";
                    dbPath = "/mnt/pool/services/tq/tq.db";
                    poolSettings = {
                      projects-dir = "/home/alice/projects";
                      yolo = "true";
                    };
                    # systemd argv: flag and value are SEPARATE elements —
                    # one glued "--max-per-tick 3" string would quote into a
                    # single argument and fail flag parsing at start.
                    extraArgs = [
                      "--max-per-tick"
                      "3"
                    ];
                    serve = {
                      enable = true;
                      addr = "127.0.0.1:8100";
                    };
                  };
                };
                defaultPool = defaultUnits.tq-agent-pool;
                deployedPool = deployedUnits.tq-agent-pool;
                deployedServe = deployedUnits.tq-serve;
                tokenServe = tokenUnits.tq-serve;
                drainInvariants =
                  unit:
                  builtins.all (kv: kv != null) [
                    unit.serviceConfig.KillSignal or null
                    unit.serviceConfig.KillMode or null
                    unit.serviceConfig.TimeoutStopSec or null
                  ];
                # Pin the exact binary token (2026-09-08): a lib.getExe pname
                # fallback (e.g. .../bin/go-taskqueue) silently renders
                # ExecStarts that fragment regexes still matched — the pool's
                # first token must be the package's own /bin/tq, and the serve
                # line must equal its full expected string exactly.
                expectedBin = "${config.packages.default}/bin/tq";
                poolFirstToken = builtins.head (lib.splitString " " deployedPool.serviceConfig.ExecStart);
                allOk =
                  # default path: synthetic user + StateDirectory, no mount gate
                  defaultPool.serviceConfig.StateDirectory or "" == "tq"
                  && !(defaultPool.unitConfig ? RequiresMountsFor)
                  # deployment path: pool user, mount gate, config file wired
                  && deployedPool.serviceConfig.User == "alice"
                  && deployedPool.unitConfig.RequiresMountsFor == [ "/mnt/pool/services/tq" ]
                  # exact binary + config file wired (store paths render as
                  # <hash>-tq-pool.conf — dash, not slash)
                  && poolFirstToken == expectedBin
                  && builtins.match ".*--config .*tq-pool\\.conf.*" deployedPool.serviceConfig.ExecStart != null
                  && builtins.elem "TQ_DB=/mnt/pool/services/tq/tq.db" deployedPool.serviceConfig.Environment
                  # serve unit exists with the addr + no StateDirectory branch
                  && deployedServe.serviceConfig != { }
                  && deployedServe.serviceConfig.ExecStart == "${expectedBin} serve --addr 127.0.0.1:8100"
                  && !(deployedServe.serviceConfig ? StateDirectory)
                  # unknown poolSettings keys must SURVIVE rendering (tq
                  # rejects them loudly at unit start; vanishing = silent
                  # defaults) — checked by the grep below — and the
                  # extraArgs example renders as two argv tokens
                  &&
                    builtins.match ".*--config .*tq-pool\.conf.* --max-per-tick 3.*" deployedPool.serviceConfig.ExecStart
                    != null
                  # authTokenFile wires EnvironmentFile on the serve unit
                  && tokenServe.serviceConfig.EnvironmentFile == [ "/run/tq-token" ]
                  # drain invariants survive on both units
                  && drainInvariants deployedPool
                  && (deployedPool.serviceConfig.KillSignal or "" == "SIGINT")
                  && (deployedPool.serviceConfig.KillMode or "" == "process")
                  && (deployedPool.serviceConfig.TimeoutStopSec or "" == "45min");
              in
              ''
                echo "pool ExecStart: ${deployedPool.serviceConfig.ExecStart}"
                echo "serve ExecStart: ${deployedServe.serviceConfig.ExecStart}"
                if ! grep -q 'typo-key-that-tq-rejects' '${unknownKeyConfPath}'; then
                  echo 'nixos-module-eval FAILED: unknown poolSettings key vanished from the rendered pool.conf (typos would run with silent defaults)'
                  exit 1
                fi
                echo "assertions: ${
                  builtins.toJSON {
                    defaultStateDirectory = defaultPool.serviceConfig.StateDirectory or null;
                    defaultRequiresMountsFor = defaultPool.unitConfig ? RequiresMountsFor;
                    deployedUser = deployedPool.serviceConfig.User or null;
                    deployedRequiresMountsFor = deployedPool.unitConfig.RequiresMountsFor or null;
                    deployedEnvironment = deployedPool.serviceConfig.Environment or null;
                    inherit expectedBin;
                    inherit poolFirstToken;
                    serveExecStart = deployedServe.serviceConfig.ExecStart or null;
                    serveStateDirectoryPresent = deployedServe.serviceConfig ? StateDirectory;
                    killSignal = deployedPool.serviceConfig.KillSignal or null;
                    killMode = deployedPool.serviceConfig.KillMode or null;
                    timeoutStopSec = deployedPool.serviceConfig.TimeoutStopSec or null;
                  }
                }"
                ${lib.optionalString allOk "touch $out"}
                ${lib.optionalString (!allOk) "echo 'nixos-module-eval FAILED'; exit 1"}
              ''
            )
          );

          # Recompile the web UI stylesheet into the committed, embedded
          # static asset (dev step — the nix build just embeds the output).
          apps.webui-css = {
            type = "app";
            meta.description = "Recompile internal/webui/static/app.css via tailwindcss --minify";
            program = pkgs.writeShellApplication {
              name = "webui-css";
              runtimeInputs = [
                pkgs.tailwindcss_4
                pkgs.go
              ];
              text = ''
                exec bash scripts/build-webui-css.sh
              '';
            };
          };
        };
    };
}
