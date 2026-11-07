using Gorilla.UI.Client;

namespace Gorilla.UI.Core.ViewModels;

public sealed partial class HomeViewModel
{
    public Task RetryActivityLoadAsync(CancellationToken cancellationToken)
        => RefreshRetainedOperationsAsync(_needsOperationReconciliation, cancellationToken);

    private async Task RefreshRetainedOperationsAsync(bool force, CancellationToken cancellationToken)
    {
        if (IsActivityLoaded && !force)
        {
            return;
        }

        try
        {
            var warning = InfrastructureWarning;
            var operations = await _operationTracker.RefreshKnownOperationsAsync(cancellationToken);
            IsActivityLoaded = true;
            _needsOperationReconciliation = false;
            lock (_projectionStateLock)
            {
                foreach (var item in _catalogItems.Values)
                {
                    ReprojectOperation(item);
                }
                RebuildActivityProjection();
            }

            foreach (var operation in operations)
            {
                ProjectOperation(operation);
                if (operation.State != OperationState.Completed)
                {
                    StartRecoveredTracking(operation, cancellationToken);
                }
            }

            ClearRetainedOperationLookupInfrastructureWarning();
            if (InfrastructureWarning == warning &&
                warning.Context == "Operation tracking-loss reconciliation failed")
            {
                if (operations.Any(operation => operation.OperationId == warning.OperationId))
                {
                    ClearInfrastructureWarning();
                }
                else
                {
                    SetInfrastructureWarning(
                        "Operation tracking is no longer available. Gorilla refreshed current installation state without assuming the previous operation succeeded or failed.",
                        "Operation disappeared during tracking-loss reconciliation",
                        operationId: warning.OperationId,
                        itemName: warning.ItemName,
                        expectedAction: warning.ExpectedAction
                    );
                }
            }
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch (Exception ex)
        {
            // Retain the operation-specific context for a later successful
            // reconciliation. A generic lookup failure would otherwise overwrite
            // the only evidence needed to explain a vanished operation.
            if (_needsOperationReconciliation &&
                InfrastructureWarning.Context == "Operation tracking-loss reconciliation failed")
            {
                return;
            }
            SetInfrastructureWarning(
                "Operation status is temporarily unavailable.",
                "Retained operation lookup during App Catalog refresh",
                ex
            );
        }
    }
}
