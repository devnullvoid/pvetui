{
  description = "A Terminal User Interface for Proxmox Virtual Environment";

  inputs = {
    # Nixpkgs 26.11 drops Intel macOS, which this flake still supports.
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        go = pkgs.go_1_26;
        buildGoModule = pkgs.buildGoModule.override { inherit go; };

        pvetui = buildGoModule {
          pname = "pvetui";
          version = if (self ? rev) then self.shortRev else "dev";

          src = self;

          vendorHash = "sha256-Hin3jd2bmxBD7gR6eksAYpHWG38QlPE5M5bEmhVYP/o=";

          subPackages = [ "cmd/pvetui" ];

          ldflags = [
            "-X github.com/devnullvoid/pvetui/internal/version.version=${if (self ? rev) then self.shortRev else "dev"}"
            "-X github.com/devnullvoid/pvetui/internal/version.buildDate=1970-01-01T00:00:00Z"
            "-X github.com/devnullvoid/pvetui/internal/version.commit=${if (self ? rev) then self.rev else "unknown"}"
          ];

          meta = with pkgs.lib; {
            description = "A Terminal User Interface for Proxmox Virtual Environment";
            homepage = "https://github.com/devnullvoid/pvetui";
            license = licenses.mit;
            maintainers = [];
            mainProgram = "pvetui";
          };
        };
      in
      {
        packages = {
          default = pvetui;
          pvetui = pvetui;
        };

        apps.default = {
          type = "app";
          program = "${pvetui}/bin/pvetui";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = [
            go
            pkgs.gopls
            pkgs.gotools
            pkgs.golangci-lint
          ];

          shellHook = ''
            echo "pvetui development environment"
            echo "Go version: $(go version)"
          '';
        };
      }
    );
}
