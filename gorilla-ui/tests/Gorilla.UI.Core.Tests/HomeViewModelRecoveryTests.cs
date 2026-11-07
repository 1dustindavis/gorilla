using System.ComponentModel;
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

public class HomeViewModelRecoveryTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-10T05:00:00Z");

    [Fact]
    public async Task InitializeAsync_ProjectsRecoveredActiveOperationOntoRefreshedCard()
    {
        using var cancellation = new CancellationTokenSource();
        var streamStarted = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var client = new FakeClient
        {
            Operations = [ActiveOperation()],
            StreamAsync = (_, token) => ActiveStream(streamStarted, token),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        var viewModel = new HomeViewModel(client, coordinator, new OperationTracker(client));

        await viewModel.InitializeAsync(cancellation.Token);
        await streamStarted.Task;

        var item = Assert.Single(viewModel.Items);
        Assert.Equal("VLC", item.ItemName);
        Assert.True(item.IsBusy);
        Assert.False(item.CanInstall);
        Assert.False(item.CanRemove);
        Assert.Equal("Installing: Installing item via managed run", item.Status);
        Assert.Equal([true], client.RefreshArguments);

        cancellation.Cancel();
    }

    [Fact]
    public async Task InstallAsync_StreamFailuresThenActiveReconciliation_ContinuesUntilCompletionAndClearsRecoveredStatusWarning()
    {
        using var cancellation = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        var warnings = new List<string>();
        var client = new FakeClient
        {
            Operations = [ActiveOperation()],
        };
        client.StreamAsync = (_, _) => client.StreamCalls switch
        {
            <= 2 => ThrowingStream(new IOException("pipe closed")),
            _ => CompletedStream(),
        };

        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        var viewModel = new HomeViewModel(client, coordinator, new OperationTracker(client));
        viewModel.PropertyChanged += (_, args) =>
        {
            if (args.PropertyName == nameof(HomeViewModel.InfrastructureWarning))
            {
                warnings.Add(viewModel.WarningBanner);
            }
        };
        var item = MakeUiItem();

        await viewModel.InstallAsync(item, cancellation.Token);

        Assert.False(item.IsBusy);
        Assert.Equal("Succeeded: Installed", item.Status);
        Assert.Equal(3, client.StreamCalls);
        Assert.Equal(1, client.ListOperationsCalls);
        Assert.Contains(
            "Install was accepted, but Gorilla can't currently confirm its status.",
            warnings
        );
        Assert.Empty(viewModel.WarningBanner);
        Assert.Equal([false], client.RefreshArguments);
    }

    [Fact]
    public async Task InstallAsync_StatusEvidenceForDifferentOperationDoesNotClearUnrelatedWarning()
    {
        var client = new FakeClient
        {
            StreamAsync = (_, _) => CompletedStream(),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        var viewModel = new HomeViewModel(client, coordinator, new OperationTracker(client));
        var item = MakeUiItem();
        var unrelatedFailure = new IOException("other operation status unavailable");

        viewModel.ReportInfrastructureWarning(
            "Gorilla can't currently confirm another operation's status.",
            "Operation status stream lost; operation remains known and active",
            unrelatedFailure,
            operationId: "op-2",
            itemName: "OtherApp",
            expectedAction: AppCatalog.Action.Install.ToString()
        );

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Equal(
            "Gorilla can't currently confirm another operation's status.",
            viewModel.WarningBanner
        );
        Assert.Equal("op-2", viewModel.InfrastructureWarning.OperationId);
        Assert.Equal([false], client.RefreshArguments);
    }

    [Fact]
    public async Task PostOperationRefreshFailure_UsesCatalogStateAndManualRecoveryClearsIt()
    {
        var listAttempt = 0;
        var client = new FakeClient
        {
            ListAsync = (refresh, _) => ++listAttempt switch
            {
                1 => Task.FromResult(SnapshotTestData.Idle([CatalogItem()], Now)),
                2 => Task.FromException<OptionalInstallsSnapshotResult>(new IOException("catalog refresh failed")),
                _ => Task.FromResult(SnapshotTestData.Idle([CatalogItem()], Now.AddMinutes(2))),
            },
            StreamAsync = (_, _) => CompletedStream(),
        };
        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        var viewModel = new HomeViewModel(client, coordinator, new OperationTracker(client));

        await viewModel.InitializeAsync(CancellationToken.None);
        var item = Assert.Single(viewModel.Items);

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.Equal(string.Empty, viewModel.WarningBanner);
        Assert.True(viewModel.CatalogState.HasRefreshFailure);
        Assert.Contains("catalog refresh failed", viewModel.CatalogState.RefreshFailure!.Message);

        await viewModel.RefreshCatalogAsync(CancellationToken.None);

        Assert.Equal(string.Empty, viewModel.WarningBanner);
        Assert.False(viewModel.CatalogState.HasRefreshFailure);
        Assert.True(viewModel.CatalogState.IsLive);
        Assert.Equal([true, false, true], client.RefreshArguments);
    }

    [Fact]
    public async Task ManualRefresh_AfterStreamAndLookupFailure_ClearsVanishedOperationWithoutInventingResult()
    {
        var client = new FakeClient();
        var tracker = new OperationTracker(client);
        var viewModel = new HomeViewModel(client,
            new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore()), tracker);
        await viewModel.InitializeAsync(CancellationToken.None);
        var item = Assert.Single(viewModel.Items);
        client.FailRetainedLookup = true;
        client.StreamAsync = (_, _) => ActiveThenDroppedStream();

        await viewModel.InstallAsync(item, CancellationToken.None);

        Assert.NotNull(item.ActiveOperation);
        Assert.Equal("Operation tracking-loss reconciliation failed", viewModel.InfrastructureWarning.Context);
        await viewModel.RefreshCatalogAsync(CancellationToken.None);
        item = Assert.Single(viewModel.Items);
        Assert.True(item.IsBusy);

        client.FailRetainedLookup = false;
        await viewModel.RefreshCatalogAsync(CancellationToken.None);

        item = Assert.Single(viewModel.Items);
        Assert.False(item.IsBusy);
        Assert.True(item.CanInstall);
        Assert.Null(item.ActiveOperation);
        Assert.Null(item.LatestOperation);
        Assert.Empty(viewModel.ActivityItems);
        Assert.Null(tracker.GetActiveForItem("VLC"));
        Assert.Null(tracker.GetLatestTerminalForItem("VLC"));
        Assert.Equal(2, client.StreamCalls);
    }

    [Fact]
    public async Task ManualRefresh_RepeatedLookupFailureThenMissingOperation_RetainsLossExplanation()
    {
        var client = new FakeClient();
        var tracker = new OperationTracker(client);
        var viewModel = new HomeViewModel(client,
            new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore()), tracker);
        await viewModel.InitializeAsync(CancellationToken.None);
        client.FailRetainedLookup = true;
        client.StreamAsync = (_, _) => ActiveThenDroppedStream();

        await viewModel.InstallAsync(Assert.Single(viewModel.Items), CancellationToken.None);
        Assert.Equal("Operation tracking-loss reconciliation failed", viewModel.InfrastructureWarning.Context);
        Assert.Equal("op-1", viewModel.InfrastructureWarning.OperationId);

        await viewModel.RefreshCatalogAsync(CancellationToken.None);
        Assert.Equal("Operation tracking-loss reconciliation failed", viewModel.InfrastructureWarning.Context);
        Assert.Equal("op-1", viewModel.InfrastructureWarning.OperationId);
        Assert.True(Assert.Single(viewModel.Items).IsBusy);

        client.FailRetainedLookup = false;
        await viewModel.RefreshCatalogAsync(CancellationToken.None);

        Assert.Equal("Operation disappeared during tracking-loss reconciliation",
            viewModel.InfrastructureWarning.Context);
        Assert.Equal("op-1", viewModel.InfrastructureWarning.OperationId);
        Assert.Contains("no longer available", viewModel.WarningBanner);
        Assert.False(Assert.Single(viewModel.Items).IsBusy);
        Assert.Empty(viewModel.ActivityItems);
    }

    [Fact]
    public async Task ManualRefresh_AfterFailedReconciliation_ResumesRetainedOperationOnce()
    {
        using var cancellation = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        var client = new FakeClient();
        var tracker = new OperationTracker(client);
        var viewModel = new HomeViewModel(client,
            new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore()), tracker);
        await viewModel.InitializeAsync(cancellation.Token);
        client.FailRetainedLookup = true;
        client.StreamAsync = (_, _) => ActiveThenDroppedStream();
        await viewModel.InstallAsync(Assert.Single(viewModel.Items), cancellation.Token);

        client.FailRetainedLookup = false;
        client.Operations = [ActiveOperation()];
        var started = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        client.StreamAsync = (_, token) => ActiveStream(started, token);
        await viewModel.RefreshCatalogAsync(cancellation.Token);
        await started.Task.WaitAsync(cancellation.Token);
        await viewModel.RefreshCatalogAsync(cancellation.Token);

        Assert.True(Assert.Single(viewModel.Items).IsBusy);
        Assert.Equal(3, client.StreamCalls);
        Assert.Empty(viewModel.WarningBanner);
        cancellation.Cancel();
    }

    private static async IAsyncEnumerable<OperationStatusEvent> ActiveThenDroppedStream()
    {
        yield return ActiveOperation();
        await Task.Yield();
        throw new IOException("service stopped");
    }

    private static UiOptionalInstallItem MakeUiItem() => new()
    {
        ItemName = "VLC",
        DisplayName = "VLC",
        Version = "1.0.0",
        Status = "NotInstalled",
        IsInstalled = false,
        InstallAllowed = true,
        RemoveAllowed = false,
    };

    private static OptionalInstallItem CatalogItem() => new(
        "VLC",
        "VLC",
        "1.0.0",
        "testcatalog",
        "ps1",
        "VLC",
        "VLC.ps1",
        true,
        false,
        OptionalInstallStatus.NotInstalled,
        Now,
        null,
        Actions: new Actions(
            new ActionDecision(true, "allowed"),
            new ActionDecision(false, "not_installed")
        )
    );

    private static OperationStatusEvent ActiveOperation() => new(
        "op-1",
        OperationState.Installing,
        null,
        "Installing item via managed run",
        Now,
        "VLC",
        AppCatalog.Action.Install
    );

    private static async IAsyncEnumerable<OperationStatusEvent> ActiveStream(
        TaskCompletionSource<bool> started,
        [EnumeratorCancellation] CancellationToken cancellationToken
    )
    {
        started.TrySetResult(true);
        yield return ActiveOperation();
        await Task.Delay(Timeout.InfiniteTimeSpan, cancellationToken);
    }

    private static async IAsyncEnumerable<OperationStatusEvent> CompletedStream()
    {
        await Task.Yield();
        yield return new OperationStatusEvent(
            "op-1",
            OperationState.Completed,
            null,
            "Installed",
            Now.AddSeconds(1),
            "VLC",
            AppCatalog.Action.Install,
            new Result(Outcome.Succeeded, "completed", Message: "Installed")
        );
    }

    private static async IAsyncEnumerable<OperationStatusEvent> ThrowingStream(Exception exception)
    {
        await Task.Yield();
        throw exception;
#pragma warning disable CS0162
        yield break;
#pragma warning restore CS0162
    }

    private sealed class InMemoryCacheStore : IOptionalInstallsCacheStore
    {
        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult<OptionalInstallsCacheDocument?>(null);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
            => Task.CompletedTask;
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public IReadOnlyList<OperationStatusEvent> Operations { get; set; } = [];
        public bool FailRetainedLookup { get; set; }
        public Func<string, CancellationToken, IAsyncEnumerable<OperationStatusEvent>> StreamAsync { get; set; }
            = (_, _) => EmptyStream();
        public Func<bool, CancellationToken, Task<OptionalInstallsSnapshotResult>> ListAsync { get; init; }
            = (_, _) => Task.FromResult(SnapshotTestData.Idle([CatalogItem()], Now));

        public int StreamCalls { get; private set; }
        public int ListOperationsCalls { get; private set; }
        public List<bool> RefreshArguments { get; } = [];

        public Task<OptionalInstallsSnapshotResult> ListOptionalInstallsAsync(
            bool refresh,
            CancellationToken cancellationToken
        )
        {
            RefreshArguments.Add(refresh);
            return ListAsync(refresh, cancellationToken);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => Task.FromResult(new OperationAccepted("op-1", true, Now));

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
        {
            ListOperationsCalls++;
            if (FailRetainedLookup)
            {
                throw new IOException("service unavailable");
            }
            return Task.FromResult(Operations);
        }

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            CancellationToken cancellationToken
        )
        {
            StreamCalls++;
            return StreamAsync(operationId, cancellationToken);
        }

        private static async IAsyncEnumerable<OperationStatusEvent> EmptyStream()
        {
            await Task.CompletedTask;
            yield break;
        }
    }
}
