{
  description = "A Terminal User Interface for Proxmox Virtual Environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        novnc = pkgs.fetchFromGitHub {
          owner = "novnc";
          repo = "noVNC";
          rev = "4cb5aa45ae559f8fa85fe2b424abbc6ef6d4c6f9";
          hash = "sha256-14LN0Pr6xKck4PvRYHwTOLTO68WKMey+Z0LWRbkyWb8=";
        };

        pvetui = pkgs.buildGoModule {
          pname = "pvetui";
          version = if (self ? rev) then self.shortRev else "dev";

          src = ./.;

          postUnpack = ''
              rm -rf $sourceRoot/internal/vnc/novnc
              cp -r ${novnc} $sourceRoot/internal/vnc/novnc
              chmod -R u+w $sourceRoot/internal/vnc/novnc
          '';

          vendorHash = "sha256-quGKUBmX4ebrykhWRnp71yYt/cUeISN0wPu13m8lNsM=";

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
          buildInputs = with pkgs; [
            go_1_24
            gopls
            gotools
            golangci-lint
          ];

          shellHook = ''
            echo "pvetui development environment"
            echo "Go version: $(go version)"
          '';
        };
      }
    );
}
