# Application branding

Gorilla's packaged UI can be customized with a different app name and visual assets without changing how Gorilla works.

## What can be customized

The supported branding fields are:

- `Package/Properties/DisplayName`;
- `Package/Applications/Application/uap:VisualElements/@DisplayName`;
- `Package/Applications/Application/uap:VisualElements/@Description`;
- the visual assets referenced by the package manifest, including the app/package logos.

The package display name is also used for the WinUI window title through `Package.Current.DisplayName`. Source-built or unpackaged development runs fall back to `App Catalog`.

The default Gorilla package still uses `App Catalog` and the existing Gorilla artwork.

## What stays the same

Branding should not change Gorilla's underlying package or runtime identity. These values stay fixed:

- `Identity.Name`;
- `Identity.Publisher`;
- `Application/@Id`;
- the packaged app executable identity;
- the `gorilla.exe` execution alias;
- the `gorilla` Windows service name;
- the service executable, startup account, startup type, and arguments;
- package capabilities;
- package dependencies.

`Identity.Version` is release/package metadata, not branding.

`PublisherDisplayName` also stays Gorilla-controlled.

The basic rule is simple: branding can change how the app is presented, but it should not change what Gorilla is or how it runs.

If one of the fixed identity values ever needs to change, treat that as a separate Gorilla product change rather than part of branding.
