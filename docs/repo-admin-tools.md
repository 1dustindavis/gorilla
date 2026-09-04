# Repository administration

Gorilla can compile package metadata into catalogs:

```powershell
gorilla.exe -build -config C:\path\to\config.yaml
```

Use a normal client configuration containing `url` and `manifest`, and set `repo_path` to the local Gorilla content repository root. If `repo_path` is omitted, Gorilla uses the current working directory.

```yaml
repo_path: C:/path/to/gorilla-repo
```

Each YAML file under `packages-info/` should contain `catalog` and normal catalog-item fields:

```yaml
item_name: GoogleChrome
catalog: base
display_name: Google Chrome
installer:
  type: nupkg
  location: packages/google-chrome/GoogleChrome.nupkg
  hash: <sha256>
check:
  registry:
    name: Google Chrome
    version: 1.2.3.4
version: 1.2.3.4
```

See [the package-info example](../examples/example_package-info.yaml).

`-build` writes one file per catalog to `<repo_path>/catalogs/<catalog>.yaml`. Item keys are selected from `item_name`, then `display_name` without spaces, then the package-info filename. Entries without `catalog` or any usable item name are skipped.

The `-import <path>` command is reserved but not yet implemented.
