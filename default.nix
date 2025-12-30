{ fetchurl, unzip, zopfli, lib, stdenvNoCC, overrides ? (_: _: {}) }:
let packages = _:
  let
    pluginJSON = builtins.fromJSON (lib.readFile ./plugins.json);
    themeJSON = builtins.fromJSON (lib.readFile ./themes.json);
    mkPkg = pname: value: stdenvNoCC.mkDerivation rec {
      inherit pname;
      inherit (value) version;
      src = fetchurl {
        inherit (value) url sha256;
        name = "${pname}-${version}.zip";
      };
      buildInputs = [ unzip ];
      installPhase = ''
        mkdir -p $out/share/plugins/${pname}
        cp -R ./. $out/share/plugins/${pname}/

        find $out -type f -name '*.png' -exec ${zopfli}/bin/zopflipng --lossy_transparent -y '{}' '{}' \;
      '';
    };
  in {
    plugins = lib.recurseIntoAttrs (lib.mapAttrs mkPkg pluginJSON);
    themes = lib.recurseIntoAttrs (lib.mapAttrs mkPkg themeJSON);
  };
in lib.fix' (lib.extends overrides packages)
