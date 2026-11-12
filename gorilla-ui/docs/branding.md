# Application branding

The packaged Gorilla UI supports a narrow presentation-only branding surface.

## Supported branding

A packaged build may customize these presentation values:

- `Package/Properties/DisplayName`;
- `Package/Applications/Application/uap:VisualElements/@DisplayName`;
- `Package/Applications/Application/uap:VisualElements/@Description`;
- visual resources referenced by the package manifest, such as the application and package logos.

The package display name is the authoritative application title. The WinUI window resolves its title from `Package.Current.DisplayName`; source-built or unpackaged development runs fall back to `App Catalog`.

The checked-in/default Gorilla package remains branded as `App Catalog`.

## Fixed Gorilla identity

Branding does not define a different Gorilla runtime. The following are functional identity and are outside the supported branding surface:

- package `Identity.Name`;
- package `Identity.Publisher`;
- package `Application/@Id`;
- packaged application executable identity;
- the `gorilla.exe` application execution alias;
- the `gorilla` packaged Windows service name;
- the packaged service executable, startup account, startup type, and arguments;
- package capabilities;
- package dependencies.

`Identity.Version` is package/release metadata rather than branding.

`PublisherDisplayName` remains Gorilla-controlled and is not a third-party branding field.

## Invariant

> Gorilla branding changes presentation only. A branded Gorilla package remains Gorilla: it retains the same package family identity, application identity, executable and execution alias, Windows service identity, capabilities, and runtime behavior.

Changes to fixed identity values are valid only as deliberate Gorilla product changes and should be reviewed independently of branding work.
