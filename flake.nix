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
        vendorHash = "sha256-xbSEDxrY54nC+QIzsgnB77XEs1q9+FLw79H5x9PZ/eY=";
        description = "Projects-aware task work queue: embedded SQLite journal, lease-based claims, DAG deps, DLQ, pluggable executors";
        subPackages = [ "cmd/tq" ];
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
        };

        shellExtraEnv = {
          CGO_ENABLED = "0";
          GOEXPERIMENT = "jsonv2";
        };

        devShellExtraPackages = pkgs: [
          pkgs.git
          pkgs.gotools
          pkgs.gofumpt
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
