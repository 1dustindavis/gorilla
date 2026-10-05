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

Preview repository cleanup with:

```text
gorilla admin cleanup
gorilla admin cleanup --repo /srv/gorilla --keep 2
```

If `--repo` is omitted, Gorilla uses the current working directory. `--keep` defaults to `3` versions per active item.

Cleanup reads all manifests in `manifests/`, follows package dependencies, and reports superseded package-info, abandoned items, abandoned files under `packages/` and `icons/`, and missing referenced files.

Cleanup is currently dry-run only. It does not modify repository files.
