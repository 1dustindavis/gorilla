using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using AppCatalog = Gorilla.UI.Client.AppCatalog;

namespace Gorilla.UI.Core.Tests;

public class ActivityProjectionTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-12T03:00:00Z");

    [Fact]
    public async Task Activity_UsesOperationIdentityDeduplicatesAndOrdersActiveBeforeTerminal()
    {
        var activeOlder = Event("op-b", "Beta", OperationState.Installing, Now.AddMinutes(-2));
        var activeNewer = Event("op-a", "Alpha", OperationState.Removing, Now.AddMinutes(-1), action: AppCatalog.Action.Remove);
        var terminal = Completed("op-z", "Alpha", Outcome.Succeeded, Now);
        var client = new FakeClient
        {
            Catalogs = [[Item("Alpha", "Alpha App"), Item("Beta", "Beta App")]],
            OperationSnapshots = [[terminal, activeOlder, activeNewer]],
        };
        var tracker = new OperationTracker(client);
        var viewModel = CreateViewModel(client, tracker);

        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.True(viewModel.IsActivityLoaded);
        Assert.Equal(["op-a", "op-b", "op-z"], viewModel.ActivityItems.Select(item => item.OperationId));
        Assert.Equal("Alpha App", viewModel.ActivityItems[0].DisplayName);
        Assert.Equal("Remove", viewModel.ActivityItems[0].ActionLabel);
        Assert.Equal("Install", viewModel.ActivityItems[1].ActionLabel);
        Assert.Equal(3, viewModel.ActivityItems.Select(item => item.OperationId).Distinct().Count());
    }

    [Fact]
    public async Task Activity_ActiveBecomingTerminalReconcilesSameLogicalEntry()
    {
        var active = Event("op-1", "Example", OperationState.Installing, Now, progress: 25);
        var terminal = Completed("op-1", "Example", Outcome.Failed, Now.AddMinutes(1), "execution_failed", "Installer failed");
        var client = new FakeClient
        {
            Catalogs = [[Item("Example", "Example App")]],
            OperationSnapshots = [[active], [terminal]],
        };
        var tracker = new OperationTracker(client);
        var viewModel = CreateViewModel(client, tracker);
        await viewModel.InitializeAsync(CancellationToken.None);
        var before = Assert.Single(viewModel.ActivityItems);
        Assert.True(before.IsActive);

        await tracker.RefreshKnownOperationsAsync(CancellationToken.None);

        var after = Assert.Single(viewModel.ActivityItems);
        Assert.Same(before, after);
        Assert.True(after.IsTerminal);
        Assert.Equal(Outcome.Failed, after.Result?.Outcome);
        Assert.Equal("Installer failed", after.DetailText);
        Assert.Equal("op-1", Assert.Single(viewModel.Items).LatestOperation?.OperationId);
    }

    [Theory]
    [InlineData(Outcome.Succeeded)]
    [InlineData(Outcome.AlreadySatisfied)]
    [InlineData(Outcome.Failed)]
    [InlineData(Outcome.Unverified)]
    [InlineData(Outcome.Interrupted)]
    public async Task Activity_PreservesEveryStructuredTerminalOutcome(Outcome outcome)
    {
        var operation = Completed("op-1", "Example", outcome, Now, $"{outcome}_code", $"{outcome} detail");
        var client = new FakeClient
        {
            Catalogs = [[Item("Example", "Example App")]],
            OperationSnapshots = [[operation]],
        };
        var viewModel = CreateViewModel(client, new OperationTracker(client));

        await viewModel.InitializeAsync(CancellationToken.None);

        var activity = Assert.Single(viewModel.ActivityItems);
        Assert.Equal(outcome, activity.Result?.Outcome);
        Assert.Equal($"{outcome}_code", activity.Result?.Code);
        Assert.Equal($"{outcome} detail", activity.Result?.Message);
        Assert.Equal(outcome.ToString(), activity.StateText);
    }

    [Fact]
    public async Task Activity_MissingCatalogItemRemainsVisibleAndNonNavigable()
    {
        var operation = Completed("op-orphan", "GoneApp", Outcome.Interrupted, Now);
        var client = new FakeClient
        {
            Catalogs = [[]],
            OperationSnapshots = [[operation]],
        };
        var viewModel = CreateViewModel(client, new OperationTracker(client));

        await viewModel.InitializeAsync(CancellationToken.None);

        var activity = Assert.Single(viewModel.ActivityItems);
        Assert.Equal("GoneApp", activity.ItemName);
        Assert.Equal("GoneApp", activity.DisplayName);
        Assert.False(activity.CanNavigate);
    }

    [Fact]
    public async Task Activity_CatalogArrivalUpgradesDisplayNameWithoutChangingEntryIdentity()
    {
        var operation = Completed("op-1", "Example", Outcome.Succeeded, Now);
        var client = new FakeClient
        {
            Catalogs = [[], [Item("Example", "Friendly Example")]],
            OperationSnapshots = [[operation], [operation]],
        };
        var tracker = new OperationTracker(client);
        var viewModel = CreateViewModel(client, tracker);
        await viewModel.InitializeAsync(CancellationToken.None);
        var before = Assert.Single(viewModel.ActivityItems);
        Assert.Equal("Example", before.DisplayName);
        Assert.False(before.CanNavigate);

        await viewModel.InitializeAsync(CancellationToken.None);

        var after = Assert.Single(viewModel.ActivityItems);
        Assert.Same(before, after);
        Assert.Equal("Friendly Example", after.DisplayName);
        Assert.True(after.CanNavigate);
    }

    [Fact]
    public async Task Activity_RecoveryUsesListOperationsWithoutSubmittingMutation()
    {
        var terminal = Completed("op-recovered", "Example", Outcome.Unverified, Now, "verification_unavailable", "Could not verify");
        var client = new FakeClient
        {
            Catalogs = [[Item("Example", "Example App")]],
            OperationSnapshots = [[terminal]],
        };
        var viewModel = CreateViewModel(client, new OperationTracker(client));

        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.Equal("op-recovered", Assert.Single(viewModel.ActivityItems).OperationId);
        Assert.Equal(0, client.InstallCalls);
        Assert.Equal(0, client.RemoveCalls);
    }

    [Fact]
    public async Task Activity_FailedOperationRecoveryDoesNotClaimAuthoritativeEmptyState()
    {
        var client = new FakeClient
        {
            Catalogs = [[]],
            ListOperationsFailure = new IOException("service unavailable"),
        };
        var viewModel = CreateViewModel(client, new OperationTracker(client));

        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.False(viewModel.IsActivityLoaded);
        Assert.Empty(viewModel.ActivityItems);
        Assert.Contains("Operation status is temporarily unavailable", viewModel.WarningBanner, StringComparison.OrdinalIgnoreCase);
    }

    [Fact]
    public async Task Activity_PreservesStructuredIdentityNeededForFutureCurrentPolicyRetry()
    {
        var result = new Result(Outcome.Failed, "execution_failed", Message: "Installer failed");
        var operation = new OperationStatusEvent(
            "op-history", OperationState.Completed, null, "Done", Now, "Example", AppCatalog.Action.Install, result
        );
        var client = new FakeClient
        {
            Catalogs = [[Item("Example", "Example App")]],
            OperationSnapshots = [[operation]],
        };
        var viewModel = CreateViewModel(client, new OperationTracker(client));
        await viewModel.InitializeAsync(CancellationToken.None);

        var activity = Assert.Single(viewModel.ActivityItems);
        Assert.Equal("op-history", activity.OperationId);
        Assert.Equal("Example", activity.ItemName);
        Assert.Equal(AppCatalog.Action.Install, activity.Action);
        Assert.Same(result, activity.Result);
        Assert.Equal("Install", activity.ActionLabel);
    }

    private static HomeViewModel CreateViewModel(FakeClient client, OperationTracker tracker)
        => new(client, new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore()), tracker);

    private static OperationStatusEvent Event(
        string operationId,
        string itemName,
        OperationState state,
        DateTimeOffset timestamp,
        AppCatalog.Action action = AppCatalog.Action.Install,
        int? progress = null)
        => new(operationId, state, progress, state.ToString(), timestamp, itemName, action);

    private static OperationStatusEvent Completed(
        string operationId,
        string itemName,
        Outcome outcome,
        DateTimeOffset timestamp,
        string code = "completed",
        string message = "Done")
        => new(
            operationId,
            OperationState.Completed,
            null,
            message,
            timestamp,
            itemName,
            AppCatalog.Action.Install,
            new Result(outcome, code, Message: message)
        );

    private static OptionalInstallItem Item(string itemName, string displayName)
        => new(
            itemName, displayName, "1.0", "catalog", "msi", "package", "installer.msi",
            true, false, OptionalInstallStatus.NotInstalled, Now, null, "1.0",
            new Observation(ObservedState.Absent, null, Now, "absent", RequirementState.Unsatisfied),
            Actions: new Actions(new ActionDecision(true, ""), new ActionDecision(false, "not_installed"))
        );

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
        private int _catalogCall;
        private int _operationCall;

        public IReadOnlyList<IReadOnlyList<OptionalInstallItem>> Catalogs { get; init; } = [[]];
        public IReadOnlyList<IReadOnlyList<OperationStatusEvent>> OperationSnapshots { get; init; } = [[]];
        public Exception? ListOperationsFailure { get; init; }
        public int InstallCalls { get; private set; }
        public int RemoveCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            var index = Math.Min(_catalogCall++, Catalogs.Count - 1);
            return Task.FromResult(Catalogs[index]);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
        {
            InstallCalls++;
            return Task.FromResult(new OperationAccepted("new-op", true, Now));
        }

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
        {
            RemoveCalls++;
            return Task.FromResult(new OperationAccepted("new-op", true, Now));
        }

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
        {
            if (ListOperationsFailure is not null)
            {
                throw ListOperationsFailure;
            }

            var index = Math.Min(_operationCall++, OperationSnapshots.Count - 1);
            return Task.FromResult(OperationSnapshots[index]);
        }

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            CancellationToken cancellationToken)
            => EmptyStream(cancellationToken);

        private static async IAsyncEnumerable<OperationStatusEvent> EmptyStream(
            [EnumeratorCancellation] CancellationToken cancellationToken = default)
        {
            await Task.CompletedTask;
            yield break;
        }
    }
}
