using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Tests;

public sealed class CatalogRefreshStateTests
{
    [Fact]
    public async Task NoCache_LivePending_RemainsInitialLoadingUntilLiveSucceeds()
    {
        var pending = new TaskCompletionSource<IReadOnlyList<OptionalInstallItem>>(
            TaskCreationOptions.RunContinuationsAsynchronously
        );
        var client = new FakeClient { ListAsync = _ => pending.Task };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new TestCacheStore());

        Assert.Null(await coordinator.LoadCachedAsync(CancellationToken.None));
        var refresh = coordinator.RefreshAsync(CancellationToken.None);
        await Task.Yield();

        Assert.True(coordinator.State.IsInitialLoading);
        Assert.True(coordinator.State.IsRefreshing);
        Assert.False(coordinator.State.HasUsableData);

        pending.SetResult([Item("one", "One")]);
        await refresh;

        Assert.True(coordinator.State.IsLive);
        Assert.False(coordinator.State.IsInitialLoading);
        Assert.False(coordinator.State.IsRefreshing);
        Assert.NotNull(coordinator.State.LastSuccessfulRefreshUtc);
    }

    [Fact]
    public async Task NoCache_LiveEmpty_IsSuccessfulEmpty_NotLoadFailure()
    {
        var client = new FakeClient { ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]) };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new TestCacheStore());

        await coordinator.RefreshAsync(CancellationToken.None);

        Assert.True(coordinator.State.IsLive);
        Assert.True(coordinator.State.IsSuccessfulEmpty);
        Assert.False(coordinator.State.HasLoadFailure);
    }

    [Fact]
    public async Task NoCache_LiveFailure_IsLoadFailureWithoutUsableData()
    {
        var failure = new IOException("pipe unavailable");
        var client = new FakeClient { ListAsync = _ => Task.FromException<IReadOnlyList<OptionalInstallItem>>(failure) };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new TestCacheStore());

        await Assert.ThrowsAsync<IOException>(() => coordinator.RefreshAsync(CancellationToken.None));

        Assert.False(coordinator.State.HasUsableData);
        Assert.True(coordinator.State.HasLoadFailure);
        Assert.Same(failure, coordinator.State.LoadFailure);
        Assert.Null(coordinator.State.LastSuccessfulRefreshUtc);
    }

    [Fact]
    public async Task CacheFirst_CacheIsExplicitUntilLiveSucceeds()
    {
        var cachedAt = DateTimeOffset.Parse("2026-09-12T18:00:00Z");
        var liveReady = new TaskCompletionSource<IReadOnlyList<OptionalInstallItem>>(
            TaskCreationOptions.RunContinuationsAsynchronously
        );
        var store = new TestCacheStore
        {
            Document = new OptionalInstallsCacheDocument(cachedAt, [Item("cached", "Cached")]),
        };
        var client = new FakeClient { ListAsync = _ => liveReady.Task };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);
        IReadOnlyList<OptionalInstallItem>? appliedCache = null;
        IReadOnlyList<OptionalInstallItem>? appliedLive = null;
        var loader = new OptionalInstallsStartupLoader(coordinator);

        var initialize = loader.InitializeAsync(
            items => appliedCache = items,
            items => appliedLive = items,
            CancellationToken.None
        );
        await Task.Yield();

        Assert.NotNull(appliedCache);
        Assert.Null(appliedLive);
        Assert.True(coordinator.State.IsCached);
        Assert.Equal(cachedAt, coordinator.State.CachedAtUtc);
        Assert.Null(coordinator.State.LastSuccessfulRefreshUtc);

        liveReady.SetResult([Item("live", "Live")]);
        await initialize;

        Assert.NotNull(appliedLive);
        Assert.True(coordinator.State.IsLive);
        Assert.NotNull(coordinator.State.LastSuccessfulRefreshUtc);
        Assert.Null(coordinator.State.RefreshFailure);
    }

    [Fact]
    public async Task CacheFirst_LiveFailure_PreservesCachedStateAndTimestamp()
    {
        var cachedAt = DateTimeOffset.Parse("2026-09-12T18:00:00Z");
        var store = new TestCacheStore
        {
            Document = new OptionalInstallsCacheDocument(cachedAt, [Item("cached", "Cached")]),
        };
        var failure = new TimeoutException("timed out");
        var client = new FakeClient { ListAsync = _ => Task.FromException<IReadOnlyList<OptionalInstallItem>>(failure) };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        await coordinator.LoadCachedAsync(CancellationToken.None);
        await Assert.ThrowsAsync<TimeoutException>(() => coordinator.RefreshAsync(CancellationToken.None));

        Assert.True(coordinator.State.IsCached);
        Assert.True(coordinator.State.HasUsableData);
        Assert.Same(failure, coordinator.State.RefreshFailure);
        Assert.Equal(cachedAt, coordinator.State.CachedAtUtc);
        Assert.Null(coordinator.State.LastSuccessfulRefreshUtc);
    }

    [Fact]
    public async Task CacheSaveFailure_DoesNotInvalidateSuccessfulLiveRefresh()
    {
        var store = new TestCacheStore { SaveFailure = new IOException("disk full") };
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([Item("live", "Live")]),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        var result = await coordinator.RefreshAsync(CancellationToken.None);

        Assert.Single(result.Items);
        Assert.NotNull(result.CacheWriteFailure);
        Assert.True(coordinator.State.IsLive);
        Assert.True(coordinator.State.HasUsableData);
        Assert.True(coordinator.State.HasCacheWriteFailure);
        Assert.False(coordinator.State.HasRefreshFailure);
        Assert.NotNull(coordinator.State.LastSuccessfulRefreshUtc);
    }

    [Fact]
    public async Task ConcurrentRefreshes_ShareOneLiveRequest()
    {
        var pending = new TaskCompletionSource<IReadOnlyList<OptionalInstallItem>>(
            TaskCreationOptions.RunContinuationsAsynchronously
        );
        var client = new FakeClient { ListAsync = _ => pending.Task };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new TestCacheStore());

        var first = coordinator.RefreshAsync(CancellationToken.None);
        var second = coordinator.RefreshAsync(CancellationToken.None);
        await Task.Yield();

        Assert.Equal(1, client.ListCalls);
        pending.SetResult([Item("one", "One")]);
        await Task.WhenAll(first, second);
        Assert.Equal(1, client.ListCalls);
    }

    [Fact]
    public async Task HomeViewModel_RefreshPreservesSearchSelectionIdentityAndActivity()
    {
        var firstItem = Item("one", "One", "1.0");
        var operation = new OperationStatusEvent(
            "operation-1",
            OperationState.Completed,
            100,
            "done",
            DateTimeOffset.Parse("2026-09-12T18:00:00Z"),
            "one",
            CatalogAction.Install,
            new Result(ResultOutcome.Succeeded, "ok", "Installed")
        );
        var responses = new Queue<IReadOnlyList<OptionalInstallItem>>();
        responses.Enqueue([firstItem]);
        responses.Enqueue([Item("one", "One updated", "2.0"), Item("two", "Two")]);
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromResult(responses.Dequeue()),
            Operations = [operation],
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new TestCacheStore());
        var tracker = new OperationTracker(client);
        var viewModel = new HomeViewModel(client, coordinator, tracker);

        await viewModel.InitializeAsync(CancellationToken.None);
        viewModel.SearchQuery = "One";
        Assert.True(viewModel.SelectItem("one"));
        var canonical = viewModel.SelectedItem;
        Assert.NotNull(canonical);
        Assert.Single(viewModel.ActivityItems);

        await viewModel.RefreshCatalogAsync(CancellationToken.None);

        Assert.Equal("One", viewModel.SearchQuery);
        Assert.Equal("one", viewModel.SelectedItemName);
        Assert.Same(canonical, viewModel.SelectedItem);
        Assert.Equal("One updated", viewModel.SelectedItem!.DisplayName);
        Assert.Single(viewModel.ActivityItems);
        Assert.Equal("operation-1", viewModel.ActivityItems[0].OperationId);
    }

    [Fact]
    public async Task HomeViewModel_RefreshFailureKeepsExistingCatalogAndSuccessfulRetryClearsFailure()
    {
        var attempt = 0;
        var client = new FakeClient
        {
            ListAsync = _ => ++attempt switch
            {
                1 => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([Item("one", "One")]),
                2 => Task.FromException<IReadOnlyList<OptionalInstallItem>>(new IOException("service unavailable")),
                _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([Item("one", "One refreshed")]),
            },
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new TestCacheStore());
        var viewModel = new HomeViewModel(client, coordinator, new OperationTracker(client));

        await viewModel.InitializeAsync(CancellationToken.None);
        var canonical = viewModel.FindItem("one");
        var firstTimestamp = viewModel.CatalogState.LastSuccessfulRefreshUtc;

        await Assert.ThrowsAsync<IOException>(() => viewModel.RefreshCatalogAsync(CancellationToken.None));
        Assert.Same(canonical, viewModel.FindItem("one"));
        Assert.True(viewModel.CatalogState.HasRefreshFailure);
        Assert.Equal(firstTimestamp, viewModel.CatalogState.LastSuccessfulRefreshUtc);

        await viewModel.RefreshCatalogAsync(CancellationToken.None);
        Assert.Same(canonical, viewModel.FindItem("one"));
        Assert.Equal("One refreshed", viewModel.FindItem("one")!.DisplayName);
        Assert.False(viewModel.CatalogState.HasRefreshFailure);
        Assert.True(viewModel.CatalogState.IsLive);
        Assert.True(viewModel.CatalogState.LastSuccessfulRefreshUtc >= firstTimestamp);
    }

    [Fact]
    public async Task HomeViewModel_RemovedSelectedItemBecomesUnavailable()
    {
        var responses = new Queue<IReadOnlyList<OptionalInstallItem>>();
        responses.Enqueue([Item("one", "One")]);
        responses.Enqueue([Item("two", "Two")]);
        var client = new FakeClient { ListAsync = _ => Task.FromResult(responses.Dequeue()) };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new TestCacheStore());
        var viewModel = new HomeViewModel(client, coordinator, new OperationTracker(client));

        await viewModel.InitializeAsync(CancellationToken.None);
        Assert.True(viewModel.SelectItem("one"));

        await viewModel.RefreshCatalogAsync(CancellationToken.None);

        Assert.Null(viewModel.SelectedItemName);
        Assert.Null(viewModel.SelectedItem);
    }

    private static OptionalInstallItem Item(string itemName, string displayName, string version = "1.0")
    {
        var now = DateTimeOffset.Parse("2026-09-12T18:00:00Z");
        return new OptionalInstallItem(
            ItemName: itemName,
            DisplayName: displayName,
            Version: version,
            Catalog: "test",
            InstallerType: "ps1",
            InstallerPackageId: itemName,
            InstallerLocation: $"{itemName}.ps1",
            IsManaged: true,
            IsInstalled: false,
            Status: OptionalInstallStatus.NotInstalled,
            StatusUpdatedAtUtc: now,
            LastOperationId: null,
            TargetVersion: version,
            Observation: new Observation(
                ObservedState.Absent,
                InstalledVersion: null,
                CheckedAtUtc: now,
                DetailCode: string.Empty,
                InstallRequirement: RequirementState.Unknown
            ),
            Policy: null,
            Actions: new Actions(
                new ActionDecision(true, string.Empty),
                new ActionDecision(false, "Not installed")
            ),
            Description: "Test app"
        );
    }

    private sealed class TestCacheStore : IOptionalInstallsCacheStore
    {
        public OptionalInstallsCacheDocument? Document { get; set; }
        public Exception? SaveFailure { get; set; }

        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult(Document);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
        {
            if (SaveFailure is not null)
            {
                return Task.FromException(SaveFailure);
            }

            Document = document;
            return Task.CompletedTask;
        }
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public required Func<CancellationToken, Task<IReadOnlyList<OptionalInstallItem>>> ListAsync { get; init; }
        public IReadOnlyList<OperationStatusEvent> Operations { get; init; } = [];
        public int ListCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            ListCalls++;
            return ListAsync(cancellationToken);
        }

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
            => Task.FromResult(Operations);

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
