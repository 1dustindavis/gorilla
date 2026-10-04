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

## Plan repository cleanup

Repository cleanup is currently a dry-run planner. It reports what a future cleanup operation would remove but does not delete, rename, rewrite, rebuild, or otherwise modify repository content.

From the repository root:

```text
gorilla admin cleanup
```

Or select a repository and retention count explicitly:

```text
gorilla admin cleanup --repo /srv/gorilla
gorilla admin cleanup --repo /srv/gorilla --keep 2
```

`--repo` defaults to the current working directory. `--keep` defaults to `3` and means the total number of package-info versions retained for each live `(catalog, item_name)`, including the current version. Values less than 1 are rejected.

Cleanup recursively reads every `.yaml` and `.yml` file under `manifests/`. Every repository manifest is treated as a root; cleanup does not use the client-configured manifest or runtime include traversal. A missing `manifests/` directory or any malformed manifest causes planning to fail rather than assuming repository content is abandoned.

The following manifest lists make item names live roots:

- `managed_installs`
- `optional_installs`
- `managed_uninstalls`
- `managed_updates`

Manifest references contain an item name but not a catalog. Cleanup therefore uses a conservative repository-wide rule: a reference to an item name keeps every matching `(catalog, item_name)` identity live. The same rule applies to dependencies.

Dependencies are read only from the newest package-info version of each live identity and expanded transitively until no additional item names become live. Cycles are valid. Historical dependency metadata does not keep obsolete items live.

For each live identity, package-info versions are ordered with the same numeric comparator used by catalog builds:

- the newest version is **current**;
- older versions still within `--keep` are **retained**;
- older versions beyond `--keep` are **superseded**.

An identity that is not reachable from any manifest or transitive dependency is an **abandoned item**. Every package-info record for an abandoned item is **abandoned package-info**, regardless of the retention count.

Cleanup also evaluates repository file references from these package-info fields:

- `icon`
- `check.script`
- `installer.location`
- `uninstaller.location`
- `preinstall_script`
- `postinstall_script`

Only current and retained package-info records keep referenced files live. Reference tracking is global, so a file shared by multiple versions or items remains live as long as at least one surviving record references it.

Repository asset references must be safe repository-relative paths. Absolute paths, drive-qualified paths, URLs, UNC paths, and paths that escape the repository are rejected during planning. This establishes the path-safety invariant required by a future destructive cleanup command.

Abandoned-file discovery is intentionally limited to regular files under:

```text
packages/
icons/
```

Gorilla does not scan manifests, package-info, generated catalogs, metadata, documentation, `.git/`, or arbitrary repository files for cleanup. Directories are not classified. `.gitkeep` marker files are ignored.

A managed file is **abandoned** when no current or retained package-info record references it. A file referenced only by superseded or abandoned package-info is therefore an abandoned-file candidate.

The planner also reports the inverse repository-health problem: a safe repository-relative file referenced by current or retained package-info that is absent from disk is listed as a **missing referenced file**. Missing files are not cleanup candidates and cleanup does not try to repair them.

The report distinguishes active items, current/retained/superseded versions, abandoned items, abandoned files, and missing referenced files, then ends with an explicit dry-run notice. Running `gorilla admin cleanup` never changes repository files in this phase.
