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

public class Stage6RecoveryBoundaryTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-14T00:00:00Z");

    [Fact]
    public async Task RetryAsync_ServicePolicyRejectionBlocksRetryUntilSuccessfulRefresh()
    {
        var client = new RejectingClient("already_selected") { Catalog = [ProtocolItem()] };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        var before = Assert.Single(viewModel.ActivityItems);
        Assert.True(before.CanRetry);
        var item = Assert.IsType<UiOptionalInstallItem>(viewModel.FindItem("VLC"));
        Assert.True(item.DetailsPresentation.CanRetryLatest);
        Assert.True(item.InstallDecision.Allowed);

        var result = await viewModel.RetryAsync("old-op", CancellationToken.None);

        Assert.False(result.Started);
        Assert.NotNull(result.Feedback);
        Assert.Contains("already selected", result.Feedback!, StringComparison.OrdinalIgnoreCase);
        Assert.Equal(1, client.InstallCalls);

        var activity = Assert.Single(viewModel.ActivityItems);
        Assert.Equal("old-op", activity.OperationId);
        Assert.Equal(Outcome.Failed, activity.Result?.Outcome);
        Assert.False(activity.CanRetry);
        Assert.NotNull(activity.RetryUnavailableReason);
        Assert.Contains("already selected", activity.RetryUnavailableReason!, StringComparison.OrdinalIgnoreCase);
        Assert.NotNull(activity.RetryAttemptFeedback);
        Assert.Contains("already selected", activity.RetryAttemptFeedback!, StringComparison.OrdinalIgnoreCase);

        Assert.True(item.InstallDecision.Allowed);
        Assert.NotNull(item.InstallRetryBlockedReason);
        Assert.Contains("already selected", item.InstallRetryBlockedReason!, StringComparison.OrdinalIgnoreCase);
        Assert.Null(item.RemoveRetryBlockedReason);
        Assert.NotNull(item.TransientFeedback);
        Assert.False(item.DetailsPresentation.CanRetryLatest);
        Assert.Contains(
            "already selected",
            item.DetailsPresentation.RetryUnavailableReason!,
            StringComparison.OrdinalIgnoreCase
        );

        var secondAttempt = await viewModel.RetryAsync("old-op", CancellationToken.None);
        Assert.False(secondAttempt.Started);
        Assert.Equal(1, client.InstallCalls);
        Assert.Single(viewModel.ActivityItems);

        client.Catalog = [ProtocolItem()];
        await viewModel.RefreshCatalogAsync(CancellationToken.None);

        item = Assert.IsType<UiOptionalInstallItem>(viewModel.FindItem("VLC"));
        activity = Assert.Single(viewModel.ActivityItems);
        Assert.Null(item.InstallRetryBlockedReason);
        Assert.Null(item.RemoveRetryBlockedReason);
        Assert.Null(item.TransientFeedback);
        Assert.Null(activity.RetryAttemptFeedback);
        Assert.True(activity.CanRetry);
        Assert.True(item.DetailsPresentation.CanRetryLatest);
        Assert.True(item.InstallDecision.Allowed);
        Assert.Equal(1, client.InstallCalls);
        Assert.Equal(Outcome.Failed, activity.Result?.Outcome);
    }

    [Fact]
    public async Task RetryAsync_ServiceRejectionBlocksAllRetainedFailuresForSameActionOnly()
    {
        var client = new RejectingClient("already_selected")
        {
            Catalog = [ProtocolItem(removeAllowed: true)],
            Operations =
            [
                Historical("install-old", CatalogAction.Install, Now),
                Historical("install-new", CatalogAction.Install, Now.AddMinutes(1)),
                Historical("remove-old", CatalogAction.Remove, Now.AddMinutes(2)),
            ],
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.True(viewModel.ActivityItems.Single(x => x.OperationId == "install-old").CanRetry);
        Assert.True(viewModel.ActivityItems.Single(x => x.OperationId == "install-new").CanRetry);
        Assert.True(viewModel.ActivityItems.Single(x => x.OperationId == "remove-old").CanRetry);

        await viewModel.RetryAsync("install-old", CancellationToken.None);

        Assert.Equal(1, client.InstallCalls);
        Assert.False(viewModel.ActivityItems.Single(x => x.OperationId == "install-old").CanRetry);
        Assert.False(viewModel.ActivityItems.Single(x => x.OperationId == "install-new").CanRetry);
        Assert.True(viewModel.ActivityItems.Single(x => x.OperationId == "remove-old").CanRetry);

        var item = Assert.IsType<UiOptionalInstallItem>(viewModel.FindItem("VLC"));
        Assert.NotNull(item.InstallRetryBlockedReason);
        Assert.Null(item.RemoveRetryBlockedReason);

        var secondInstall = await viewModel.RetryAsync("install-new", CancellationToken.None);
        Assert.False(secondInstall.Started);
        Assert.Equal(1, client.InstallCalls);

        await viewModel.RefreshCatalogAsync(CancellationToken.None);
        Assert.True(viewModel.ActivityItems.Single(x => x.OperationId == "install-old").CanRetry);
        Assert.True(viewModel.ActivityItems.Single(x => x.OperationId == "install-new").CanRetry);
        Assert.True(viewModel.ActivityItems.Single(x => x.OperationId == "remove-old").CanRetry);
    }

    [Fact]
    public async Task RetryAsync_GenericRejectedAdmissionAlsoBlocksUntilRefresh()
    {
        var client = new FalseAdmissionClient { Catalog = [ProtocolItem()] };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.True(Assert.Single(viewModel.ActivityItems).CanRetry);

        var result = await viewModel.RetryAsync("old-op", CancellationToken.None);

        Assert.False(result.Started);
        Assert.Equal(1, client.InstallCalls);
        var activity = Assert.Single(viewModel.ActivityItems);
        var item = Assert.IsType<UiOptionalInstallItem>(viewModel.FindItem("VLC"));
        Assert.False(activity.CanRetry);
        Assert.False(item.DetailsPresentation.CanRetryLatest);
        Assert.Equal("Install was not accepted for VLC.", item.InstallRetryBlockedReason);
        Assert.True(item.InstallDecision.Allowed);

        await viewModel.RetryAsync("old-op", CancellationToken.None);
        Assert.Equal(1, client.InstallCalls);

        await viewModel.RefreshCatalogAsync(CancellationToken.None);
        Assert.True(Assert.Single(viewModel.ActivityItems).CanRetry);
        Assert.True(Assert.IsType<UiOptionalInstallItem>(viewModel.FindItem("VLC")).DetailsPresentation.CanRetryLatest);
    }

    [Fact]
    public async Task RetryAsync_UnknownServiceErrorStillPropagatesAsInfrastructureFailure()
    {
        var client = new RejectingClient("unexpected_backend_error") { Catalog = [ProtocolItem()] };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        var error = await Assert.ThrowsAsync<ServiceErrorException>(
            () => viewModel.RetryAsync("old-op", CancellationToken.None)
        );

        Assert.Equal("unexpected_backend_error", error.ErrorCode);
        Assert.Single(viewModel.ActivityItems);
        Assert.Equal("old-op", viewModel.ActivityItems[0].OperationId);
    }

    [Fact]
    public void RecoveryPresentation_UnknownSingleLineDetailStaysTechnicalOnly()
    {
        const string diagnostic = "Future backend diagnostic that should not become primary UI";
        var operation = FailedPresentation("op-future", "future_backend_detail", diagnostic);

        var recovery = OperationRecoveryPresentationMapper.Map(operation, UiItem(), false);

        Assert.Null(recovery.UserMessage);
        Assert.Equal("Installation failed", recovery.OutcomeTitle);
        Assert.Contains("Detail code: future_backend_detail", recovery.TechnicalDetails);
        Assert.Contains(diagnostic, recovery.TechnicalDetails);
    }

    [Fact]
    public void RecoveryPresentation_ExceptionLikeKnownMessageStaysTechnicalOnly()
    {
        const string diagnostic = "System.InvalidOperationException: installer bridge failed";
        var operation = FailedPresentation("op-exception", "installer_failed", diagnostic);

        var recovery = OperationRecoveryPresentationMapper.Map(operation, UiItem(), false);

        Assert.Null(recovery.UserMessage);
        Assert.Contains(diagnostic, recovery.TechnicalDetails);
    }

    [Theory]
    [InlineData("future_backend_detail", "Future backend diagnostic that should not become primary UI")]
    [InlineData("installer_failed", "System.InvalidOperationException: installer bridge failed")]
    public void DetailsPresentation_DoesNotRestoreSuppressedRecoveryDiagnostic(string detailCode, string diagnostic)
    {
        var item = UiItem();
        item.LatestOperation = FailedPresentation("details-op", detailCode, diagnostic);

        var details = item.DetailsPresentation;

        Assert.True(details.HasLatestRecovery);
        Assert.Null(details.LatestRecovery?.UserMessage);
        Assert.Null(details.LatestFailureMessage);
        Assert.False(details.HasLatestFailureMessage);
        Assert.Contains(diagnostic, details.LatestTechnicalDetails);
    }

    private static HomeViewModel CreateViewModel(IGorillaServiceClient client)
        => new(client, new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore()), new OperationTracker(client));

    private static UiOptionalInstallItem UiItem() => new()
    {
        ItemName = "VLC",
        DisplayName = "VLC",
        TargetVersion = "4.0",
        Observation = new Observation(ObservedState.Absent, null, Now, string.Empty, RequirementState.NotSatisfied),
        Policy = new Policy(true, false, false, false, Selection.None),
        InstallDecision = new ActionDecision(true, string.Empty),
        RemoveDecision = new ActionDecision(false, "already_absent"),
    };

    private static OptionalInstallItem ProtocolItem(bool removeAllowed = false)
        => new(
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
                new ActionDecision(true, string.Empty),
                new ActionDecision(removeAllowed, removeAllowed ? string.Empty : "already_absent")
            ),
            Description: "VLC media player"
        );

    private static UiOperationPresentation FailedPresentation(string id, string detailCode, string message)
        => new(
            id,
            CatalogAction.Install,
            OperationState.Completed,
            null,
            new Result(Outcome.Failed, "execution_failed", detailCode, message),
            message,
            Now
        );

    private static OperationStatusEvent Historical(
        string id = "old-op",
        CatalogAction action = CatalogAction.Install,
        DateTimeOffset? timestamp = null)
        => new(
            id,
            OperationState.Completed,
            null,
            action == CatalogAction.Remove ? "Removal error: exit status 7" : "Installation error: exit status 7",
            timestamp ?? Now,
            "VLC",
            action,
            new Result(
                Outcome.Failed,
                "execution_failed",
                "installer_failed",
                action == CatalogAction.Remove ? "Removal error: exit status 7" : "Installation error: exit status 7"
            )
        );

    private static async IAsyncEnumerable<OperationStatusEvent> Empty(
        [EnumeratorCancellation] CancellationToken cancellationToken = default
    )
    {
        await Task.CompletedTask;
        yield break;
    }

    private sealed class InMemoryCacheStore : IOptionalInstallsCacheStore
    {
        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult<OptionalInstallsCacheDocument?>(null);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
            => Task.CompletedTask;
    }

    private sealed class RejectingClient : IGorillaServiceClient
    {
        private readonly string _errorCode;

        public RejectingClient(string errorCode)
        {
            _errorCode = errorCode;
        }

        public IReadOnlyList<OptionalInstallItem> Catalog { get; set; } = [];
        public IReadOnlyList<OperationStatusEvent> Operations { get; set; } = [Historical()];
        public int InstallCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult(Catalog);

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
            => Task.FromResult(Operations);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
        {
            InstallCalls++;
            throw new ServiceErrorException(_errorCode, "Current service truth rejected the request.");
        }

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new InvalidOperationException("Unexpected remove request.");

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            CancellationToken cancellationToken
        ) => Empty(cancellationToken);
    }

    private sealed class FalseAdmissionClient : IGorillaServiceClient
    {
        public IReadOnlyList<OptionalInstallItem> Catalog { get; set; } = [];
        public int InstallCalls { get; private set; }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult(Catalog);

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
            => Task.FromResult<IReadOnlyList<OperationStatusEvent>>([Historical()]);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
        {
            InstallCalls++;
            return Task.FromResult(new OperationAccepted(string.Empty, false, default));
        }

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new InvalidOperationException("Unexpected remove request.");

        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            CancellationToken cancellationToken
        ) => Empty(cancellationToken);
    }
}
