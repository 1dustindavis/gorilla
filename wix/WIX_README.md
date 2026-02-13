# Building an MSI with Wix
To build an MSI, run `make msi` from the repo root (Windows), or run the included Batch script `make-msi.bat` from the `wix` directory.
An MSI should be created in the same directory.

`ProductVersion` is passed to WiX from Makefile (`MSI_VERSION`) via `PRODUCT_VERSION`.

## Requirements
* [Wix Toolset](http://wixtoolset.org)
