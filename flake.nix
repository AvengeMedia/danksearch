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
          dsearchVersion = "0.0.7";
        in
        {
          dsearch = pkgs.buildGoModule {
            pname = "dsearch";
            version = dsearchVersion;
            
            src = ./.;
            vendorHash = "sha256-65NFlAtix5ehyaRok3/0Z6+j6U7ccc0Kdye0KFepLLM=";

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
