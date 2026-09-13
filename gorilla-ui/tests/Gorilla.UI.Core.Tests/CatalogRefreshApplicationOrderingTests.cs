using Gorilla.UI.Client;
using Gorilla.UI.Core;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public sealed class CatalogRefreshApplicationOrderingTests
{
    [Fact]
    public async Task Refresh_DoesNotPublishLiveOrCompleteUntilSnapshotIsAccepted()
    {
        var snapshotReceived = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var releaseApplication = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var applied = false;
        var client = new FakeClient
        {
            Items = [Item("fresh")],
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new NoOpCacheStore());

        var refresh = coordinator.RefreshAsync(
            async (items, _) =>
            {
                Assert.Equal("fresh", Assert.Single(items).ItemName);
                snapshotReceived.TrySetResult(true);
                await releaseApplication.Task;
                applied = true;
            },
            CancellationToken.None
        );

        await snapshotReceived.Task.WaitAsync(TimeSpan.FromSeconds(2));

        Assert.False(applied);
        Assert.False(refresh.IsCompleted);
        Assert.True(coordinator.State.IsRefreshing);
        Assert.False(coordinator.State.HasUsableData);
        Assert.False(coordinator.State.IsLive);
        Assert.Null(coordinator.State.LastSuccessfulRefreshUtc);

        // A refresh requested while reconciliation is still pending must join the
        // same operation instead of allowing the fetched-but-unapplied snapshot to
        // race with a newer request/application.
        var joined = coordinator.RefreshAsync(
            (_, _) => throw new InvalidOperationException("joined refresh must not replace the active reconciler"),
            CancellationToken.None
        );
        Assert.Same(refresh, joined);
        Assert.Equal(1, client.ListCalls);

        releaseApplication.TrySetResult(true);
        await refresh.WaitAsync(TimeSpan.FromSeconds(2));

        Assert.True(applied);
        Assert.True(coordinator.State.HasUsableData);
        Assert.True(coordinator.State.IsLive);
        Assert.False(coordinator.State.IsRefreshing);
        Assert.NotNull(coordinator.State.LastSuccessfulRefreshUtc);
        Assert.Equal(1, client.ListCalls);
    }

    private static OptionalInstallItem Item(string itemName)
    {
        var now = DateTimeOffset.Parse("2026-09-13T16:00:00Z");
        return new OptionalInstallItem(
            itemName,
            itemName,
            "1.0.0",
            "testcatalog",
            "ps1",
            itemName,
            $"{itemName}.ps1",
            true,
            false,
            OptionalInstallStatus.NotInstalled,
            now,
            null
        );
    }

    private sealed class NoOpCacheStore : IOptionalInstallsCacheStore
    {
        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult<OptionalInstallsCacheDocument?>(null);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
            => Task.CompletedTask;
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public required IReadOnlyList<OptionalInstallItem> Items { get; init; }
        public int ListCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            ListCalls++;
            return Task.FromResult(Items);
        }

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
