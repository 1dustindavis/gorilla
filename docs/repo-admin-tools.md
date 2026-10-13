# Repository administration

Use `gorilla admin` commands to manage a Gorilla repository. These commands do not use Gorilla’s client config file.

## Build catalogs

From the repository root:

```text
gorilla admin build
```

Or specify the repository explicitly:

```text
gorilla admin build --repo /srv/gorilla
gorilla admin build --repo C:\gorilla-repo
```

If `--repo` is omitted, Gorilla uses the current working directory.

The build recursively reads `.yaml` and `.yml` files under `packages-info/` and regenerates `catalogs/<catalog>.yaml`.

Every package-info record must explicitly contain:

```yaml
item_name: GoogleChrome
catalog: base
version: 145.0.7632.76
```

`item_name`, `catalog`, and `version` are required. Gorilla does not infer them from `display_name` or the filename. Catalog names cannot contain path separators, `..`, or platform-specific path syntax.

You can keep older package-info versions for the same item. For each `(catalog, item_name)`, Gorilla compares the versions and publishes only the newest one. Filenames and filesystem order do not affect which version is selected.

For example, these records may coexist:

```text
packages-info/chrome-143.yaml
packages-info/chrome-144.yaml
packages-info/chrome-145.yaml
```

If they use the same `catalog` and `item_name`, only the newest version is written to the generated catalog. The package-info files stay in place.

Versions must use dotted numbers such as `1`, `1.0`, `24.09`, `2.47.1`, or `145.0.7632.76`. Values such as `latest`, `stable`, `1.2-beta`, `v2.4.1`, or `2026-Q3` are not supported.

Trailing zeros do not affect ordering, so `1`, `1.0`, and `1.0.0` compare as the same version. Do not create separate package-info records for equivalent versions; Gorilla treats that as an error.

A duplicate `(catalog, item_name, version)` is also an error. If any package-info record is invalid, the build stops without replacing the current catalogs.

Gorilla builds the new catalogs in a temporary directory first. It replaces `catalogs/` only after the full build succeeds.

Package-info can still use the normal catalog fields, including display name, description, icon, dependencies, checks, installer/uninstaller settings, blocking apps, and pre/post-install scripts.

See [the package-info example](../examples/example_package-info.yaml).

## Repository cleanup

Preview repository cleanup with:

```text
gorilla admin cleanup
gorilla admin cleanup --repo /srv/gorilla --keep 2
```

Apply the cleanup with:

```text
gorilla admin cleanup --apply
gorilla admin cleanup --repo /srv/gorilla --keep 3 --apply
```

If `--repo` is omitted, Gorilla uses the current working directory. `--keep` defaults to `3` versions per active item, including the current version. Cleanup only previews changes unless `--apply` is provided.

Cleanup checks every manifest under `manifests/`, then follows package dependencies. An item is active if any manifest or active dependency references it. Gorilla keeps the newest `--keep` versions of each active item and marks older versions as superseded. Items that are not referenced are abandoned, regardless of `--keep`.

Cleanup tracks repository files referenced by `icon`, `installer.location`, and `uninstaller.location`. Inline PowerShell fields such as `check.script`, `preinstall_script`, and `postinstall_script` are script contents, not file paths. A shared file is kept as long as any surviving package-info record still uses it. Missing referenced files are reported but are not deleted.

With `--apply`, Gorilla removes superseded and abandoned package-info files, removes abandoned files under `packages/` and `icons/`, rebuilds the catalogs, and removes directories that became empty. The `packages/`, `icons/`, and `packages-info/` roots are never removed, and directories containing files such as `.gitkeep` stay in place.

Before deleting anything permanently, Gorilla moves the cleanup candidates into a temporary staging directory inside the repository. It then rebuilds the catalogs from the files that remain. If staging or catalog generation fails, Gorilla restores the staged files. After the new catalogs are in place, cleanup is committed and the staged files are deleted.

`--apply` is the only option that removes files. Cleanup does not prompt for confirmation, change manifests, create Git commits, or delete unrelated repository files.

## Admin binaries for macOS and Linux

Gorilla releases include repository-admin binaries for macOS and Linux. Download the appropriate `gorilla-admin-<version>-<os>-<arch>` file from [GitHub Releases](https://github.com/1dustindavis/gorilla/releases).

These binaries support `gorilla admin build` and `gorilla admin cleanup`, including `--apply`. They are only for repository administration. The Gorilla client and service remain Windows-only.
