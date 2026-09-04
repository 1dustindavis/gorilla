# Client configuration

Gorilla reads YAML configuration from `%ProgramData%\gorilla\config.yaml` by default. To use another file:

```powershell
gorilla.exe -config C:\path\to\config.yaml
```

## Example

```yaml
url: https://example.com/gorilla/
manifest: example
catalogs:
  - production
app_data_path: C:/ProgramData/gorilla
service_interval: 1h
```

`url` and `manifest` are required for normal managed runs. Repository URLs must include a trailing slash because Gorilla appends paths such as `manifests/example.yaml`.

## Keys

- `url`: Base URL containing the `manifests/` and `catalogs/` directories. A local repository may use a URL such as `file://C:/example/gorilla/`.
- `manifest`: Primary manifest assigned to the computer.
- `catalogs`: Optional ordered list of catalogs. Catalogs may instead be supplied by manifests.
- `url_packages`: Optional alternate base URL for package downloads. Defaults to `url`.
- `local_manifests`: Optional list of local manifest paths processed after remote manifests.
- `app_data_path`: Working directory for cache, logs, reports, and the service manifest. Defaults to `%ProgramData%\gorilla`.
- `service_interval`: Interval between service runs in Go duration format, such as `30m`, `1h`, or `2h`. Defaults to `1h`.
- `repo_path`: Local repository root used by [repository administration](repo-admin-tools.md). Defaults to the current directory.
- `auth_user` and `auth_pass`: Optional HTTP Basic Authentication credentials.
- `tls_auth`: Enables mutual TLS when `true`.
- `tls_client_cert`, `tls_client_key`, and `tls_server_cert`: PEM paths used for mutual TLS.
- `verbose`, `debug`, and `checkonly`: Optional runtime behavior flags.
