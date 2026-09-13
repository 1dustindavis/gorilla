using System.Runtime.CompilerServices;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Tests;

public class HomeViewModelRetryTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-13T17:42:00Z");

    [Theory]
    [InlineData(Outcome.Failed)]
    [InlineData(Outcome.Unverified)]
    [InlineData(Outcome.Interrupted)]
    public void RecoveryPresentation_RetryCandidateRequiresCurrentAllowedAction(Outcome outcome)
    {
        var item = UiItem(installAllowed: true, removeAllowed: true);
        var install = Presentation("old-install", CatalogAction.Install, outcome);
        var remove = Presentation("old-remove", CatalogAction.Remove, outcome);

        Assert.True(OperationRecoveryPresentationMapper.Map(install, item, false).CanRetry);
        Assert.True(OperationRecoveryPresentationMapper.Map(remove, item, false).CanRetry);
    }

    [Theory]
    [InlineData(Outcome.Succeeded)]
    [InlineData(Outcome.AlreadySatisfied)]
    public void RecoveryPresentation_SuccessfulOutcomeDoesNotRetry(Outcome outcome)
    {
        var recovery = OperationRecoveryPresentationMapper.Map(
            Presentation("old", CatalogAction.Install, outcome),
            UiItem(installAllowed: true),
            false
        );

        Assert.False(recovery.IsRetryCandidate);
        Assert.False(recovery.CanRetry);
    }

    [Fact]
    public void RecoveryPresentation_ActiveOperationDoesNotRetry()
    {
        var operation = Presentation("active", CatalogAction.Install, null, OperationState.Installing);
        var recovery = OperationRecoveryPresentationMapper.Map(operation, UiItem(installAllowed: true), false);

        Assert.False(recovery.IsRetryCandidate);
        Assert.False(recovery.CanRetry);
    }

    [Fact]
    public void RecoveryPresentation_MissingItemDoesNotRetry()
    {
        var recovery = OperationRecoveryPresentationMapper.Map(
            Presentation("old", CatalogAction.Install, Outcome.Failed),
            currentItem: null,
            hasConflictingActiveOperation: false
        );

        Assert.False(recovery.CanRetry);
        Assert.Contains("no longer available", recovery.RetryUnavailableReason);
    }

    [Fact]
    public void RecoveryPresentation_DisallowedCurrentActionUsesCurrentReason()
    {
        var item = UiItem(installAllowed: false, installReason: "required_install");
        var recovery = OperationRecoveryPresentationMapper.Map(
            Presentation("old", CatalogAction.Install, Outcome.Failed),
            item,
            false
        );

        Assert.False(recovery.CanRetry);
        Assert.Contains("required to stay installed", recovery.RetryUnavailableReason);
    }

    [Fact]
    public void RecoveryPresentation_ConflictingActiveOperationDoesNotRetry()
    {
        var recovery = OperationRecoveryPresentationMapper.Map(
            Presentation("old", CatalogAction.Install, Outcome.Failed),
            UiItem(installAllowed: true),
            hasConflictingActiveOperation: true
        );

        Assert.False(recovery.CanRetry);
        Assert.Contains("already active", recovery.RetryUnavailableReason);
    }

    [Fact]
    public void RecoveryPresentation_PreservesHistoricalInstallWhenCurrentPresentationWouldBeUpdate()
    {
        var item = UiItem(installAllowed: true);
        item.Observation = item.Observation with { State = ObservedState.UpdateAvailable };
        var historical = Presentation("old", CatalogAction.Install, Outcome.Failed);

        var recovery = OperationRecoveryPresentationMapper.Map(historical, item, false);

        Assert.Equal(CatalogAction.Install, historical.Action);
        Assert.True(recovery.CanRetry);
        Assert.Equal("Installation failed", recovery.OutcomeTitle);
        Assert.Equal("Update", item.CardPresentation.PrimaryAction?.Label);
    }

    [Theory]
    [InlineData(Outcome.Failed, "Installation failed")]
    [InlineData(Outcome.Unverified, "Installation couldn't be verified")]
    [InlineData(Outcome.Interrupted, "Installation was interrupted")]
    [InlineData(Outcome.Succeeded, "Installation succeeded")]
    [InlineData(Outcome.AlreadySatisfied, "Installation was already satisfied")]
    public void RecoveryPresentation_UsesDistinctOutcomeLanguage(Outcome outcome, string expected)
    {
        var recovery = OperationRecoveryPresentationMapper.Map(
            Presentation("old", CatalogAction.Install, outcome),
            UiItem(installAllowed: true),
            false
        );

        Assert.Equal(expected, recovery.OutcomeTitle);
    }

    [Fact]
    public void RecoveryPresentation_PreservesStructuredTechnicalDetail()
    {
        var operation = new UiOperationPresentation(
            "op-123",
            CatalogAction.Install,
            OperationState.Completed,
            null,
            new Result(Outcome.Failed, "execution_failed", "installer_exit", "Installer exited with status 7."),
            "phase detail",
            Now
        );

        var recovery = OperationRecoveryPresentationMapper.Map(operation, UiItem(installAllowed: true), false);

        Assert.Equal("Installer exited with status 7.", recovery.UserMessage);
        Assert.Contains("Operation ID: op-123", recovery.TechnicalDetails);
        Assert.Contains("Outcome: Failed", recovery.TechnicalDetails);
        Assert.Contains("Code: execution_failed", recovery.TechnicalDetails);
        Assert.Contains("Detail code: installer_exit", recovery.TechnicalDetails);
        Assert.Contains("Result message: Installer exited with status 7.", recovery.TechnicalDetails);
        Assert.Contains("Operation message: phase detail", recovery.TechnicalDetails);
    }

    [Fact]
    public async Task RetryAsync_FailedInstallUsesCurrentInstallPathAndCreatesSeparateOperation()
    {
        var client = new FakeClient
        {
            Catalog = [ProtocolItem(installAllowed: true)],
            Operations = [Historical("old-op", CatalogAction.Install, Outcome.Failed)],
            InstallAccepted = new OperationAccepted("new-op", true, Now.AddMinutes(5)),
            StreamFactory = (id, token) => CompletedStream(id, CatalogAction.Install, Outcome.Succeeded, token),
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        await viewModel.RetryAsync("old-op", CancellationToken.None);

        Assert.Equal(1, client.InstallCalls);
        Assert.Equal(0, client.RemoveCalls);
        Assert.Equal("VLC", client.LastInstallItem);
        Assert.Contains(viewModel.ActivityItems, item => item.OperationId == "old-op" && item.Result?.Outcome == Outcome.Failed);
        Assert.Contains(viewModel.ActivityItems, item => item.OperationId == "new-op" && item.Result?.Outcome == Outcome.Succeeded);
        Assert.NotEqual("old-op", "new-op");
    }

    [Fact]
    public async Task RetryAsync_FailedRemoveUsesCurrentRemovePath()
    {
        var client = new FakeClient
        {
            Catalog = [ProtocolItem(installAllowed: false, removeAllowed: true, installed: true)],
            Operations = [Historical("old-remove", CatalogAction.Remove, Outcome.Failed)],
            RemoveAccepted = new OperationAccepted("new-remove", true, Now.AddMinutes(5)),
            StreamFactory = (id, token) => CompletedStream(id, CatalogAction.Remove, Outcome.Succeeded, token),
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        await viewModel.RetryAsync("old-remove", CancellationToken.None);

        Assert.Equal(0, client.InstallCalls);
        Assert.Equal(1, client.RemoveCalls);
        Assert.Contains(viewModel.ActivityItems, item => item.OperationId == "old-remove");
        Assert.Contains(viewModel.ActivityItems, item => item.OperationId == "new-remove");
    }

    [Fact]
    public async Task RetryAsync_RapidSecondInvocationDoesNotSubmitDuplicateMutation()
    {
        var admissionGate = new TaskCompletionSource<OperationAccepted>(TaskCreationOptions.RunContinuationsAsynchronously);
        var client = new FakeClient
        {
            Catalog = [ProtocolItem(installAllowed: true)],
            Operations = [Historical("old-op", CatalogAction.Install, Outcome.Failed)],
            InstallGate = admissionGate,
            StreamFactory = (id, token) => CompletedStream(id, CatalogAction.Install, Outcome.Succeeded, token),
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        var firstRetry = viewModel.RetryAsync("old-op", CancellationToken.None);
        await WaitUntilAsync(() => client.InstallCalls == 1);

        await viewModel.RetryAsync("old-op", CancellationToken.None);
        Assert.Equal(1, client.InstallCalls);
        Assert.Contains("already active", viewModel.FindItem("VLC")?.TransientFeedback, StringComparison.OrdinalIgnoreCase);

        admissionGate.SetResult(new OperationAccepted("new-op", true, Now.AddMinutes(5)));
        await firstRetry;
        Assert.Contains(viewModel.ActivityItems, item => item.OperationId == "new-op");
    }

    [Fact]
    public async Task RetryAsync_DisallowedBeforeAdmissionCreatesNoNewOperation()
    {
        var client = new FakeClient
        {
            Catalog = [ProtocolItem(installAllowed: false, installReason: "already_selected", installed: true)],
            Operations = [Historical("old-op", CatalogAction.Install, Outcome.Failed)],
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        await viewModel.RetryAsync("old-op", CancellationToken.None);

        Assert.Equal(0, client.InstallCalls);
        Assert.Single(viewModel.ActivityItems);
        Assert.Equal("old-op", viewModel.ActivityItems[0].OperationId);
        Assert.Equal(Outcome.Failed, viewModel.ActivityItems[0].Result?.Outcome);
    }

    [Fact]
    public async Task RetryAsync_RejectedAdmissionDoesNotCreateFakeActivity()
    {
        var client = new FakeClient
        {
            Catalog = [ProtocolItem(installAllowed: true)],
            Operations = [Historical("old-op", CatalogAction.Install, Outcome.Failed)],
            InstallAccepted = new OperationAccepted("not-created", false, Now.AddMinutes(5)),
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        await viewModel.RetryAsync("old-op", CancellationToken.None);

        Assert.Equal(1, client.InstallCalls);
        Assert.Single(viewModel.ActivityItems);
        Assert.Equal("old-op", viewModel.ActivityItems[0].OperationId);
        Assert.Equal("Install was not accepted for VLC.", viewModel.FindItem("VLC")?.TransientFeedback);
    }

    [Fact]
    public async Task RefreshCatalogAsync_DynamicallyRemovesRetryWithoutChangingHistory()
    {
        var client = new FakeClient
        {
            Catalog = [ProtocolItem(installAllowed: true)],
            Operations = [Historical("old-op", CatalogAction.Install, Outcome.Failed)],
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);
        viewModel.RefreshActivityRecoveryPresentations();
        Assert.True(viewModel.ActivityItems.Single().CanRetry);

        client.Catalog = [ProtocolItem(installAllowed: false, installReason: "already_selected", installed: true)];
        await viewModel.RefreshCatalogAsync(CancellationToken.None);

        var activity = viewModel.ActivityItems.Single();
        Assert.False(activity.CanRetry);
        Assert.Equal(CatalogAction.Install, activity.Action);
        Assert.Equal(Outcome.Failed, activity.Result?.Outcome);
    }

    [Fact]
    public async Task RefreshCatalogAsync_ItemDisappearanceAndReappearanceRecomputeRetry()
    {
        var client = new FakeClient
        {
            Catalog = [ProtocolItem(installAllowed: true)],
            Operations = [Historical("old-op", CatalogAction.Install, Outcome.Failed)],
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);
        viewModel.RefreshActivityRecoveryPresentations();
        Assert.True(viewModel.ActivityItems.Single().CanRetry);
        Assert.True(viewModel.ActivityItems.Single().CanNavigate);

        client.Catalog = [];
        await viewModel.RefreshCatalogAsync(CancellationToken.None);
        Assert.False(viewModel.ActivityItems.Single().CanRetry);
        Assert.False(viewModel.ActivityItems.Single().CanNavigate);
        Assert.Contains("no longer available", viewModel.ActivityItems.Single().RetryUnavailableReason);

        client.Catalog = [ProtocolItem(installAllowed: true)];
        await viewModel.RefreshCatalogAsync(CancellationToken.None);
        Assert.True(viewModel.ActivityItems.Single().CanRetry);
        Assert.True(viewModel.ActivityItems.Single().CanNavigate);
        Assert.Equal(Outcome.Failed, viewModel.ActivityItems.Single().Result?.Outcome);
    }

    [Fact]
    public async Task RecoveredFailureAfterInitializationCanRetryFromRetainedOperation()
    {
        var client = new FakeClient
        {
            Catalog = [ProtocolItem(installAllowed: true)],
            Operations = [Historical("retained-failure", CatalogAction.Install, Outcome.Failed)],
        };
        var viewModel = CreateViewModel(client);

        await viewModel.InitializeAsync(CancellationToken.None);
        viewModel.RefreshActivityRecoveryPresentations();

        var activity = Assert.Single(viewModel.ActivityItems);
        Assert.Equal("retained-failure", activity.OperationId);
        Assert.True(activity.CanRetry);
    }

    private static async Task WaitUntilAsync(Func<bool> condition)
    {
        for (var attempt = 0; attempt < 100; attempt++)
        {
            if (condition())
            {
                return;
            }
            await Task.Delay(10);
        }
        throw new TimeoutException("Timed out waiting for test condition.");
    }

    private static HomeViewModel CreateViewModel(FakeClient client)
        => new(client, new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore()), new OperationTracker(client));

    private static UiOptionalInstallItem UiItem(
        bool installAllowed = false,
        bool removeAllowed = false,
        string installReason = "install_unavailable",
        string removeReason = "remove_unavailable") => new()
    {
        ItemName = "VLC",
        DisplayName = "VLC",
        TargetVersion = "4.0",
        Observation = new Observation(ObservedState.Absent, null, Now, string.Empty, RequirementState.NotSatisfied),
        Policy = new Policy(true, false, false, false, Selection.None),
        InstallDecision = new ActionDecision(installAllowed, installReason),
        RemoveDecision = new ActionDecision(removeAllowed, removeReason),
    };

    private static UiOperationPresentation Presentation(
        string id,
        CatalogAction action,
        Outcome? outcome,
        OperationState state = OperationState.Completed)
        => new(
            id,
            action,
            state,
            null,
            outcome is null ? null : new Result(outcome.Value, "test_code", "test_detail", "service message"),
            "operation message",
            Now
        );

    private static OperationStatusEvent Historical(string id, CatalogAction action, Outcome outcome)
        => new(
            id,
            OperationState.Completed,
            null,
            "historical operation message",
            Now,
            "VLC",
            action,
            new Result(outcome, "execution_failed", "installer_exit", "historical result message")
        );

    private static OptionalInstallItem ProtocolItem(
        bool installAllowed,
        bool removeAllowed = false,
        string installReason = "install_unavailable",
        string removeReason = "remove_unavailable",
        bool installed = false)
        => new(
            "VLC",
            "VLC",
            "4.0",
            "testcatalog",
            "nupkg",
            "VLC",
            "packages/VLC/VLC.nupkg",
            true,
            installed,
            installed ? OptionalInstallStatus.Installed : OptionalInstallStatus.NotInstalled,
            Now,
            null,
            TargetVersion: "4.0",
            Observation: new Observation(
                installed ? ObservedState.Installed : ObservedState.Absent,
                installed ? "4.0" : null,
                Now,
                string.Empty,
                installed ? RequirementState.Satisfied : RequirementState.NotSatisfied
            ),
            Policy: new Policy(true, false, false, false, installed ? Selection.Install : Selection.None),
            Actions: new Actions(
                new ActionDecision(installAllowed, installReason),
                new ActionDecision(removeAllowed, removeReason)
            ),
            Description: "VLC media player"
        );

    private static async IAsyncEnumerable<OperationStatusEvent> CompletedStream(
        string operationId,
        CatalogAction action,
        Outcome outcome,
        [EnumeratorCancellation] CancellationToken cancellationToken = default)
    {
        await Task.Yield();
        cancellationToken.ThrowIfCancellationRequested();
        yield return new OperationStatusEvent(
            operationId,
            OperationState.Completed,
            null,
            outcome == Outcome.Succeeded ? "Completed" : "Failed again",
            Now.AddMinutes(5),
            "VLC",
            action,
            new Result(outcome, outcome == Outcome.Succeeded ? "completed" : "execution_failed", Message: "result")
        );
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
        public IReadOnlyList<OptionalInstallItem> Catalog { get; set; } = [];
        public IReadOnlyList<OperationStatusEvent> Operations { get; set; } = [];
        public OperationAccepted InstallAccepted { get; set; } = new("new-install", true, Now.AddMinutes(5));
        public OperationAccepted RemoveAccepted { get; set; } = new("new-remove", true, Now.AddMinutes(5));
        public TaskCompletionSource<OperationAccepted>? InstallGate { get; set; }
        public Func<string, CancellationToken, IAsyncEnumerable<OperationStatusEvent>> StreamFactory { get; set; }
            = (id, token) => CompletedStream(id, CatalogAction.Install, Outcome.Succeeded, token);
        public int InstallCalls { get; private set; }
        public int RemoveCalls { get; private set; }
        public string? LastInstallItem { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult(Catalog);

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
            => Task.FromResult(Operations);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
        {
            InstallCalls++;
            LastInstallItem = itemName;
            return InstallGate?.Task ?? Task.FromResult(InstallAccepted);
        }

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
        {
            RemoveCalls++;
            return Task.FromResult(RemoveAccepted);
        }

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(string operationId, CancellationToken cancellationToken)
            => StreamFactory(operationId, cancellationToken);
    }
}
