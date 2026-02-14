# PipeHarness

Minimal CLI harness for validating Gorilla named-pipe protocol interactions before full UI wiring.

## Goals
- Verify request/response envelope shape.
- Verify operation acceptance and `operationId` correlation.
- Verify `StreamOperationStatus` event flow to terminal state.

## Planned usage (Windows)
- `dotnet run --project gorilla-ui/tools/PipeHarness -- list`
- `dotnet run --project gorilla-ui/tools/PipeHarness -- install <itemId>`
- `dotnet run --project gorilla-ui/tools/PipeHarness -- remove <itemId>`
- `dotnet run --project gorilla-ui/tools/PipeHarness -- stream <operationId>`
