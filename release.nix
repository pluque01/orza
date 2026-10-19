# A release checkout includes Git metadata so Go retains genuine VCS build settings.
# Invoke only on a trusted, credential-free checkout (see the release workflow).
{
  version,
  revision,
  target,
  system ? builtins.currentSystem,
}:
let
  root = toString ./.;
  flake = builtins.getFlake ("path:" + root);
  pkgs = flake.inputs.nixpkgs.legacyPackages.${system};
  lib = pkgs.lib;
  src = builtins.path {
    path = ./.;
    name = "orza-release-source";
    filter =
      path: type:
      if path == root + "/.git" || lib.hasPrefix (root + "/.git/") path then
        true
      else
        !(builtins.elem (baseNameOf path) [
          "vendor"
          "build"
          "result"
        ])
        && lib.cleanSourceFilter path type;
  };
in
assert builtins.match "[0-9]+\\.[0-9]+\\.[0-9]+" version != null;
assert builtins.match "[0-9a-f]{40}" revision != null;
assert builtins.pathExists (root + "/.git/HEAD");
flake.packages.${system}.${target}.overrideAttrs (old: {
  inherit version src;
  nativeBuildInputs = old.nativeBuildInputs ++ [ pkgs.gitMinimal ];
  postPatch = (old.postPatch or "") + ''
    test "$(git rev-parse HEAD)" = "${revision}"
  '';
  postConfigure = old.postConfigure + ''
    export GOFLAGS="$GOFLAGS -buildvcs=true"
  '';
  ldflags = [
    "-s"
    "-w"
    "-buildid="
    "-X=main.version=${version}"
  ];
})
