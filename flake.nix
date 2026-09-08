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

      go-standard = {
        pname = "go-taskqueue";
        version = "0.1.0";
        vendorHash = "sha256-iofA9U48LWcMVfgVC2RCy1srV+Qdb9W6kS2huc6rEJo=";
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
        { config, pkgs, ... }:
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
