# Gorilla UI Plan (historical)

This file is retained only as a historical pointer. It is **not** an active roadmap or implementation checklist.

The staged App Catalog implementation history is recorded in [issue #208](https://github.com/1dustindavis/gorilla/issues/208). Current behavior and contributor guidance live in:

- [ARCHITECTURE.md](ARCHITECTURE.md) — current App/Core/Client architecture, protocol boundary, operation/recovery semantics, cache/freshness behavior, and validation layers.
- [docs/app-catalog-contract.md](docs/app-catalog-contract.md) — current optional-software state/action/result contract and its historical rationale.
- [docs/app-catalog-recovery.md](docs/app-catalog-recovery.md) — mutation identity, operation retention, reconnect, relaunch, and service-restart recovery behavior.
- [docs/accessibility-automation.md](docs/accessibility-automation.md) — current keyboard, accessibility, focus, and UI Automation contract.
- [README.md](README.md) — development and validation commands.
- [../docs/app-catalog.md](../docs/app-catalog.md) — user/operator product documentation.

Earlier versions of this file described implementation TODOs and the initial v0/v1 design while App Catalog was still being built. Those steps have since been superseded by the completed work tracked in #208 and should not be used to infer current protocol fields, UI surfaces, or remaining product work.
