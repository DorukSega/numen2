{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    nodejs
    vsce
  ];

  shellHook = ''
    echo "run vsce package"
  '';
}
