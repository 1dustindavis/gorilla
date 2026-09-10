using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using AppCatalog = Gorilla.UI.Client.AppCatalog;

namespace Gorilla.UI.Core.Tests;

public class HomeViewModelOperationResultTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-09T19:45:00Z");

    [Theory]
    [InlineData(Outcome.Succeeded, "Succeeded: Installed", "")]
    [InlineData(Outcome.AlreadySatisfied, "AlreadySatisfied: Already installed", "")]
    [InlineData(Outcome.Failed, "Failed: Installer exited with code 1", "Operation for VLC ended with Failed: Installer exited with code 1")]
    [InlineData(Outcome.Unverified, "Unverified: Unable to confirm installed state", "Operation for VLC ended with Unverified: Unable to confirm installed state")]
    [InlineData(Outcome.Interrupted, "Interrupted: Service operation was interrupted", "Operation for VLC ended with Interrupted: Service operation was interrupted")]
    public async Task InstallAsync_UsesAuthoritativeOutcome(
        Outcome outcome,
        string expectedStatus,
        string expectedWarning)
    {
        var resultMessage = outcome switch
        {
            Outcome.Succeeded => "Installed",
            Outcome.AlreadySatisfied => "Already installed",
            Outcome.Failed => "Installer exited with code 1",
            Outcome.Unverified => "Unable to confirm installed state",
            Outcome.Interrupted => "Service operation was interrupted",
            _ => throw new ArgumentOutOfRangeException(nameof(outcome)),
        };

        var client = new FakeClient
        {
            StreamAsync = (_, _) => Stream(new OperationStatusEvent(
                OperationId: "op-1",
                State: OperationState.Completed,
                ProgressPercent: null,
                Message: resultMessage,
                TimestampUtc: Now,
                ItemName: "VLC",
                Action: AppCatalog.Action.Install,
                Result: new Result(outcome, OutcomeCode(outcome), Message: resultMessage)
            )),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem();

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Equal(expectedStatus, item.Status);
        Assert.Equal(expectedWarning, viewModel.WarningBanner);
    }

    [Fact]
    public async Task InstallAsync_RejectsMismatchedStreamIdentityAsStatusFailure()
    {
        var client = new FakeClient
        {
            StreamAsync = (_, _) => Stream(new OperationStatusEvent(
                OperationId: "op-1",
                State: OperationState.Completed,
                ProgressPercent: null,
                Message: "Installed",
                TimestampUtc: Now,
                ItemName: "DifferentItem",
                Action: AppCatalog.Action.Install,
                Result: new Result(Outcome.Succeeded, "completed", Message: "Installed")
            )),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem();

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Contains("Install was accepted, but operation status is temporarily unavailable", viewModel.WarningBanner);
        Assert.Contains("identity mismatch", viewModel.WarningBanner);
        Assert.Equal(0, client.ListCalls);
    }

    [Fact]
    public async Task InstallAsync_RejectsMismatchedStreamActionAsStatusFailure()
    {
        var client = new FakeClient
        {
            StreamAsync = (_, _) => Stream(new OperationStatusEvent(
                OperationId: "op-1",
                State: OperationState.Completed,
                ProgressPercent: null,
                Message: "Installed",
                TimestampUtc: Now,
                ItemName: "VLC",
                Action: AppCatalog.Action.Remove,
                Result: new Result(Outcome.Succeeded, "completed", Message: "Installed")
            )),
        };
        var viewModel = CreateViewModel(client);
        var item = MakeUiItem();

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Contains("Install was accepted, but operation status is temporarily unavailable", viewModel.WarningBanner);
        Assert.Contains("action mismatch", viewModel.WarningBanner);
        Assert.Equal(0, client.ListCalls);
    }

    [Fact]
    public async Task InitializeAsync_UsesServiceOwnedActionAvailability()
    {
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([
                MakeProtocolItem(
                    install: new ActionDecision(false, "Already selected for installation"),
                    remove: new ActionDecision(true, "")
                )
            ]),
        };
        var viewModel = CreateViewModel(client);

        await viewModel.InitializeAsync(CancellationToken.None);

        var item = Assert.Single(viewModel.Items);
        Assert.False(item.InstallAllowed);
        Assert.False(item.CanInstall);
        Assert.Equal("Already selected for installation", item.InstallUnavailableReason);
        Assert.True(item.RemoveAllowed);
        Assert.True(item.CanRemove);

        item.IsBusy = true;
        Assert.False(item.CanInstall);
        Assert.False(item.CanRemove);

        item.IsBusy = false;
        Assert.False(item.CanInstall);
        Assert.True(item.CanRemove);
    }

    [Fact]
    public async Task InitializeAsync_MissingActionPolicyDoesNotGuessAvailability()
    {
        var item = MakeProtocolItem(
            install: new ActionDecision(true, ""),
            remove: new ActionDecision(false, "Not installed")
        ) with { Actions = null };
        var client = new FakeClient
        {
            ListAsync = _ => Task.FromResult<IReadOnlyList<OptionalInstallItem>>([item]),
        };
        var viewModel = CreateViewModel(client);

        await viewModel.InitializeAsync(CancellationToken.None);

        var uiItem = Assert.Single(viewModel.Items);
        Assert.False(uiItem.CanInstall);
        Assert.False(uiItem.CanRemove);
        Assert.Equal("Refresh required before installing.", uiItem.InstallUnavailableReason);
        Assert.Equal("Refresh required before removing.", uiItem.RemoveUnavailableReason);
    }

    private static string OutcomeCode(Outcome outcome) => outcome switch
    {
        Outcome.Succeeded => "completed",
        Outcome.AlreadySatisfied => "already_satisfied",
        Outcome.Failed => "execution_failed",
        Outcome.Unverified => "verification_unavailable",
        Outcome.Interrupted => "interrupted",
        _ => "unknown",
    };

    private static HomeViewModel CreateViewModel(FakeClient client)
    {
        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        return new HomeViewModel(client, coordinator, new OperationTracker(client));
    }

    private static UiOptionalInstallItem MakeUiItem() => new()
    {
        ItemName = "VLC",
        DisplayName = "VLC",
        Version = "1.0",
        Status = "NotInstalled",
        IsInstalled = false,
        InstallAllowed = true,
        RemoveAllowed = false,
    };

    private static OptionalInstallItem MakeProtocolItem(ActionDecision install, ActionDecision remove) => new(
        ItemName: "VLC",
        DisplayName: "VLC",
        Version: "1.0",
        Catalog: "test",
        InstallerType: "msi",
        InstallerPackageId: "VLC",
        InstallerLocation: "vlc.msi",
        IsManaged: true,
        IsInstalled: true,
        Status: OptionalInstallStatus.Installed,
        StatusUpdatedAtUtc: Now,
        LastOperationId: null,
        Actions: new Actions(install, remove)
    );

    private static async IAsyncEnumerable<OperationStatusEvent> Stream(params OperationStatusEvent[] events)
    {
        foreach (var update in events)
        {
            await Task.Yield();
            yield return update;
        }
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
        public Func<CancellationToken, Task<IReadOnlyList<OptionalInstallItem>>> ListAsync { get; init; } = _ =>
            Task.FromResult<IReadOnlyList<OptionalInstallItem>>([]);
        public Func<string, CancellationToken, IAsyncEnumerable<OperationStatusEvent>> StreamAsync { get; init; } = (_, _) => Stream();

        public int ListCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            ListCalls++;
            return ListAsync(cancellationToken);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken) =>
            Task.FromResult(new OperationAccepted("op-1", true, Now));

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken) =>
            Task.FromResult(new OperationAccepted("op-2", true, Now));

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(string operationId, CancellationToken cancellationToken) =>
            StreamAsync(operationId, cancellationToken);
    }
}
