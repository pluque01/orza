{
  nixpkgs,
  goNixpkgs,
  goVersion,
  system,
}:
let
  pkgs = nixpkgs.legacyPackages.${system};
  minor = nixpkgs.lib.versions.majorMinor goVersion;
  versionSuffix = builtins.replaceStrings [ "." ] [ "" ] minor;
  builder = "buildGo" + versionSuffix + "Module";
  go = pkgs.callPackage (goNixpkgs + "/pkgs/development/compilers/go/${minor}.nix") {
    ${builder} = pkgs.buildGoModule.override { inherit go; };
  };
in
# The hosted CI runners can substitute the pinned nixpkgs compiler directly.
# Keep the platform-compatible recipe for other hosts.
if system == "x86_64-linux" then
  goNixpkgs.legacyPackages.${system}."go_${builtins.replaceStrings [ "." ] [ "_" ] minor}"
else
  go
