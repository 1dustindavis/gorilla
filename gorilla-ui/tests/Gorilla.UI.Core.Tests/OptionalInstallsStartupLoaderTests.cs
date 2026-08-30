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
        var loader = new OptionalInstallsStartupLoader(new OptionalInstallsCacheCoordinator(client, cacheStore));

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

        var refreshNow = DateTimeOffset.Parse("2026-02-19T18:11:00Z");
        refreshReady.SetResult([MakeItem("FreshChrome", refreshNow)]);
        var warning = await initializeTask;

        Assert.Equal(string.Empty, warning);
        Assert.Equal("CachedVLC", Assert.Single(cachedItems!).ItemName);
        Assert.Equal("FreshChrome", Assert.Single(refreshedItems!).ItemName);
        Assert.True(applyOrder.TryDequeue(out var first));
        Assert.Equal("cached", first);
        Assert.True(applyOrder.TryDequeue(out var second));
        Assert.Equal("refreshed", second);
    }

    [Fact]
    public async Task InitializeAsync_RefreshFailureKeepsCachedAndReturnsWarning()
    {
        var now = DateTimeOffset.Parse("2026-02-19T18:10:00Z");
        var cacheStore = new InMemoryCacheStore(new OptionalInstallsCacheDocument(now, [MakeItem("CachedVLC", now)]));
        var client = new FakeClient { ListAsync = _ => Task.FromException<IReadOnlyList<OptionalInstallItem>>(new InvalidOperationException("service unavailable")) };
        var loader = new OptionalInstallsStartupLoader(new OptionalInstallsCacheCoordinator(client, cacheStore));

        IReadOnlyList<OptionalInstallItem>? cachedItems = null;
        var refreshedCalled = false;
        var warning = await loader.InitializeAsync(
            items => cachedItems = items,
            _ => refreshedCalled = true,
            CancellationToken.None
        );

        Assert.Equal("CachedVLC", Assert.Single(cachedItems!).ItemName);
        Assert.False(refreshedCalled);
        Assert.Contains("Showing cached data. Refresh failed:", warning);
        Assert.Contains("service unavailable", warning);
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

        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken) => Task.FromResult(_document);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
        {
            _document = document;
            return Task.CompletedTask;
        }
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public Func<CancellationToken, Task<IReadOnlyList<OptionalInstallItem>>> ListAsync { get; init; } = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]);

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken) => ListAsync(cancellationToken);
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
