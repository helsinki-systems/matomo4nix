{ fetchurl, recurseIntoAttrs, unzip, zopfli, lib, stdenvNoCC, overrides ? (self: super: {}) }: with lib;
let packages = (self:
  let
    pluginJSON = builtins.fromJSON (readFile ./plugins.json);
    themeJSON = builtins.fromJSON (readFile ./themes.json);
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
    plugins = recurseIntoAttrs (mapAttrs mkPkg pluginJSON);
    themes = recurseIntoAttrs (mapAttrs mkPkg themeJSON);
  });
in fix' (extends overrides packages)
