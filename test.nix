{ pkgs ? import <nixpkgs> {} }:
let
  matomo4nix = pkgs.callPackage ./. {};
in
  (with matomo4nix.plugins; [
    Chat
    LoginLdap
    ServerMonitor
  ])
  ++ (with matomo4nix.themes; [
    DarkTheme
    ClassicTheme
  ])
