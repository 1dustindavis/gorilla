using Gorilla.UI.Client;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public class HomeViewModelResidualMutationTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-02-19T18:10:00Z");

    [Fact]
    public async Task InstallAsync_StreamFailureAfterTerminalEvent_DoesNotRefresh()
    {
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, _) => TerminalThenThrowingStream(),
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Equal(0, client.ListCalls);
        Assert.Contains("Install queued, but live status stream failed:", viewModel.WarningBanner);
        Assert.Contains("pipe closed", viewModel.WarningBanner);
    }

    [Fact]
    public async Task InstallAsync_NonTerminalUpdate_DoesNotClearExistingWarning()
    {
        var client = new FakeClient
        {
            InstallAsync = (_, _) => Task.FromResult(new OperationAccepted("op-1", true, Now)),
            StreamAsync = (_, _) => Stream(
                new OperationStatusEvent("op-1", OperationState.Installing, 50, "Copying files", Now)
            ),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem("VLC");
        viewModel.SetWarningBanner("existing warning");

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Equal("existing warning", viewModel.WarningBanner);
        Assert.Equal("Installing: Copying files", item.Status);
        Assert.Equal(0, client.ListCalls);
    }

    private static HomeViewModel CreateViewModel(FakeClient client)
    {
        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        return new HomeViewModel(client, coordinator, new OperationTracker(client));
    }

    private static UiOptionalInstallItem MakeUiItem(string itemName) => new()
    {
        ItemName = itemName,
        DisplayName = itemName,
        Version = "1.0.0",
        Status = "NotInstalled",
        IsInstalled = false,
    };

    private static async IAsyncEnumerable<OperationStatusEvent> Stream(params OperationStatusEvent[] events)
    {
        foreach (var update in events)
        {
            await Task.Yield();
            yield return update;
        }
    }

    private static async IAsyncEnumerable<OperationStatusEvent> TerminalThenThrowingStream()
    {
        await Task.Yield();
        yield return new OperationStatusEvent("op-1", OperationState.Succeeded, 100, "Installed", Now);
        throw new IOException("pipe closed");
    }

    private sealed class InMemoryCacheStore : IOptionalInstallsCacheStore
    {
        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken) => Task.FromResult<OptionalInstallsCacheDocument?>(null);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken) => Task.CompletedTask;
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public Func<CancellationToken, Task<IReadOnlyList<OptionalInstallItem>>> ListAsync { get; init; } = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]);
        public Func<string, CancellationToken, Task<OperationAccepted>> InstallAsync { get; init; } = (_, _) => Task.FromResult(new OperationAccepted("op-install", true, Now));
        public Func<string, CancellationToken, Task<OperationAccepted>> RemoveAsync { get; init; } = (_, _) => Task.FromResult(new OperationAccepted("op-remove", true, Now));
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
