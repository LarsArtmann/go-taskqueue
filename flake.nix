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
        vendorHash = "sha256-H2J6GJy6BTngW7A6qaYHVEHxA6wLGoquDa+kW+EXmjs=";
        description = "Projects-aware task work queue: embedded SQLite journal, lease-based claims, DAG deps, DLQ, pluggable executors";
        subPackages = [ "cmd/tq" ];

        extraBuildAttrs = {
          env = {
            CGO_ENABLED = "0";
          };
          preBuild = ''
            export HOME=$TMPDIR
          '';
        };

        shellExtraEnv = {
          CGO_ENABLED = "0";
        };

        devShellExtraPackages = pkgs: [
          pkgs.git
          pkgs.gotools
          pkgs.gofumpt
          pkgs.dprint
          pkgs.templ
        ];
      };

      perSystem =
        { config, pkgs, ... }:
        {
          # Fast vendorHash drift gate: realizes ONLY the go-modules FOD so a
          # go.mod/go.sum change fails in seconds with the hash mismatch.
          checks.vendor-hash = pkgs.runCommand "vendor-hash" { } ''
            echo "vendor hash verified: ${config.packages.default.goModules}"
            touch $out
          '';
        };
    };
}
