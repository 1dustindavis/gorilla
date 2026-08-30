using Gorilla.UI.Client;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public class HomeViewModelTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-02-19T18:10:00Z");

    [Fact]
    public async Task InstallAsync_RejectedOperation_SetsWarningAndClearsBusy()
    {
        var client = new FakeClient
        {
            InstallAsync = (itemName, _) => Task.FromResult(new OperationAccepted("op-1", false, Now)),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.False(item.IsBusy);
        Assert.Equal("Install was not accepted for VLC.", viewModel.WarningBanner);
        Assert.Equal(0, client.ListCalls);
    }

    [Fact]
    public async Task RemoveAsync_RejectedOperation_SetsWarningAndClearsBusy()
    {
        var client = new FakeClient
        {
            RemoveAsync = (itemName, _) => Task.FromResult(new OperationAccepted("op-2", false, Now)),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        await viewModel.RemoveAsync(item, CancellationToken.None);

        Assert.False(item.IsBusy);
        Assert.Equal("Remove was not accepted for VLC.", viewModel.WarningBanner);
        Assert.Equal(0, client.ListCalls);
    }

    [Fact]
    public async Task InstallAsync_SuccessfulStream_RefreshesAuthoritativeItems()
    {
        var client = new FakeClient
        {
            InstallAsync = (itemName, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, _) => Stream(
                new OperationStatusEvent("op-1", OperationState.Installing, 50, "Installing", Now),
                new OperationStatusEvent("op-1", OperationState.Succeeded, 100, "Installed", Now)
            ),
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([MakeProtocolItem("VLC", true)]),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");
        viewModel.Items.Add(item);
        viewModel.SetWarningBanner("old warning");

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.False(item.IsBusy);
        Assert.Equal(string.Empty, viewModel.WarningBanner);
        Assert.Equal(1, client.ListCalls);
        var refreshedItem = Assert.Single(viewModel.Items);
        Assert.Equal("VLC", refreshedItem.ItemName);
        Assert.True(refreshedItem.IsInstalled);
        Assert.Equal("Installed", refreshedItem.Status);
    }

    [Fact]
    public async Task RemoveAsync_SuccessfulStream_RefreshesAuthoritativeItems()
    {
        var client = new FakeClient
        {
            RemoveAsync = (itemName, _) => Task.FromResult(new OperationAccepted("op-2", true, Now)),
            StreamAsync = (_, _) => Stream(
                new OperationStatusEvent("op-2", OperationState.Removing, 50, "Removing", Now),
                new OperationStatusEvent("op-2", OperationState.Succeeded, 100, "Removed", Now)
            ),
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([MakeProtocolItem("VLC", false)]),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC", installed: true);
        viewModel.Items.Add(item);

        await viewModel.RemoveAsync(item, CancellationToken.None);

        Assert.False(item.IsBusy);
        Assert.Equal(1, client.ListCalls);
        var refreshedItem = Assert.Single(viewModel.Items);
        Assert.Equal("VLC", refreshedItem.ItemName);
        Assert.False(refreshedItem.IsInstalled);
        Assert.Equal("NotInstalled", refreshedItem.Status);
    }

    [Fact]
    public async Task InstallAsync_TerminalFailure_RefreshesItemsAndPreservesOperationWarning()
    {
        var client = new FakeClient
        {
            InstallAsync = (itemName, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, _) => Stream(
                new OperationStatusEvent("op-1", OperationState.Failed, 40, "Install failed", Now, "installer_failed", "exit code 1")
            ),
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([MakeProtocolItem("VLC", false)]),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");
        viewModel.Items.Add(item);

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Equal(1, client.ListCalls);
        Assert.Equal("Operation for VLC ended with Failed: exit code 1", viewModel.WarningBanner);
        Assert.False(Assert.Single(viewModel.Items).IsInstalled);
        Assert.False(item.IsBusy);
    }

    [Fact]
    public async Task InstallAsync_RefreshFailureAfterSuccess_SetsRefreshWarning()
    {
        var client = new FakeClient
        {
            InstallAsync = (itemName, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, _) => Stream(
                new OperationStatusEvent("op-1", OperationState.Succeeded, 100, "Installed", Now)
            ),
            ListAsync = _ => Task.FromException<IReadOnlyList<OptionalInstallItem>>(new IOException("refresh unavailable")),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Equal(1, client.ListCalls);
        Assert.Contains("Operation completed, but optional installs refresh failed:", viewModel.WarningBanner);
        Assert.Contains("refresh unavailable", viewModel.WarningBanner);
        Assert.False(item.IsBusy);
    }

    [Fact]
    public async Task InstallAsync_StreamFailure_SetsQueuedWarningAndDoesNotRefresh()
    {
        var client = new FakeClient
        {
            InstallAsync = (itemName, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, _) => ThrowingStream(new IOException("pipe closed")),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Contains("Install queued, but live status stream failed:", viewModel.WarningBanner);
        Assert.Contains("pipe closed", viewModel.WarningBanner);
        Assert.Equal(0, client.ListCalls);
        Assert.False(item.IsBusy);
    }

    [Fact]
    public async Task InstallAsync_RequestCancellation_DoesNotLeaveItemBusy()
    {
        var client = new FakeClient
        {
            InstallAsync = (itemName, _) => Task.FromException<OperationAccepted>(new OperationCanceledException()),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        await Assert.ThrowsAsync<OperationCanceledException>(() => viewModel.InstallAsync(item, CancellationToken.None));

        Assert.False(item.IsBusy);
    }

    [Fact]
    public async Task InitializeAsync_MapsProtocolItemsToPresentationItems()
    {
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([MakeProtocolItem("GoogleChrome", true)]),
        };
        var viewModel = CreateViewModel(client);

        await viewModel.InitializeAsync(CancellationToken.None);

        var item = Assert.Single(viewModel.Items);
        Assert.Equal("GoogleChrome", item.ItemName);
        Assert.Equal("GoogleChrome", item.DisplayName);
        Assert.True(item.IsInstalled);
        Assert.Equal("Installed", item.Status);
    }

    [Fact]
    public async Task FindItem_IsCaseInsensitive()
    {
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([MakeProtocolItem("GoogleChrome", false)]),
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        var item = viewModel.FindItem("googlechrome");

        Assert.NotNull(item);
        Assert.Equal("GoogleChrome", item!.ItemName);
    }

    private static HomeViewModel CreateViewModel(FakeClient client)
    {
        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        return new HomeViewModel(client, coordinator, new OperationTracker(client));
    }

    private static UiOptionalInstallItem MakeUiItem(string itemName, bool installed = false) => new()
    {
        ItemName = itemName,
        DisplayName = itemName,
        Version = "1.0.0",
        Status = installed ? "Installed" : "NotInstalled",
        IsInstalled = installed,
    };

    private static OptionalInstallItem MakeProtocolItem(string itemName, bool installed) => new(
        itemName,
        itemName,
        "1.0.0",
        "testcatalog",
        "nupkg",
        itemName,
        $"packages/{itemName}/{itemName}.nupkg",
        true,
        installed,
        installed ? OptionalInstallStatus.Installed : OptionalInstallStatus.NotInstalled,
        Now,
        null
    );

    private static async IAsyncEnumerable<OperationStatusEvent> Stream(params OperationStatusEvent[] events)
    {
        foreach (var update in events)
        {
            await Task.Yield();
            yield return update;
        }
    }

    private static async IAsyncEnumerable<OperationStatusEvent> ThrowingStream(Exception exception)
    {
        await Task.Yield();
        if (exception is not null)
        {
            throw exception;
        }

        yield break;
    }

    private sealed class InMemoryCacheStore : IOptionalInstallsCacheStore
    {
        private OptionalInstallsCacheDocument? _document;

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
        public Func<string, CancellationToken, Task<OperationAccepted>> InstallAsync { get; init; } = (itemName, _) => Task.FromResult(new OperationAccepted("op-install", true, Now));
        public Func<string, CancellationToken, Task<OperationAccepted>> RemoveAsync { get; init; } = (itemName, _) => Task.FromResult(new OperationAccepted("op-remove", true, Now));
        public Func<string, CancellationToken, IAsyncEnumerable<OperationStatusEvent>> StreamAsync { get; init; } = (_, _) => Stream();

        public int ListCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            ListCalls++;
            return ListAsync(cancellationToken);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken) => InstallAsync(itemName, cancellationToken);
        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken) => RemoveAsync(itemName, cancellationToken);
        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(string operationId, CancellationToken cancellationToken) => StreamAsync(operationId, cancellationToken);
    }
}
