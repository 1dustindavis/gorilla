using Gorilla.UI.Client;

namespace Gorilla.UI.Core;

public sealed class OptionalInstallsStartupLoader
{
    private readonly OptionalInstallsCacheCoordinator _cacheCoordinator;

    public OptionalInstallsStartupLoader(OptionalInstallsCacheCoordinator cacheCoordinator)
    {
        _cacheCoordinator = cacheCoordinator;
    }

    public async Task<string> InitializeAsync(
        Action<IReadOnlyList<OptionalInstallItem>> applyCachedItems,
        Action<IReadOnlyList<OptionalInstallItem>> applyRefreshedItems,
        CancellationToken cancellationToken
    )
    {
        Task AcceptRefreshedSnapshotAsync(
            IReadOnlyList<OptionalInstallItem> items,
            CancellationToken _
        )
        {
            applyRefreshedItems(items);
            return Task.CompletedTask;
        }

        // Register the canonical reconciler for later coordinator-owned refreshes,
        // including post-operation refreshes that do not flow through the manual
        // HomeViewModel.RefreshCatalogAsync entry point.
        _cacheCoordinator.RegisterSnapshotAcceptor(AcceptRefreshedSnapshotAsync);

        OptionalInstallsCacheDocument? cached = null;
        try
        {
            cached = await _cacheCoordinator.LoadCachedAsync(cancellationToken);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch
        {
            // An unreadable or invalid fallback cache is not authoritative. Treat it
            // as absent and continue to the live service instead of aborting session
            // initialization before a live catalog request can be attempted.
        }

        if (cached is not null)
        {
            applyCachedItems(cached.Items);
        }

        try
        {
            await _cacheCoordinator.RefreshAsync(AcceptRefreshedSnapshotAsync, cancellationToken);
        }
        catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
        {
            throw;
        }
        catch
        {
            // Catalog load/refresh failures are represented by the coordinator's
            // explicit CatalogDataState. WarningBanner remains reserved for other
            // service/operation infrastructure warnings.
        }

        return string.Empty;
    }
}
