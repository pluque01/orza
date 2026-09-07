{
  description = "A terminal-native SSH client with a TUI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/8b6d0ac8759bbc2499386ef5bcd7f11ee7f7ff52";
    goNixpkgs.url = "github:NixOS/nixpkgs/a5d32f8c86e7ca9388fd8b75758168f30174b218";
    vulndb = {
      url = "github:golang/vulndb/4a2cb55ee69f6a16c07b5b551676eca8f2065019";
      flake = false;
    };
  };

  outputs =
    {
      self,
      goNixpkgs,
      nixpkgs,
      vulndb,
      ...
    }:
    let
      version = "0.1.0";
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
      mkVulnerabilityDatabase =
        pkgs:
        let
          vulndbIndexer = pkgs.buildGo126Module {
            pname = "vulndb-indexer";
            version = "unstable";
            src = vulndb;
            vendorHash = "sha256-cHjJPluvGtuau9+qKTuT7PLfU1Iz56oG7eIYwHbdkY8=";
            subPackages = [ "cmd/indexdb" ];
            doCheck = false;
          };
        in
        pkgs.runCommand "go-vulnerability-database"
          {
            nativeBuildInputs = [ vulndbIndexer ];
          }
          ''
            indexdb -vulns ${vulndb}/data/osv -out $out
          '';
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          go = goNixpkgs.legacyPackages.${system}.go_1_26;
          source = nixpkgs.lib.cleanSource ./.;
          mkRelease =
            {
              goos,
              goarch,
              cgoEnabled ? false,
            }:
            let
              targetGo =
                if cgoEnabled then
                  go
                else
                  pkgs.symlinkJoin {
                    name = "go-1.26-${goos}-${goarch}";
                    paths = [ go ];
                    passthru = {
                      GOOS = goos;
                      GOARCH = goarch;
                      CGO_ENABLED = 0;
                    };
                    meta = go.meta;
                  };
              buildGoModule = pkgs.buildGo126Module.override { go = targetGo; };
            in
            buildGoModule {
              pname = "orza-${goos}-${goarch}";
              inherit version;
              src = source;
              vendorHash = null;
              subPackages = [ "cmd/orza" ];
              doCheck = false;
              ldflags = [
                "-s"
                "-w"
                "-X=main.version=${version}"
              ];
              postInstall = ''
                targetDir=$out/bin/${goos}_${goarch}
                if [ -d "$targetDir" ]; then
                  mv "$targetDir"/* $out/bin/
                  rmdir "$targetDir"
                fi
              '';
            };
          pureRelease = goos: goarch: mkRelease { inherit goos goarch; };
          nativeRelease =
            if pkgs.stdenv.hostPlatform.isDarwin then
              mkRelease {
                goos = "darwin";
                goarch = if pkgs.stdenv.hostPlatform.isAarch64 then "arm64" else "amd64";
                cgoEnabled = true;
              }
            else
              pureRelease "linux" (if pkgs.stdenv.hostPlatform.isAarch64 then "arm64" else "amd64");
        in
        {
          default = nativeRelease;
          linux-amd64 = pureRelease "linux" "amd64";
          linux-arm64 = pureRelease "linux" "arm64";
          windows-amd64 = pureRelease "windows" "amd64";
          windows-arm64 = pureRelease "windows" "arm64";
        }
        // nixpkgs.lib.optionalAttrs (system == "x86_64-darwin") {
          darwin-amd64 = nativeRelease;
        }
        // nixpkgs.lib.optionalAttrs (system == "aarch64-darwin") {
          darwin-arm64 = nativeRelease;
        }
      );

      checks = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          go = goNixpkgs.legacyPackages.${system}.go_1_26;
          source = nixpkgs.lib.cleanSource ./.;
          vulnerabilityDatabase = mkVulnerabilityDatabase pkgs;
          mkGoCheck =
            name: nativeBuildInputs: command:
            pkgs.runCommand "orza-${name}"
              {
                nativeBuildInputs = [ go ] ++ nativeBuildInputs;
              }
              ''
                cp -R ${source} source
                chmod -R u+w source
                cd source
                export HOME=$TMPDIR
                export GOCACHE=$TMPDIR/go-cache
                export GOTOOLCHAIN=local
                export GOFLAGS="-mod=vendor -trimpath"
                ${command}
                touch $out
              '';
          nativeCgo = if pkgs.stdenv.hostPlatform.isDarwin then "1" else "0";
        in
        {
          build = self.packages.${system}.default;
          formatting = mkGoCheck "formatting" [ pkgs.nixfmt ] ''
            files=$(gofmt -l cmd internal tests)
            test -z "$files"
            nixfmt --check flake.nix
          '';
          go-test = mkGoCheck "go-test" [ ] ''
            CGO_ENABLED=${nativeCgo} go test ./...
          '';
          race = mkGoCheck "race" [ pkgs.stdenv.cc ] ''
            CGO_ENABLED=1 go test -race ./...
          '';
          vet = mkGoCheck "vet" [ ] ''
            CGO_ENABLED=${nativeCgo} go vet ./...
          '';
          staticcheck = mkGoCheck "staticcheck" [ pkgs.go-tools ] ''
            CGO_ENABLED=${nativeCgo} staticcheck ./...
          '';
          govulncheck = mkGoCheck "govulncheck" [ pkgs.govulncheck ] ''
            CGO_ENABLED=${nativeCgo} ORZA_VULNDB=${vulnerabilityDatabase} bash scripts/check-vulnerabilities.sh pinned
          '';
          automation =
            pkgs.runCommand "orza-automation"
              {
                nativeBuildInputs = [
                  pkgs.actionlint
                  pkgs.bash
                  pkgs.coreutils
                  pkgs.findutils
                  pkgs.gitMinimal
                  pkgs.gnutar
                  pkgs.gzip
                  pkgs.jq
                  pkgs.shellcheck
                  pkgs.unzip
                  pkgs.zip
                ];
              }
              ''
                cp -R ${source} source
                chmod -R u+w source
                cd source
                patchShebangs scripts
                actionlint .github/workflows/*.yml
                shellcheck scripts/*.sh
                for test_script in scripts/*_test.sh; do
                  bash "$test_script"
                done
                touch $out
              '';
          native-smoke = pkgs.runCommand "orza-native-smoke" { } ''
            export HOME=$TMPDIR
            export XDG_DATA_HOME=$TMPDIR/data
            test "$(${self.packages.${system}.default}/bin/orza --version)" = "orza ${version}"
            touch $out
          '';
        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
          go = goNixpkgs.legacyPackages.${system}.go_1_26;
          vulnerabilityDatabase = mkVulnerabilityDatabase pkgs;
        in
        {
          default = pkgs.mkShell {
            packages = [
              go
              pkgs.go-tools
              pkgs.govulncheck
              pkgs.nixfmt-tree
              pkgs.gitleaks
              pkgs.actionlint
              pkgs.shellcheck
              pkgs.renovate
              pkgs.libarchive
              pkgs.coreutils
              pkgs.gnutar
              pkgs.gzip
              pkgs.zip
              pkgs.unzip
            ];
            GOTOOLCHAIN = "local";
            ORZA_VULNDB = vulnerabilityDatabase;
          };
        }
      );

      formatter = forAllSystems (system: nixpkgs.legacyPackages.${system}.nixfmt-tree);
    };
}
