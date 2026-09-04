# App Catalog

![Gorilla App Catalog](https://github.com/user-attachments/assets/5defc532-6d4e-4961-8c90-3b4648e3c650)

App Catalog is Gorilla's pre-release Windows UI for on-demand software actions. It:

- Lists items assigned through `optional_installs`.
- Installs and removes optional items through the Gorilla service.
- Streams operation status updates.
- Loads cached data at startup, then refreshes it from the service.

Install the versioned `gorilla-<version>.msix` from [Gorilla releases](https://github.com/1dustindavis/gorilla/releases):

```powershell
Add-AppxPackage -Path .\gorilla-2.30.0.msix -ForceUpdateFromAnyVersion
```

The MSIX installs App Catalog and registers the automatic Gorilla service. App Catalog requires a configured manifest containing `optional_installs` items.

The UI and its protocol remain pre-release and may change.
