# Building the legacy MSI with WiX

The WiX package is retained for compatibility and development use. `gorilla-<version>.msix` is the official Gorilla installer and packages both the WinUI application and Gorilla Windows service.

To build the legacy MSI, run `make msi` from the repo root on Windows, or run the included `make-msi.bat` from the `wix` directory. An MSI should be created in the same directory.

`ProductVersion` is passed to WiX from Makefile (`MSI_VERSION`) via `PRODUCT_VERSION`.

## Requirements
* [WiX Toolset](http://wixtoolset.org)
