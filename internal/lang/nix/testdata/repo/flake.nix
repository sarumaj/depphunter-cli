{
  description = "shop: a flake with every kind of input";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-24.05";
    flake-utils.url = "github:numtide/flake-utils";
    home-manager = {
      url = "github:nix-community/home-manager/release-24.05";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    foo = {
      url = "git+https://git.example.org/team/foo?ref=main&rev=0123456789abcdef0123456789abcdef01234567";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    bar.url = "path:./sub";
    baz = {
      url = "https://example.org/downloads/baz-1.2.tar.gz";
      flake = false;
    };
    tagged.url = "gitlab:acme/tools/v1.4.0";
    registry.url = "flake:nixpkgs/nixos-unstable";
    typed = {
      type = "github";
      owner = "acme";
      repo = "typed";
      ref = "refs/heads/dev";
    };
    same.follows = "nixpkgs";
  };
  inputs.hub.url = "https://flakehub.com/f/NixOS/nixpkgs/0.1.*.tar.gz";

  outputs = { self, nixpkgs, flake-utils, home-manager, systems, ... }@inputs:
    let
      overlay = import ./overlays;
    in
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system};
      in {
        packages.default = pkgs.callPackage ./pkgs/hello { };
        packages.tool = pkgs.hello;
        devShells.default = import ./shell.nix { inherit pkgs; };
      })
    // {
      nixosModules.default = import ./modules;
      overlays.default = overlay;
      nixosConfigurations = {
        box = nixpkgs.lib.nixosSystem {
          modules = [ ./modules home-manager.nixosModules.home-manager ];
          specialArgs = { inherit inputs; };
        };
      };
      lib = import ./lib;
    };
}
