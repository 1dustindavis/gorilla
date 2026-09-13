using System.Collections.Concurrent;
using Gorilla.UI.Client;
using Gorilla.UI.Core;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public class OptionalInstallsStartupLoaderTests
{
    [Fact]
    public async Task InitializeAsync_AppliesCachedBeforeRefreshCompletes()
    {
        var cachedNow = DateTimeOffset.Parse("2026-02-19T18:10:00Z");
        var cacheStore = new InMemoryCacheStore(new OptionalInstallsCacheDocument(cachedNow, [MakeItem("CachedVLC", cachedNow)]));
        var refreshReady = new TaskCompletionSource<IReadOnlyList<OptionalInstallItem>>(TaskCreationOptions.RunContinuationsAsynchronously);
        var client = new FakeClient { ListAsync = _ => refreshReady.Task };
        var coordinator = new OptionalInstallsCacheCoordinator(client, cacheStore);
        var loader = new OptionalInstallsStartupLoader(coordinator);

        var applyOrder = new ConcurrentQueue<string>();
        var cachedApplied = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        IReadOnlyList<OptionalInstallItem>? cachedItems = null;
        IReadOnlyList<OptionalInstallItem>? refreshedItems = null;

        var initializeTask = loader.InitializeAsync(
            items =>
            {
                cachedItems = items;
                applyOrder.Enqueue("cached");
                cachedApplied.TrySetResult();
            },
            items =>
            {
                refreshedItems = items;
                applyOrder.Enqueue("refreshed");
            },
            CancellationToken.None
        );

        await cachedApplied.Task.WaitAsync(TimeSpan.FromSeconds(2));
        Assert.Null(refreshedItems);
        Assert.True(coordinator.State.IsCached);

        var refreshNow = DateTimeOffset.Parse("2026-02-19T18:11:00Z");
        refreshReady.SetResult([MakeItem("FreshChrome", refreshNow)]);
        var warning = await initializeTask;

        Assert.Equal(string.Empty, warning);
        Assert.Equal("CachedVLC", Assert.Single(cachedItems!).ItemName);
        Assert.Equal("FreshChrome", Assert.Single(refreshedItems!).ItemName);
        Assert.True(coordinator.State.IsLive);
        Assert.True(applyOrder.TryDequeue(out var first));
        Assert.Equal("cached", first);
        Assert.True(applyOrder.TryDequeue(out var second));
        Assert.Equal("refreshed", second);
    }

    [Fact]
    public async Task InitializeAsync_RefreshFailureKeepsCachedAndRecordsDegradedState()
    {
        var now = DateTimeOffset.Parse("2026-02-19T18:10:00Z");
        var cacheStore = new InMemoryCacheStore(new OptionalInstallsCacheDocument(now, [MakeItem("CachedVLC", now)]));
        var client = new FakeClient { ListAsync = _ => Task.FromException<IReadOnlyList<OptionalInstallItem>>(new InvalidOperationException("service unavailable")) };
        var coordinator = new OptionalInstallsCacheCoordinator(client, cacheStore);
        var loader = new OptionalInstallsStartupLoader(coordinator);

        IReadOnlyList<OptionalInstallItem>? cachedItems = null;
        var refreshedCalled = false;
        var warning = await loader.InitializeAsync(
            items => cachedItems = items,
            _ => refreshedCalled = true,
            CancellationToken.None
        );

        Assert.Equal("CachedVLC", Assert.Single(cachedItems!).ItemName);
        Assert.False(refreshedCalled);
        Assert.Equal(string.Empty, warning);
        Assert.True(coordinator.State.IsCached);
        Assert.True(coordinator.State.HasRefreshFailure);
        Assert.Contains("service unavailable", coordinator.State.RefreshFailure!.Message);
    }

    [Fact]
    public async Task InitializeAsync_InvalidCacheStillLoadsSuccessfulLiveCatalog()
    {
        var now = DateTimeOffset.Parse("2026-02-19T18:11:00Z");
        var cacheStore = new InMemoryCacheStore(null)
        {
            LoadFailure = new InvalidDataException("cached protocol data is invalid"),
        };
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([MakeItem("FreshChrome", now)]),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, cacheStore);
        var loader = new OptionalInstallsStartupLoader(coordinator);
        var cachedCalled = false;
        IReadOnlyList<OptionalInstallItem>? refreshedItems = null;

        var warning = await loader.InitializeAsync(
            _ => cachedCalled = true,
            items => refreshedItems = items,
            CancellationToken.None
        );

        Assert.Equal(string.Empty, warning);
        Assert.False(cachedCalled);
        Assert.Equal("FreshChrome", Assert.Single(refreshedItems!).ItemName);
        Assert.Equal(1, client.ListCalls);
        Assert.True(coordinator.State.IsLive);
        Assert.False(coordinator.State.HasLoadFailure);
    }

    private static OptionalInstallItem MakeItem(string itemName, DateTimeOffset now) => new(
        itemName,
        itemName,
        "1.0.0",
        "testcatalog",
        "nupkg",
        itemName,
        $"packages/{itemName}/{itemName}.nupkg",
        true,
        false,
        OptionalInstallStatus.NotInstalled,
        now,
        null
    );

    private sealed class InMemoryCacheStore : IOptionalInstallsCacheStore
    {
        private OptionalInstallsCacheDocument? _document;

        public InMemoryCacheStore(OptionalInstallsCacheDocument? document) => _document = document;

        public Exception? LoadFailure { get; init; }

        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
        {
            if (LoadFailure is not null)
            {
                return Task.FromException<OptionalInstallsCacheDocument?>(LoadFailure);
            }

            return Task.FromResult(_document);
        }

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
        {
            _document = document;
            return Task.CompletedTask;
        }
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public Func<CancellationToken, Task<IReadOnlyList<OptionalInstallItem>>> ListAsync { get; init; } = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]);
        public int ListCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            ListCalls++;
            return ListAsync(cancellationToken);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken) => throw new NotSupportedException();
        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken) => throw new NotSupportedException();

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
