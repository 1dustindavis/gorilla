namespace Gorilla.UI.Client;

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
        var cached = await _cacheCoordinator.LoadCachedAsync(cancellationToken);
        if (cached is not null)
        {
            applyCachedItems(cached.Items);
        }

        try
        {
            var refreshed = await _cacheCoordinator.RefreshAsync(cancellationToken);
            applyRefreshedItems(refreshed.Items);
            return string.Empty;
        }
        catch (Exception ex)
        {
            return $"Showing cached data. Refresh failed: {ex.Message}";
        }
    }
}
