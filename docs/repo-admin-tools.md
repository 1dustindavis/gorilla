# Repository administration

Gorilla repository administration uses a dedicated command boundary and does not load normal client configuration.

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

`item_name`, `catalog`, and `version` are required. Gorilla does not derive identity from `display_name` or the filename, and a record without a catalog is an error. Catalog names are logical identifiers and may not contain path separators, traversal components, or platform-specific path forms.

Package-info files may retain historical versions of the same item. For each `(catalog, item_name)`, catalog generation compares dotted numeric versions and publishes only the newest version. Selection is independent of filenames and filesystem traversal order.

For example, these records may coexist:

```text
packages-info/chrome-143.yaml
packages-info/chrome-144.yaml
packages-info/chrome-145.yaml
```

If they all describe the same `catalog` and `item_name`, the generated catalog contains only the numerically newest version while all source package-info files remain in place.

Versions must be concrete dotted numeric values such as `1`, `1.0`, `24.09`, `2.47.1`, or `145.0.7632.76`. Arbitrary labels and textual/prerelease forms such as `latest`, `stable`, `1.2-beta`, `v2.4.1`, or `2026-Q3` are rejected.

Trailing numeric zero segments compare equivalently, so `1`, `1.0`, and `1.0.0` have the same ordering value. Separate package-info records for the same `(catalog, item_name)` may not use distinct version strings that compare equivalently; such ambiguity is rejected rather than resolved by filename ordering.

A duplicate `(catalog, item_name, version)` is invalid. Any malformed, duplicate, equivalent-version, or unsupported package-info record causes the entire build to fail.

Gorilla writes the complete generated catalog set to a temporary sibling directory before replacing `catalogs/`. If activation fails, the previous catalog directory is restored rather than leaving missing or partially generated output.

Normal `catalog.Item` metadata remains supported, including display name, description, icon, dependencies, checks, installer/uninstaller metadata, blocking apps, and pre/post-install scripts.

See [the package-info example](../examples/example_package-info.yaml).

## Repository cleanup

Preview repository cleanup with:

```text
gorilla admin cleanup
gorilla admin cleanup --repo /srv/gorilla --keep 2
```

Apply the displayed cleanup plan explicitly with:

```text
gorilla admin cleanup --apply
gorilla admin cleanup --repo /srv/gorilla --keep 3 --apply
```

If `--repo` is omitted, Gorilla uses the current working directory. `--keep` defaults to `3` total package-info versions per active item, including the current version. Cleanup is a dry run unless `--apply` is provided.

Cleanup reads every manifest under `manifests/` as a reachability root, then follows package dependencies transitively. For each live item Gorilla keeps the newest configured number of package-info versions and classifies older versions as superseded. Items not reachable from any repository manifest or dependency are abandoned regardless of retention count.

Managed repository-file references are derived from surviving package-info fields for `icon`, `installer.location`, and `uninstaller.location`. Inline PowerShell fields such as `check.script`, `preinstall_script`, and `postinstall_script` are script contents, not repository-file paths. Shared managed assets remain if any surviving package-info record still references them. Missing referenced assets are reported as repository-health findings but are not deletion candidates by themselves.

When `--apply` is used Gorilla removes the package-info files already classified as superseded or abandoned, removes exactly the abandoned managed assets from the plan, regenerates catalogs from the surviving package-info, and removes newly empty descendant directories under `packages/`, `icons/`, and `packages-info/`. The managed roots themselves are retained, as are directories containing files such as `.gitkeep`.

Applied cleanup uses a staging transaction inside the repository. Planned files are first moved out of the live managed roots while preserving their repository-relative paths. Catalogs are then regenerated against the surviving package-info. If staging or catalog generation fails before the new catalogs are committed, the staged package-info and assets are restored rather than leaving a partially cleaned repository. Once catalog replacement is committed, cleanup does not restore old package-info into the live repository merely because later temporary-file housekeeping fails.

`--apply` is the only destructive opt-in. Cleanup does not prompt interactively, modify manifests, create Git commits, or perform arbitrary filesystem garbage collection.
