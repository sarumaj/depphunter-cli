{ config, pkgs, lib, inputs, ... }:
{
  imports = [ ./services.nix ./users ] ++ [ ../lib/options.nix ];
  environment.systemPackages = with pkgs; [ vim git ];
  home.packages = [ pkgs.htop ];
  services.foo.package = inputs.foo.packages.x86_64-linux.default;
  users.motd = builtins.readFile ../lib/VERSION;
}
