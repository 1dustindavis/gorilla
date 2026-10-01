using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Core.Models;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public sealed class CatalogRefreshPreexistingLiveRegressionTests
{
    private static readonly DateTimeOffset T0900 = DateTimeOffset.Parse("2026-09-30T09:00:00Z");
    private static readonly DateTimeOffset T1000 = DateTimeOffset.Parse("2026-09-30T10:00:00Z");
    private static readonly DateTimeOffset T1005 = DateTimeOffset.Parse("2026-09-30T10:05:00Z");

    [Fact]
    public async Task RefreshAsync_FinalIdleOlderThanPreexistingLiveSnapshot_IsRefreshFailureWithoutRegression()
    {
        var client = new SequenceClient();
        client.Enqueue(Result("preexisting-live", T1005, CatalogRefreshState.Idle));
        client.Enqueue(Result("regressed-terminal", T1000, CatalogRefreshState.Idle));
        var coordinator = new OptionalInstallsCacheCoordinator(client, new NoOpCacheStore(), TimeSpan.Zero);

        await coordinator.ReadLatestAsync((_, _) => Task.CompletedTask, CancellationToken.None);
        Assert.Equal(CatalogDataSource.Live, coordinator.State.DataSource);
        Assert.Equal(T1005, coordinator.State.LastSuccessfulRefreshUtc);

        var acceptCalls = 0;
        await Assert.ThrowsAsync<CatalogRefreshException>(() => coordinator.RefreshAsync(
            (_, _) =>
            {
                acceptCalls++;
                return Task.CompletedTask;
            },
            CancellationToken.None
        ));

        Assert.Equal(0, acceptCalls);
        Assert.Equal(CatalogDataSource.Live, coordinator.State.DataSource);
        Assert.Equal(T1005, coordinator.State.LastSuccessfulRefreshUtc);
        Assert.IsType<CatalogRefreshException>(coordinator.State.RefreshFailure);
    }

    private static OptionalInstallsSnapshotResult Result(
        string name,
        DateTimeOffset generatedAt,
        CatalogRefreshState state
    ) => new(
        Items: [Item(name)],
        SnapshotAvailable: true,
        SnapshotGeneratedAtUtc: generatedAt,
        RefreshState: state,
        RefreshRequestedAtUtc: null,
        RefreshCompletedAtUtc: null,
        RefreshErrorCode: null
    );

    private static OptionalInstallItem Item(string name) => new(
        ItemName: name,
        DisplayName: name,
        Version: "1.0",
        Catalog: "test",
        InstallerType: "msi",
        InstallerPackageId: name,
        InstallerLocation: $"{name}.msi",
        IsManaged: false,
        IsInstalled: false,
        Status: OptionalInstallStatus.NotInstalled,
        StatusUpdatedAtUtc: T0900,
        LastOperationId: null
    );

    private sealed class SequenceClient : IGorillaServiceClient
    {
        private readonly Queue<OptionalInstallsSnapshotResult> _responses = new();

        public void Enqueue(OptionalInstallsSnapshotResult response) => _responses.Enqueue(response);

        public Task<OptionalInstallsSnapshotResult> ListOptionalInstallsAsync(
            bool refresh,
            CancellationToken cancellationToken
        ) => Task.FromResult(_responses.Dequeue());

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public async IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            [EnumeratorCancellation] CancellationToken cancellationToken
        )
        {
            await Task.CompletedTask;
            yield break;
        }
    }

    private sealed class NoOpCacheStore : IOptionalInstallsCacheStore
    {
        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult<OptionalInstallsCacheDocument?>(null);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
            => Task.CompletedTask;
    }
}
