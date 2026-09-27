{ pkgs ? import <nixpkgs> { } }:
pkgs.mkShell {
  packages = [ pkgs.git pkgs.nodejs_20 ];
  buildInputs = with pkgs; [ openssl zlib ] ++ pkgs.lib.optionals pkgs.stdenv.isLinux [ pkgs.systemd ];
  nativeBuildInputs = [ pkgs.cmake (pkgs.python3.withPackages (ps: [ ps.requests ])) ];
  shellHook = ''
    echo "import ./not-a-file.nix"
    source ${./scripts/env.sh}
  '';
}
