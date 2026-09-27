{ lib, stdenv, openssl, zlib, python3Packages, writeShellScriptBin }:
let
  local = writeShellScriptBin "local" "echo hi";
in
stdenv.mkDerivation rec {
  pname = "hello";
  version = "1.0";
  src = ./src;
  patches = [ ./fix.patch ];
  buildInputs = [ openssl zlib local ];
  nativeBuildInputs = [ python3Packages.setuptools (writeShellScriptBin "x" "y") ];
  postInstall = ''
    cp ${./src/hello.c} $out/share
    # import ./fake.nix is text, and so are ''${./escaped.nix} and '''quotes'''
    echo "${pname}"
  '';
  meta.description = "import ./also-fake.nix";
}
