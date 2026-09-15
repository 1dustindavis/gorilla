using Gorilla.UI.Client;
using Gorilla.UI.Core.Models;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public sealed class CacheFallbackStateTests
{
    [Fact]
    public void InitialState_HasUnknownCacheFallback()
    {
        Assert.Equal(CacheFallbackState.Unknown, CatalogDataState.InitialLoading.CacheFallback);
        Assert.False(CatalogDataState.InitialLoading.HasNoUsableCache);
    }

    [Fact]
    public async Task CacheMiss_MarksFallbackUnavailableWithoutEndingInitialLoading()
    {
        var coordinator = new OptionalInstallsCacheCoordinator(new FakeClient(), new TestCacheStore());

        Assert.Null(await coordinator.LoadCachedAsync(CancellationToken.None));

        Assert.Equal(CacheFallbackState.Unavailable, coordinator.State.CacheFallback);
        Assert.True(coordinator.State.IsInitialLoading);
        Assert.True(coordinator.State.IsRefreshing);
        Assert.False(coordinator.State.HasUsableData);
    }

    [Fact]
    public async Task ValidCache_MarksFallbackAvailable()
    {
        var cachedAt = DateTimeOffset.Parse("2026-09-13T18:00:00Z");
        var store = new TestCacheStore
        {
            Document = new OptionalInstallsCacheDocument(cachedAt, []),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(new FakeClient(), store);

        var cached = await coordinator.LoadCachedAsync(CancellationToken.None);

        Assert.NotNull(cached);
        Assert.Equal(CacheFallbackState.Available, coordinator.State.CacheFallback);
        Assert.True(coordinator.State.IsCached);
    }

    [Fact]
    public async Task CacheReadFailure_MarksFallbackUnavailableBeforeRethrowing()
    {
        var coordinator = new OptionalInstallsCacheCoordinator(
            new FakeClient(),
            new TestCacheStore { LoadFailure = new InvalidDataException("invalid cache") }
        );

        await Assert.ThrowsAsync<InvalidDataException>(() => coordinator.LoadCachedAsync(CancellationToken.None));

        Assert.Equal(CacheFallbackState.Unavailable, coordinator.State.CacheFallback);
        Assert.True(coordinator.State.HasNoUsableCache);
    }

    [Fact]
    public async Task NoCache_LiveFailure_ProducesLoadFailureAndExplicitNoCacheState()
    {
        var failure = new IOException("service unavailable");
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromException<IReadOnlyList<OptionalInstallItem>>(failure),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new TestCacheStore());

        Assert.Null(await coordinator.LoadCachedAsync(CancellationToken.None));
        await Assert.ThrowsAsync<IOException>(() => coordinator.RefreshAsync(CancellationToken.None));

        Assert.True(coordinator.State.HasLoadFailure);
        Assert.True(coordinator.State.HasNoUsableCache);
        Assert.Equal(CacheFallbackState.Unavailable, coordinator.State.CacheFallback);
        Assert.False(coordinator.State.HasUsableData);
    }

    [Fact]
    public async Task NoCache_LoadFailureThenSuccessfulRefresh_ReplacesFailureWithLiveCatalog()
    {
        var attempt = 0;
        var client = new FakeClient
        {
            ListAsync = _ => ++attempt == 1
                ? Task.FromException<IReadOnlyList<OptionalInstallItem>>(new IOException("service unavailable"))
                : Task.FromResult<IReadOnlyList<OptionalInstallItem>>([Item("VLC")]),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new TestCacheStore());

        Assert.Null(await coordinator.LoadCachedAsync(CancellationToken.None));
        await Assert.ThrowsAsync<IOException>(() => coordinator.RefreshAsync(CancellationToken.None));
        Assert.True(coordinator.State.HasLoadFailure);
        Assert.True(coordinator.State.HasNoUsableCache);

        var refreshed = await coordinator.RefreshAsync(CancellationToken.None);

        Assert.Single(refreshed.Items);
        Assert.True(coordinator.State.IsLive);
        Assert.True(coordinator.State.HasUsableData);
        Assert.False(coordinator.State.HasLoadFailure);
        Assert.False(coordinator.State.IsSuccessfulEmpty);
        await WaitUntilAsync(() => coordinator.State.CacheFallback == CacheFallbackState.Available);
    }

    [Fact]
    public async Task ValidCache_LiveFailure_PreservesFallbackAndDoesNotReportNoCache()
    {
        var cachedAt = DateTimeOffset.Parse("2026-09-13T18:00:00Z");
        var store = new TestCacheStore
        {
            Document = new OptionalInstallsCacheDocument(cachedAt, []),
        };
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromException<IReadOnlyList<OptionalInstallItem>>(new IOException("service unavailable")),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        await coordinator.LoadCachedAsync(CancellationToken.None);
        await Assert.ThrowsAsync<IOException>(() => coordinator.RefreshAsync(CancellationToken.None));

        Assert.True(coordinator.State.IsCached);
        Assert.True(coordinator.State.HasUsableData);
        Assert.Equal(CacheFallbackState.Available, coordinator.State.CacheFallback);
        Assert.False(coordinator.State.HasNoUsableCache);
    }

    [Fact]
    public async Task SuccessfulLiveSave_ChangesPreviouslyUnavailableFallbackToAvailable()
    {
        var store = new TestCacheStore();
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        await coordinator.LoadCachedAsync(CancellationToken.None);
        Assert.Equal(CacheFallbackState.Unavailable, coordinator.State.CacheFallback);

        await coordinator.RefreshAsync(CancellationToken.None);
        await WaitUntilAsync(() => coordinator.State.CacheFallback == CacheFallbackState.Available);

        Assert.True(coordinator.State.IsLive);
        Assert.True(coordinator.State.IsSuccessfulEmpty);
        Assert.False(coordinator.State.HasLoadFailure);
    }

    private static OptionalInstallItem Item(string itemName) => new(
        ItemName: itemName,
        DisplayName: itemName,
        Version: "1.0",
        Catalog: "test",
        InstallerType: "msi",
        InstallerPackageId: itemName,
        InstallerLocation: $"{itemName}.msi",
        IsManaged: false,
        IsInstalled: false,
        Status: OptionalInstallStatus.NotInstalled,
        StatusUpdatedAtUtc: DateTimeOffset.Parse("2026-09-13T18:00:00Z"),
        LastOperationId: null
    );

    private static async Task WaitUntilAsync(Func<bool> predicate)
    {
        using var timeout = new CancellationTokenSource(TimeSpan.FromSeconds(2));
        while (!predicate())
        {
            await Task.Delay(10, timeout.Token);
        }
    }

    private sealed class TestCacheStore : IOptionalInstallsCacheStore
    {
        public OptionalInstallsCacheDocument? Document { get; set; }
        public Exception? LoadFailure { get; init; }

        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
        {
            if (LoadFailure is not null)
            {
                return Task.FromException<OptionalInstallsCacheDocument?>(LoadFailure);
            }
            return Task.FromResult(Document);
        }

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
        {
            Document = document;
            return Task.CompletedTask;
        }
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public Func<CancellationToken, Task<IReadOnlyList<OptionalInstallItem>>> ListAsync { get; init; } =
            _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]);

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => ListAsync(cancellationToken);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public async IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            [System.Runtime.CompilerServices.EnumeratorCancellation] CancellationToken cancellationToken
        )
        {
            await Task.CompletedTask;
            yield break;
        }
    }
}
