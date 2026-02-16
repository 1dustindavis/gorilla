# Gorilla.UI.App Templates

These files are copy-ready starter templates for wiring the generated WinUI app project to `Gorilla.UI.Client`.

Usage on Windows VM after running `gorilla-ui/tools/scaffold-winui.ps1`:
1. Copy these files into the generated `gorilla-ui/src/Gorilla.UI.App/` project.
2. Adjust namespaces if your generated project name differs.
3. Register service/viewmodel dependencies in `App.xaml.cs`.
4. Wire `HomePage` as the startup page.
5. Iterate on XAML/UX once end-to-end calls are functional.

This template set is intentionally minimal and focused on cache-first startup + install/remove status flow wiring.
