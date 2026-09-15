using Gorilla.UI.Client;

namespace Gorilla.UI.Core.ViewModels;

public sealed partial class HomeViewModel
{
    public async Task RetryActivityLoadAsync(CancellationToken cancellationToken)
    {
        if (IsActivityLoaded)
        {
            return;
        }

        try
        {
            var operations = await _operationTracker.RefreshKnownOperationsAsync(cancellationToken);
            IsActivityLoaded = true;
            RebuildActivityProjection();

            foreach (var operation in operations)
            {
                ProjectOperation(operation);
                if (operation.State != OperationState.Completed)
                {
                    StartRecoveredTracking(operation, cancellationToken);
                }
            }

            ClearRetainedOperationLookupInfrastructureWarning();
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch (Exception ex)
        {
            SetInfrastructureWarning(
                "Operation status is temporarily unavailable.",
                "Retained operation lookup during App Catalog refresh",
                ex
            );
        }
    }
}
