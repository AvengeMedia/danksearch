{
  description = "Indexed filesystem search in GO";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      forAllSystems = (f:
        nixpkgs.lib.genAttrs supportedSystems (system:
          f nixpkgs.legacyPackages.${system}
        )
      );

    in
    {
      packages = forAllSystems (
        pkgs:
        let
          inherit (pkgs) lib system;
          dsearchVersion = "0.1.1";
        in
        {
          dsearch = pkgs.buildGoModule {
            pname = "dsearch";
            version = dsearchVersion;
            
            src = ./.;
            vendorHash = "sha256-PNWwTurmpn1Vj8Q5jFQFHGOEYCEQwpgGNGxtil10um8=";

            subPackages = [ "cmd/dsearch" ];

            ldflags = [
              "-s"
              "-w"
              "-X main.Version=${dsearchVersion}"
            ];

            meta = {
              description = "Indexed filesystem search in GO";
              homepage = "https://github.com/AvengeMedia/danksearch";
              mainProgram = "dsearch";
              license = lib.licenses.mit;
              platforms = lib.platforms.unix;
            };
          };

          default = self.packages.${system}.dsearch;
        }
      );

      homeModules = {
        default = self.homeModules.dsearch;
        dsearch = import ./distro/nix self;
      };
    };
}
