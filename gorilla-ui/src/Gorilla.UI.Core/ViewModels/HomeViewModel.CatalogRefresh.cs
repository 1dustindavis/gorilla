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
                // Published snapshots are useful immediately, even while a requested
                // regeneration is still queued or running. Applying them must not run
                // completion-only recovery side effects.
                ApplyItems(items);
                return Task.CompletedTask;
            },
            cancellationToken
        );

        // Only successful terminal completion is affirmative evidence that the manual
        // refresh superseded prior attempt-level/recovery state.
        lock (_projectionStateLock)
        {
            foreach (var activity in _activityItems.Values)
            {
                activity.SetRetryAttemptFeedback(null);
            }
            foreach (var item in _catalogItems.Values)
            {
                item.ClearRetryBlocks();
                item.TransientFeedback = null;
            }

            RebuildActivityProjection();
        }

        ClearCatalogRecoveryInfrastructureWarning();
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
