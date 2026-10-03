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

`item_name`, `catalog`, and `version` are required. Gorilla does not derive identity from `display_name` or the filename, and a record without a catalog is an error.

Package-info files may retain historical versions of the same item. For each `(catalog, item_name)`, catalog generation compares concrete version strings deterministically and publishes only the newest version. Selection is independent of filenames and filesystem traversal order.

For example, these records may coexist:

```text
packages-info/chrome-143.yaml
packages-info/chrome-144.yaml
packages-info/chrome-145.yaml
```

If they all describe the same `catalog` and `item_name`, the generated catalog contains only the newest version while all source package-info files remain in place.

Version comparison supports numeric and alphabetic runs separated by `.`, `-`, `_`, or `+`, as well as adjacent numeric/text runs and an optional leading `v`. Examples include `1`, `1.0`, `24.09`, `2.47.1`, `145.0.7632.76`, `1.2-beta`, `1.2b2`, `2026-Q3`, and `v2.4.1`. Numeric runs compare numerically, text runs compare case-insensitively, and common prerelease markers (`dev`, `alpha`/`a`, `beta`/`b`, `preview`/`pre`, `rc`) sort below the corresponding final release. Trailing numeric zero runs remain equivalent, so `1`, `1.0`, and `1.0.0` compare equally.

Labels that do not identify a concrete version, such as `latest` or `stable`, are rejected.

A duplicate `(catalog, item_name, version)` is invalid. Any malformed, duplicate, or unsupported package-info record causes the entire build to fail; Gorilla validates the repository before replacing the existing generated `catalogs/` directory.

Normal `catalog.Item` metadata remains supported, including display name, description, icon, dependencies, checks, installer/uninstaller metadata, blocking apps, and pre/post-install scripts.

See [the package-info example](../examples/example_package-info.yaml).

Additional repository-admin commands are planned separately; cleanup is not part of the build command.
