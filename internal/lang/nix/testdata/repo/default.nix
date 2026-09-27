{ pkgs ? import <nixpkgs> { } }:
{
  hello = pkgs.callPackage ./pkgs/hello { };
  shell = import ./shell.nix { inherit pkgs; };
  lib = import ./lib;
  nested = import <nixpkgs/nixos> { };
}
