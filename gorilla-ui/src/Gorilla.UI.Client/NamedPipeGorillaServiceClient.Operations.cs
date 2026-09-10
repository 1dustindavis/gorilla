namespace Gorilla.UI.Client;

public sealed partial class NamedPipeGorillaServiceClient
{
    public async Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
    {
        var requestEnvelope = CreateRequestEnvelope(
            operation: ProtocolConstants.Operation.ListOperations,
            operationId: string.Empty,
            payload: new ListOperationsRequest()
        );
        ClientDiagnostics.Log($"request:create operation={requestEnvelope.Operation} requestId={requestEnvelope.RequestId}");

        var responseEnvelope = await SendRequestAsync<ListOperationsRequest, ListOperationsResponse>(
            requestEnvelope,
            cancellationToken
        );

        var operations = responseEnvelope.Payload.Operations ?? [];
        var result = new List<OperationStatusEvent>(operations.Count);
        foreach (var operation in operations)
        {
            if (string.IsNullOrWhiteSpace(operation.OperationId))
            {
                throw new InvalidOperationException("Operation snapshot is missing operationId.");
            }
            ProtocolValidation.ValidateStatusEvent(operation.Status);
            result.Add(new OperationStatusEvent(
                OperationId: operation.OperationId,
                State: operation.Status.State,
                ProgressPercent: operation.Status.ProgressPercent,
                Message: operation.Status.Message,
                TimestampUtc: responseEnvelope.TimestampUtc,
                ItemName: operation.Status.ItemName!,
                Action: operation.Status.Action!.Value,
                Result: operation.Status.Result
            ));
        }

        return result;
    }
}
