{ pkgs }:
pkgs.mkShell {
  packages = with pkgs.python3Packages; [ requests pkgs.jq ];
  nativeBuildInputs = [ pkgs.legacyPackages.x86_64-linux.hello ];
}
