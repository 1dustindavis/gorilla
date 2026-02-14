# PipeHarness

CLI harness for validating Gorilla named-pipe protocol interactions before full UI wiring.

## Goals
- Verify request/response envelope shape.
- Verify operation acceptance and `operationId` correlation.
- Verify `StreamOperationStatus` event flow to terminal state.

## Usage (Windows service pipe)
- `dotnet run --project gorilla-ui/tools/PipeHarness -- list`
- `dotnet run --project gorilla-ui/tools/PipeHarness -- install <itemName>`
- `dotnet run --project gorilla-ui/tools/PipeHarness -- remove <itemName>`
- `dotnet run --project gorilla-ui/tools/PipeHarness -- stream <operationId>`

Optional pipe selection:
- `dotnet run --project gorilla-ui/tools/PipeHarness -- --pipe <pipeName> list`
- or set `GORILLA_PIPE_NAME`

## Notes
- This harness uses `Gorilla.UI.Client` shared contracts and `NamedPipeGorillaServiceClient`.
- It expects the service named-pipe endpoint to support the v1 envelope and operation names.
