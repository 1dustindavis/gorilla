# Gorilla.UI.App

WinUI 3 application project for Gorilla UI.

## Ownership
- Keep Windows-specific UI concerns here: XAML, WinUI lifecycle, page/code-behind adapters, cache-path selection, and runtime composition.
- Platform-neutral presentation and workflow behavior belongs in `Gorilla.UI.Core`.
- Named-pipe protocol and transport behavior belongs in `Gorilla.UI.Client`.
- The committed project and source files are the source of truth; do not regenerate this directory from WinUI starter templates.

## Dependencies
The intended runtime dependency direction is:

`Gorilla.UI.App -> Gorilla.UI.Core -> Gorilla.UI.Client`

App also references Client directly at the composition boundary so it can construct `NamedPipeGorillaServiceClient`.

## Validation
Run portable UI validation from the repository root with:

```text
make ui-lint
make ui-test
```

Build the real WinUI application on Windows with:

```text
make ui-windows-build
```

For broader architecture and validation guidance, see `gorilla-ui/ARCHITECTURE.md` and `gorilla-ui/README.md`.
