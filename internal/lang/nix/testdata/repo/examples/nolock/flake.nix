# No flake.lock beside it: every input is read from its reference alone.
{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
  inputs.commit.url = "github:acme/commit/0123456789abcdef0123456789abcdef01234567";
  inputs.tag.url = "github:acme/tag/v2.0.1";
  inputs.hashed.url = "https://example.org/hashed.tar.gz?narHash=sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA%3D";
  inputs.ssh.url = "git+ssh://git@github.com/Acme/Private.git?ref=refs/heads/main";
  inputs.local.url = "path:../../sub";
  inputs.alias.follows = "nixpkgs";
  inputs.nested.follows = "commit/nixpkgs";
  inputs.srht.url = "sourcehut:~user/repo";
  inputs.utils = { };

  outputs = { self, nixpkgs, ... }: {
    devShells.x86_64-linux.default = import ./shell.nix { pkgs = nixpkgs.legacyPackages.x86_64-linux; };
  };
}
