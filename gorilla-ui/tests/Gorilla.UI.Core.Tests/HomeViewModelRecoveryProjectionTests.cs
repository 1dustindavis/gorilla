using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using CatalogAction = Gorilla.UI.Client.AppCatalog.Action;

namespace Gorilla.UI.Core.Tests;

public sealed class HomeViewModelRecoveryProjectionTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-13T17:42:31.1234567Z");

    [Fact]
    public async Task InitializeAsync_ProjectsRetainedFailureRecoveryWithoutCallerRepair()
    {
        var client = new FakeClient();
        var viewModel = CreateViewModel(client);

        await viewModel.InitializeAsync(CancellationToken.None);

        var activity = Assert.Single(viewModel.ActivityItems);
        Assert.Equal("retained-failure", activity.OperationId);
        Assert.True(activity.CanRetry);
        Assert.True(activity.CanNavigate);
        Assert.Null(activity.RetryUnavailableReason);
    }

    [Fact]
    public async Task RetryAsync_RejectedAdmissionSurfacesFeedbackOnActivityWithoutNewOperation()
    {
        var client = new FakeClient
        {
            InstallAccepted = new OperationAccepted("not-created", false, Now.AddMinutes(1)),
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        var result = await viewModel.RetryAsync("retained-failure", CancellationToken.None);

        Assert.False(result.Started);
        Assert.Equal("Install was not accepted for VLC.", result.Feedback);
        Assert.Equal(1, client.InstallCalls);
        var activity = Assert.Single(viewModel.ActivityItems);
        Assert.Equal("retained-failure", activity.OperationId);
        Assert.Equal(Outcome.Failed, activity.Result?.Outcome);
        Assert.True(activity.HasRetryAttemptFeedback);
        Assert.Equal("Install was not accepted for VLC.", activity.RetryAttemptFeedback);
    }

    [Fact]
    public async Task RetryAsync_DoesNotUsePresentationFallbackWhenCanonicalItemIsAbsent()
    {
        var client = new FakeClient();
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        client.Catalog = [];
        await viewModel.RefreshCatalogAsync(CancellationToken.None);
        viewModel.Items.Add(new UiOptionalInstallItem
        {
            ItemName = "VLC",
            DisplayName = "Stale VLC presentation",
            Observation = new Observation(ObservedState.Absent, null, Now, string.Empty, RequirementState.NotSatisfied),
            Policy = new Policy(true, false, false, false, Selection.None),
            InstallDecision = new ActionDecision(true, "allowed"),
            RemoveDecision = new ActionDecision(false, "remove_unavailable"),
        });

        var result = await viewModel.RetryAsync("retained-failure", CancellationToken.None);

        Assert.False(result.Started);
        Assert.Contains("no longer available", result.Feedback, StringComparison.OrdinalIgnoreCase);
        Assert.Equal(0, client.InstallCalls);
        var activity = Assert.Single(viewModel.ActivityItems);
        Assert.Equal("retained-failure", activity.OperationId);
        Assert.Contains("no longer available", activity.RetryAttemptFeedback, StringComparison.OrdinalIgnoreCase);
    }

    [Fact]
    public void RecoveryTechnicalDetails_PreserveExactTimestamp()
    {
        var operation = new UiOperationPresentation(
            "op-exact-time",
            CatalogAction.Install,
            OperationState.Completed,
            null,
            new Result(Outcome.Failed, "execution_failed", "installer_exit", "Installer failed."),
            "operation message",
            Now
        );
        var item = new UiOptionalInstallItem
        {
            ItemName = "VLC",
            DisplayName = "VLC",
            Observation = new Observation(ObservedState.Absent, null, Now, string.Empty, RequirementState.NotSatisfied),
            Policy = new Policy(true, false, false, false, Selection.None),
            InstallDecision = new ActionDecision(true, "allowed"),
            RemoveDecision = new ActionDecision(false, "remove_unavailable"),
        };

        var recovery = OperationRecoveryPresentationMapper.Map(operation, item, false);

        Assert.Contains($"Timestamp: {Now:O}", recovery.TechnicalDetails, StringComparison.Ordinal);
    }

    private static HomeViewModel CreateViewModel(FakeClient client)
        => new(
            client,
            new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore()),
            new OperationTracker(client)
        );

    private sealed class InMemoryCacheStore : IOptionalInstallsCacheStore
    {
        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult<OptionalInstallsCacheDocument?>(null);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
            => Task.CompletedTask;
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        public IReadOnlyList<OptionalInstallItem> Catalog { get; set; } = [ProtocolItem()];
        public OperationAccepted InstallAccepted { get; set; } = new("created", true, Now.AddMinutes(1));
        public int InstallCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult(Catalog);

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
            => Task.FromResult<IReadOnlyList<OperationStatusEvent>>([
                new OperationStatusEvent(
                    "retained-failure",
                    OperationState.Completed,
                    null,
                    "failed",
                    Now,
                    "VLC",
                    CatalogAction.Install,
                    new Result(Outcome.Failed, "execution_failed", "installer_exit", "Installer failed.")
                ),
            ]);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
        {
            InstallCalls++;
            return Task.FromResult(InstallAccepted);
        }

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public async IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            [System.Runtime.CompilerServices.EnumeratorCancellation] CancellationToken cancellationToken)
        {
            await Task.CompletedTask;
            yield break;
        }
    }

    private static OptionalInstallItem ProtocolItem() => new(
        "VLC",
        "VLC",
        "4.0",
        "testcatalog",
        "nupkg",
        "VLC",
        "packages/VLC/VLC.nupkg",
        true,
        false,
        OptionalInstallStatus.NotInstalled,
        Now,
        null,
        TargetVersion: "4.0",
        Observation: new Observation(ObservedState.Absent, null, Now, string.Empty, RequirementState.NotSatisfied),
        Policy: new Policy(true, false, false, false, Selection.None),
        Actions: new Actions(
            new ActionDecision(true, "allowed"),
            new ActionDecision(false, "remove_unavailable")
        ),
        Description: "VLC media player"
    );
}
