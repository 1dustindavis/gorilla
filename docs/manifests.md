# Manifests

A manifest assigns catalog items to a computer and may include other manifests or catalogs.

```yaml
name: example
managed_installs:
  - GoogleChrome
managed_uninstalls:
  - Firefox
managed_updates:
  - Jre8
optional_installs:
  - VLC
included_manifests:
  - printers
catalogs:
  - production
```

- `name`: Manifest name.
- `managed_installs`: Items installed when missing or older than the catalog version.
- `managed_uninstalls`: Items removed when installed.
- `managed_updates`: Items updated only when already installed and older than the catalog version.
- `optional_installs`: Items exposed for on-demand installation or removal through service commands and App Catalog.
- `included_manifests`: Additional manifests processed once in discovered order.
- `catalogs`: Catalogs added after those listed in client configuration.

Remote manifests are loaded from `<url>manifests/<name>.yaml`. Files listed in the configuration's `local_manifests` are processed afterward. See [the example manifest](../examples/example_manifest.yaml).
