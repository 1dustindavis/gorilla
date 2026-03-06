using Gorilla.UI.App.Models;
using Gorilla.UI.App.Services;
using Gorilla.UI.App.ViewModels;
using Gorilla.UI.Client;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class HomeViewModelInstallFlowTests
{
    [Fact]
    public async Task InstallAsync_SucceededTerminalState_RefreshesItemsFromService()
    {
        var state = new FakeState();
        state.ListResponses.Enqueue(
            [
                BuildItem("Slack", isInstalled: true, status: OptionalInstallStatus.Installed),
            ]
        );
        state.InstallResult = new OperationAccepted(
            OperationId: "op-install-1",
            Accepted: true,
            QueuedAtUtc: DateTimeOffset.Parse("2026-03-01T18:10:00Z")
        );
        state.StreamEvents =
        [
            BuildStatus("op-install-1", OperationState.Queued, "queued"),
            BuildStatus("op-install-1", OperationState.Installing, "installing"),
            BuildStatus("op-install-1", OperationState.Succeeded, "done"),
        ];

        var client = new FakeClient(state);
        var cacheCoordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore(null));
        var tracker = new OperationTracker(client);
        var viewModel = new HomeViewModel(client, cacheCoordinator, tracker);

        viewModel.Items.Add(new UiOptionalInstallItem
        {
            ItemName = "Slack",
            DisplayName = "Slack",
            Version = "1.0.0",
            Status = OptionalInstallStatus.NotInstalled.ToString(),
            IsInstalled = false,
        });

        var item = viewModel.FindItem("Slack");
        Assert.NotNull(item);

        await viewModel.InstallAsync(item!, CancellationToken.None);

        var updated = viewModel.FindItem("Slack");
        Assert.NotNull(updated);
        Assert.True(updated!.IsInstalled);
        Assert.Equal(OptionalInstallStatus.Installed.ToString(), updated.Status);
        Assert.Equal(1, state.ListCalls);
    }

    [Fact]
    public async Task InstallAsync_FailedTerminalState_RefreshesItemsFromService()
    {
        var state = new FakeState();
        state.ListResponses.Enqueue(
            [
                BuildItem("Slack", isInstalled: false, status: OptionalInstallStatus.NotInstalled),
            ]
        );
        state.InstallResult = new OperationAccepted(
            OperationId: "op-install-2",
            Accepted: true,
            QueuedAtUtc: DateTimeOffset.Parse("2026-03-01T18:10:00Z")
        );
        state.StreamEvents =
        [
            BuildStatus("op-install-2", OperationState.Queued, "queued"),
            BuildStatus("op-install-2", OperationState.Failed, "failed", errorMessage: "install failed"),
        ];

        var client = new FakeClient(state);
        var cacheCoordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore(null));
        var tracker = new OperationTracker(client);
        var viewModel = new HomeViewModel(client, cacheCoordinator, tracker);

        viewModel.Items.Add(new UiOptionalInstallItem
        {
            ItemName = "Slack",
            DisplayName = "Slack",
            Version = "1.0.0",
            Status = OptionalInstallStatus.NotInstalled.ToString(),
            IsInstalled = false,
        });

        var item = viewModel.FindItem("Slack");
        Assert.NotNull(item);

        await viewModel.InstallAsync(item!, CancellationToken.None);

        var unchanged = viewModel.FindItem("Slack");
        Assert.NotNull(unchanged);
        Assert.False(unchanged!.IsInstalled);
        Assert.Equal(1, state.ListCalls);
        Assert.Contains("Failed", viewModel.WarningBanner);
    }

    private static OptionalInstallItem BuildItem(string itemName, bool isInstalled, OptionalInstallStatus status)
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
            IsInstalled: isInstalled,
            Status: status,
            StatusUpdatedAtUtc: DateTimeOffset.Parse("2026-03-01T18:10:00Z"),
            LastOperationId: null
        );
    }

    private static OperationStatusEvent BuildStatus(
        string operationId,
        OperationState state,
        string message,
        string? errorMessage = null
    )
    {
        return new OperationStatusEvent(
            OperationId: operationId,
            State: state,
            ProgressPercent: 50,
            Message: message,
            TimestampUtc: DateTimeOffset.Parse("2026-03-01T18:10:00Z"),
            ErrorCode: state == OperationState.Failed ? "failed" : null,
            ErrorMessage: errorMessage,
            CanceledBy: state == OperationState.Canceled ? "service" : null
        );
    }

    private sealed class FakeState
    {
        public Queue<IReadOnlyList<OptionalInstallItem>> ListResponses { get; } = new();
        public OperationAccepted InstallResult { get; set; } = new("op-default", true, DateTimeOffset.UtcNow);
        public IReadOnlyList<OperationStatusEvent> StreamEvents { get; set; } = [];
        public int ListCalls { get; set; }
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        private readonly FakeState _state;

        public FakeClient(FakeState state)
        {
            _state = state;
        }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            _state.ListCalls++;
            if (_state.ListResponses.Count == 0)
            {
                return Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]);
            }
            return Task.FromResult(_state.ListResponses.Dequeue());
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
        {
            return Task.FromResult(_state.InstallResult);
        }

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
        {
            throw new NotSupportedException();
        }

        public async IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            [System.Runtime.CompilerServices.EnumeratorCancellation] CancellationToken cancellationToken
        )
        {
            await Task.CompletedTask;
            foreach (var item in _state.StreamEvents)
            {
                yield return item;
            }
        }
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
}
