using System.Collections.Concurrent;
using Gorilla.UI.Client;
using Xunit;

namespace Gorilla.UI.Client.Tests;

public class OptionalInstallsStartupLoaderTests
{
    [Fact]
    public async Task InitializeAsync_AppliesCachedBeforeRefreshCompletes()
    {
        var cachedNow = DateTimeOffset.Parse("2026-02-19T18:10:00Z");
        var cacheStore = new InMemoryCacheStore(
            new OptionalInstallsCacheDocument(
                CachedAtUtc: cachedNow,
                Items:
                [
                    MakeItem("CachedVLC", OptionalInstallStatus.NotInstalled, cachedNow),
                ]
            )
        );

        var refreshReady = new TaskCompletionSource<IReadOnlyList<OptionalInstallItem>>(TaskCreationOptions.RunContinuationsAsynchronously);
        var client = new ControllableClient(
            _ => refreshReady.Task,
            installFunc: _ => throw new NotSupportedException(),
            removeFunc: _ => throw new NotSupportedException()
        );
        var coordinator = new OptionalInstallsCacheCoordinator(client, cacheStore);
        var loader = new OptionalInstallsStartupLoader(coordinator);

        var applyOrder = new ConcurrentQueue<string>();
        var cachedApplied = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        var refreshedApplied = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        IReadOnlyList<OptionalInstallItem>? cachedItems = null;
        IReadOnlyList<OptionalInstallItem>? refreshedItems = null;

        var initializeTask = loader.InitializeAsync(
            applyCachedItems: items =>
            {
                cachedItems = items;
                applyOrder.Enqueue("cached");
                cachedApplied.TrySetResult();
            },
            applyRefreshedItems: items =>
            {
                refreshedItems = items;
                applyOrder.Enqueue("refreshed");
                refreshedApplied.TrySetResult();
            },
            cancellationToken: CancellationToken.None
        );

        await cachedApplied.Task.WaitAsync(TimeSpan.FromSeconds(2));
        Assert.False(refreshedApplied.Task.IsCompleted);

        var refreshNow = DateTimeOffset.Parse("2026-02-19T18:11:00Z");
        refreshReady.SetResult(
            [
                MakeItem("FreshChrome", OptionalInstallStatus.Installed, refreshNow),
            ]
        );

        var warning = await initializeTask;

        Assert.Equal(string.Empty, warning);
        Assert.NotNull(cachedItems);
        Assert.Single(cachedItems!);
        Assert.Equal("CachedVLC", cachedItems[0].ItemName);

        Assert.NotNull(refreshedItems);
        Assert.Single(refreshedItems!);
        Assert.Equal("FreshChrome", refreshedItems[0].ItemName);

        Assert.True(applyOrder.TryDequeue(out var first));
        Assert.Equal("cached", first);
        Assert.True(applyOrder.TryDequeue(out var second));
        Assert.Equal("refreshed", second);
    }

    [Fact]
    public async Task InitializeAsync_RefreshFailureKeepsCachedAndReturnsWarning()
    {
        var cachedNow = DateTimeOffset.Parse("2026-02-19T18:10:00Z");
        var cacheStore = new InMemoryCacheStore(
            new OptionalInstallsCacheDocument(
                CachedAtUtc: cachedNow,
                Items:
                [
                    MakeItem("CachedVLC", OptionalInstallStatus.NotInstalled, cachedNow),
                ]
            )
        );
        var client = new ControllableClient(
            listFunc: _ => throw new InvalidOperationException("service unavailable"),
            installFunc: _ => throw new NotSupportedException(),
            removeFunc: _ => throw new NotSupportedException()
        );
        var coordinator = new OptionalInstallsCacheCoordinator(client, cacheStore);
        var loader = new OptionalInstallsStartupLoader(coordinator);

        IReadOnlyList<OptionalInstallItem>? cachedItems = null;
        var refreshedCalled = false;

        var warning = await loader.InitializeAsync(
            applyCachedItems: items => cachedItems = items,
            applyRefreshedItems: _ => refreshedCalled = true,
            cancellationToken: CancellationToken.None
        );

        Assert.NotNull(cachedItems);
        Assert.Single(cachedItems!);
        Assert.Equal("CachedVLC", cachedItems[0].ItemName);
        Assert.False(refreshedCalled);
        Assert.Contains("Showing cached data. Refresh failed:", warning);
        Assert.Contains("service unavailable", warning);
    }

    private static OptionalInstallItem MakeItem(string itemName, OptionalInstallStatus status, DateTimeOffset now)
    {
        return new OptionalInstallItem(
            ItemName: itemName,
            DisplayName: itemName,
            Version: "1.0.0",
            Catalog: "testcatalog",
            InstallerType: "nupkg",
            InstallerPackageId: itemName,
            InstallerLocation: $"packages/{itemName}/{itemName}.nupkg",
            IsManaged: true,
            IsInstalled: status == OptionalInstallStatus.Installed,
            Status: status,
            StatusUpdatedAtUtc: now,
            LastOperationId: null
        );
    }

    private sealed class InMemoryCacheStore : IOptionalInstallsCacheStore
    {
        private OptionalInstallsCacheDocument? _document;

        public InMemoryCacheStore(OptionalInstallsCacheDocument? document)
        {
            _document = document;
        }

        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
        {
            return Task.FromResult(_document);
        }

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
        {
            _document = document;
            return Task.CompletedTask;
        }
    }

    private sealed class ControllableClient : IGorillaServiceClient
    {
        private readonly Func<CancellationToken, Task<IReadOnlyList<OptionalInstallItem>>> _listFunc;
        private readonly Func<string, Task<OperationAccepted>> _installFunc;
        private readonly Func<string, Task<OperationAccepted>> _removeFunc;

        public ControllableClient(
            Func<CancellationToken, Task<IReadOnlyList<OptionalInstallItem>>> listFunc,
            Func<string, Task<OperationAccepted>> installFunc,
            Func<string, Task<OperationAccepted>> removeFunc
        )
        {
            _listFunc = listFunc;
            _installFunc = installFunc;
            _removeFunc = removeFunc;
        }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            return _listFunc(cancellationToken);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
        {
            return _installFunc(itemName);
        }

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
        {
            return _removeFunc(itemName);
        }

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
