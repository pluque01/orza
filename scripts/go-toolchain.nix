# Use the compiler recipe from goNixpkgs with the project's platform-compatible
# base package set (new nixpkgs releases may drop a supported host platform).
{
  nixpkgs,
  goNixpkgs,
  goVersion,
  system,
}:
let
  pkgs = nixpkgs.legacyPackages.${system};
  minor = nixpkgs.lib.versions.majorMinor goVersion;
  builder = "buildGo" + builtins.replaceStrings [ "." ] [ "" ] minor + "Module";
  go = pkgs.callPackage (goNixpkgs + "/pkgs/development/compilers/go/${minor}.nix") {
    ${builder} = pkgs.buildGoModule.override { inherit go; };
  };
in
go
