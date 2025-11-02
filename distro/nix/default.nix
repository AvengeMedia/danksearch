self: {
  config,
  lib,
  pkgs,
  ...
}:
with lib;
let
  tomlFormat = pkgs.formats.toml {};

  cfg = config.programs.dsearch;
in {
  options.programs.dsearch = {
    enable = mkEnableOption "danksearch";
    package = mkPackageOption self.packages.${pkgs.system} "dsearch" { };

    config = mkOption {
      type = types.nullOr tomlFormat.type;
      default = null;
      description = ''
        dsearch configuration
      '';
    };
  };

  config = mkIf cfg.enable {
    home.packages = [ cfg.package ];

    systemd.user.services.dsearch = {
      Unit = {
        Description = "dsearch - Fast filesystem search service";
        Documentation = "https://github.com/AvengeMedia/dsearch";
        After = [ "network.target" ];
      };

      Service = {
        Type = "simple";
        ExecStart = "${getExe cfg.package} serve";
        Restart = "on-failure";
        RestartSec = "5s";

        StandardOutput = "journal";
        StandardError = "journal";
        SyslogIdentifier = "dsearch";
      };

      Install = {
        WantedBy = [ "default.target" ];
      };
    };

    xdg.configFile."danksearch/config.toml" = mkIf (cfg.config != null) {
      source = tomlFormat.generate "dsearch.config.toml" cfg.config;
    };
  };
}