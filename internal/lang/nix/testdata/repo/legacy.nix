let
  sources = import ./nix/sources.nix;
  pkgs = import sources.nixpkgs { };
  pins = import ./npins;
  pinned = builtins.fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/0123456789abcdef0123456789abcdef01234567.tar.gz";
    sha256 = "0000000000000000000000000000000000000000000000000000";
  };
  git = builtins.fetchGit { url = "https://github.com/acme/tool.git"; ref = "main"; };
  chan = fetchTarball "https://nixos.org/channels/nixos-24.05/nixexprs.tar.xz";
  flake = builtins.getFlake "github:numtide/flake-utils/v1.0.0";
in
{
  inherit pkgs;
  hm = import sources.home-manager { };
  other = import pins.nixpkgs { };
  src = pkgs.fetchgit { url = "https://example.org/x.git"; rev = "abc"; };
}
