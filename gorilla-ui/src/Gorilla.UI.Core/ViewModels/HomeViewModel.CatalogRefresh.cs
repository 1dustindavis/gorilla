using Gorilla.UI.Core.Models;

namespace Gorilla.UI.Core.ViewModels;

public sealed partial class HomeViewModel
{
    private bool _catalogStateSubscribed;

    public CatalogDataState CatalogState
    {
        get
        {
            EnsureCatalogStateSubscription();
            return _cacheCoordinator.State;
        }
    }

    public async Task RefreshCatalogAsync(CancellationToken cancellationToken)
    {
        EnsureCatalogStateSubscription();
        await _cacheCoordinator.RefreshAsync(
            (items, _) =>
            {
                // First accept the live snapshot. Only after canonical reconciliation
                // succeeds is it fresh truth that may supersede attempt-level feedback
                // and a service-admission Retry block from the prior snapshot.
                ApplyItems(items);

                lock (_projectionStateLock)
                {
                    foreach (var activity in _activityItems.Values)
                    {
                        activity.SetRetryAttemptFeedback(null);
                    }
                    foreach (var item in _catalogItems.Values)
                    {
                        item.ClearRetryBlock();
                        item.TransientFeedback = null;
                    }

                    // ApplyItems rebuilt recovery while the prior attempt guard still
                    // existed. Recompute once more from the successfully applied snapshot
                    // after clearing local attempt state.
                    RebuildActivityProjection();
                }
                return Task.CompletedTask;
            },
            cancellationToken
        );
    }

    private void EnsureCatalogStateSubscription()
    {
        if (_catalogStateSubscribed)
        {
            return;
        }

        _cacheCoordinator.StateChanged += CacheCoordinator_StateChanged;
        _catalogStateSubscribed = true;
    }

    private void CacheCoordinator_StateChanged(object? sender, EventArgs e)
    {
        OnPropertyChanged(nameof(CatalogState));
    }
}
