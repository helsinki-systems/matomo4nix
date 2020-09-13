# matomo4nix

So you want to roll out Matomo on NixOS but you also want to use Nix to manage your plugins and themes instead of the builtin plugin system?
You came to the right place.

This default.nix expression contains the code to handle all themes and plugins Matomo has to offer.
It does that by parsing pre-generated JSON files with all plugins and themes.
The files are pre-generated using the `main.go` script.

Plugins are only updated if they specify compatibility with the version currently in nixpkgs.

## Generating the JSONs

The main.go script (by default) parses **all** plugins and themes for Matomo.

There also is an environment varaible, called `COMMIT_LOG`.
If set to `1`, logs are generated.
This is used by the `ci` script.

---

The `ci` script is run daily by our CI and updates all plugins and themes.
It basically runs the `generate.py` script and generates a commit message.

## Using the generated expressions

```nix
  matomoPackages = (callPackage (builtins.fetchGit {
    url = "https://git.helsinki.tools/helsinki-systems/matomo4nix";
    ref = "master";
  }) {}) // {
    withPlugins = matomoPkg: pluginPkgs: runCommand "matomo-with-plugins" {} ''
      cp -a ${matomoPkg}/. $out
      find $out -type d -exec chmod 755 {} +
      for i in ${lib.concatStringsSep " " pluginPkgs}; do
        cp -a $i/. $out
      done
    '';
  };
```

```sh
$ nix-shell -p matomoPackages.plugins.kDebug
```
