{
  description = "Indexed filesystem search in GO";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    gomod2nix = {
      url = "github:nix-community/gomod2nix/v1.7.0";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  outputs =
    { self, nixpkgs, gomod2nix }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
      ];

      forAllSystems =
        f:
        builtins.listToAttrs (
          map (system: {
            name = system;
            value = f system;
          }) supportedSystems
        );

    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
          lib = pkgs.lib;
          dsearchVersion = "0.0.7";
        in
        {
          dsearch = gomod2nix.legacyPackages.${system}.buildGoApplication {
            pname = "dsearch";
            version = dsearchVersion;
            src = ./.;
            modules = ./gomod2nix.toml;

            subPackages = [ "cmd/dsearch" ];

            ldflags = [
              "-s"
              "-w"
              "-X main.Version=${dsearchVersion}"
            ];

            postPatch = ''
              substituteInPlace assets/dsearch.service \
                --replace-fail /usr/local/bin $out/bin
            '';

            postInstall = ''              
              install -Dm 644 assets/dsearch.service -t $out/lib/systemd/user
            '';

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
    };
}
