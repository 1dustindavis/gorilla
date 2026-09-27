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
icon: icons/google-chrome.png
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

`icon` is optional presentation metadata used by App Catalog. It must be a repository-relative PNG path. Keeping icons under a top-level `icons/` directory is the recommended repository convention, but it is not required; any safe repository-relative PNG path is accepted. Absolute paths, URLs, paths that escape the repository root, unsupported image formats, missing files, unreadable files, and malformed PNG data are ignored for presentation and do not affect software detection, policy, install/remove actions, or item availability.

A typical repository may therefore use:

```text
repo/
├── catalogs/
├── manifests/
├── packages/
├── packages-info/
└── icons/
```

The service resolves icon assets through Gorilla's existing repository access configuration and stores validated local copies in its cache. The first implementation supports PNG only. Cached icons are reused without remote revalidation, so replacing an icon while retaining the same repository URL and path may continue to show the cached copy until that cache entry is removed.

See [the package-info example](../examples/example_package-info.yaml).

`-build` writes one file per catalog to `<repo_path>/catalogs/<catalog>.yaml`. Item keys are selected from `item_name`, then `display_name` without spaces, then the package-info filename. Entries without `catalog` or any usable item name are skipped.

The `-import <path>` command is reserved but not yet implemented.
