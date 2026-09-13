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
                ApplyItems(items);
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
