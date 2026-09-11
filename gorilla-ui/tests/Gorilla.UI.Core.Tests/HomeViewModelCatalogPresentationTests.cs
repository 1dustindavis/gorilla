using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;
using AppCatalog = Gorilla.UI.Client.AppCatalog;

namespace Gorilla.UI.Core.Tests;

public class HomeViewModelCatalogPresentationTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-11T05:00:00Z");

    [Fact]
    public async Task InitializeAsync_OrdersItemsDeterministicallyByDisplayNameThenItemName()
    {
        var viewModel = CreateViewModel(new FakeClient
        {
            Catalogs = [[
                Item("Zulu", "Zulu"), Item("alpha", "alpha"), Item("Beta", "Beta"),
                Item("z-last", "Same"), Item("A-first", "Same"),
            ]],
        });

        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.Equal(["alpha", "Beta", "A-first", "z-last", "Zulu"], viewModel.Items.Select(item => item.ItemName));
    }

    [Fact]
    public async Task Search_MatchesOnlyDisplayNameItemNameAndDescription()
    {
        var viewModel = CreateViewModel(new FakeClient
        {
            Catalogs = [[
                Item("GoogleChrome", "Chrome", description: "A secure web browser."),
                Item("VLC", "Media player", catalog: "browser-only-catalog", installerLocation: "browser.msi"),
            ]],
        });
        await viewModel.InitializeAsync(CancellationToken.None);

        viewModel.SearchQuery = "CHROME";
        Assert.Equal("GoogleChrome", Assert.Single(viewModel.Items).ItemName);

        viewModel.SearchQuery = "secure";
        Assert.Equal("GoogleChrome", Assert.Single(viewModel.Items).ItemName);

        viewModel.SearchQuery = "browser-only-catalog";
        Assert.Empty(viewModel.Items);

        viewModel.SearchQuery = "browser.msi";
        Assert.Empty(viewModel.Items);
    }

    [Fact]
    public async Task Refresh_ReconcilesInPlacePreservesQueryAndUpdatesSearchMetadata()
    {
        var client = new FakeClient
        {
            Catalogs = [
                [Item("Example", "Old name", description: "Editor", targetVersion: "1.0")],
                [Item("Example", "New name", description: "Web browser", targetVersion: "2.0", observation: Observation(ObservedState.UpdateAvailable, "1.7"))],
            ],
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);
        var before = Assert.Single(viewModel.Items);

        viewModel.SearchQuery = "browser";
        Assert.Empty(viewModel.Items);
        await viewModel.InitializeAsync(CancellationToken.None);

        var after = Assert.Single(viewModel.Items);
        Assert.Same(before, after);
        Assert.Equal("browser", viewModel.SearchQuery);
        Assert.Equal("New name", after.DisplayName);
        Assert.Equal("Web browser", after.Description);
        Assert.Equal("2.0", after.TargetVersion);
        Assert.Equal("1.7", after.InstalledVersion);
        Assert.Equal(ObservedState.UpdateAvailable, after.ObservedState);
    }

    [Fact]
    public async Task InitializeAsync_KeepsTargetAndInstalledVersionsDistinctIncludingMissingInstalledVersion()
    {
        var viewModel = CreateViewModel(new FakeClient
        {
            Catalogs = [[Item("Example", "Example", targetVersion: "2.0", observation: Observation(ObservedState.UpdateAvailable, null))]],
        });

        await viewModel.InitializeAsync(CancellationToken.None);

        var item = Assert.Single(viewModel.Items);
        Assert.Equal("2.0", item.TargetVersion);
        Assert.Null(item.InstalledVersion);
        Assert.Equal(ObservedState.UpdateAvailable, item.ObservedState);
    }

    [Fact]
    public async Task InitializeAsync_AllowsMissingTargetVersion()
    {
        var viewModel = CreateViewModel(new FakeClient
        {
            Catalogs = [[Item("Example", "Example", targetVersion: null, legacyVersion: "")]],
        });

        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.Null(Assert.Single(viewModel.Items).TargetVersion);
    }

    [Fact]
    public async Task Refresh_AddsAndRemovesItemsAndSelectionUsesCanonicalIdentity()
    {
        var viewModel = CreateViewModel(new FakeClient
        {
            Catalogs = [
                [Item("Example", "Example"), Item("Gone", "Gone")],
                [Item("Example", "Example"), Item("New", "New")],
                [Item("New", "New")],
            ],
        });
        await viewModel.InitializeAsync(CancellationToken.None);
        Assert.True(viewModel.SelectItem("Example"));

        viewModel.SearchQuery = "new";
        Assert.Empty(viewModel.Items);
        Assert.Equal("Example", viewModel.SelectedItemName);
        Assert.Equal("Example", viewModel.SelectedItem?.ItemName);

        await viewModel.InitializeAsync(CancellationToken.None);
        Assert.Equal(["New"], viewModel.Items.Select(item => item.ItemName));
        Assert.Equal("Example", viewModel.SelectedItemName);

        await viewModel.InitializeAsync(CancellationToken.None);
        Assert.Null(viewModel.SelectedItemName);
        Assert.Null(viewModel.SelectedItem);
    }

    [Theory]
    [InlineData(true, false)]
    [InlineData(false, true)]
    [InlineData(true, true)]
    [InlineData(false, false)]
    public async Task InitializeAsync_PreservesBothServiceActionDecisions(bool installAllowed, bool removeAllowed)
    {
        var viewModel = CreateViewModel(new FakeClient
        {
            Catalogs = [[Item(
                "Example", "Example", observation: Observation(ObservedState.Installed, "1.7"),
                install: new ActionDecision(installAllowed, "install_reason"),
                remove: new ActionDecision(removeAllowed, "remove_reason"))]],
        });

        await viewModel.InitializeAsync(CancellationToken.None);

        var item = Assert.Single(viewModel.Items);
        Assert.Equal(installAllowed, item.InstallDecision.Allowed);
        Assert.Equal(removeAllowed, item.RemoveDecision.Allowed);
        Assert.Equal("install_reason", item.InstallDecision.Reason);
        Assert.Equal("remove_reason", item.RemoveDecision.Reason);
    }

    [Fact]
    public async Task InitializeAsync_PreservesServicePolicyWithoutInference()
    {
        var policy = new Policy(
            Optional: false,
            RequiredInstall: true,
            RequiredUninstall: true,
            RequiredDependency: true,
            Selection: Selection.Install
        );
        var viewModel = CreateViewModel(new FakeClient
        {
            Catalogs = [[Item("Example", "Example", policy: policy)]],
        });

        await viewModel.InitializeAsync(CancellationToken.None);

        var item = Assert.Single(viewModel.Items);
        Assert.Equal(policy, item.Policy);
        Assert.True(item.Policy!.RequiredInstall);
        Assert.True(item.Policy.RequiredUninstall);
        Assert.True(item.Policy.RequiredDependency);
        Assert.Equal(Selection.Install, item.Policy.Selection);
    }

    [Fact]
    public async Task InitializeAsync_ProjectsActiveOperationSeparatelyFromObservation()
    {
        var active = new OperationStatusEvent("op-1", OperationState.Removing, 50, "Removing", Now, "Example", AppCatalog.Action.Remove);
        var viewModel = CreateViewModel(new FakeClient
        {
            Catalogs = [[Item("Example", "Example", observation: Observation(ObservedState.Installed, "1.7"))]],
            Operations = [active],
        });

        await viewModel.InitializeAsync(CancellationToken.None);

        var item = Assert.Single(viewModel.Items);
        Assert.Equal(ObservedState.Installed, item.ObservedState);
        Assert.Equal("1.7", item.InstalledVersion);
        Assert.Equal("op-1", item.ActiveOperation?.OperationId);
        Assert.Equal(AppCatalog.Action.Remove, item.ActiveOperation?.Action);
        Assert.Equal(OperationState.Removing, item.ActiveOperation?.State);
        Assert.Null(item.LatestOperation);
    }

    [Fact]
    public async Task Refresh_PreservesActiveOperationOnTheReconciledItem()
    {
        var active = new OperationStatusEvent("op-1", OperationState.Installing, 30, "Installing", Now, "Example", AppCatalog.Action.Install);
        var viewModel = CreateViewModel(new FakeClient
        {
            Catalogs = [
                [Item("Example", "Example", description: "Old")],
                [Item("Example", "Example updated", description: "New")],
            ],
            Operations = [active],
        });
        await viewModel.InitializeAsync(CancellationToken.None);
        var before = Assert.Single(viewModel.Items);

        await viewModel.InitializeAsync(CancellationToken.None);

        var after = Assert.Single(viewModel.Items);
        Assert.Same(before, after);
        Assert.Equal("Example updated", after.DisplayName);
        Assert.Equal("op-1", after.ActiveOperation?.OperationId);
        Assert.Equal(OperationState.Installing, after.ActiveOperation?.State);
    }

    [Theory]
    [InlineData(Outcome.Succeeded)]
    [InlineData(Outcome.Failed)]
    [InlineData(Outcome.Unverified)]
    [InlineData(Outcome.Interrupted)]
    public async Task TerminalOperation_RetainsExactStructuredOutcome(Outcome outcome)
    {
        var client = new FakeClient
        {
            Catalogs = [[Item("Example", "Example")], [Item("Example", "Example")]],
            StreamAsync = (_, _) => StatusStream(new OperationStatusEvent(
                "op-1", OperationState.Completed, null, "Done", Now, "Example", AppCatalog.Action.Install,
                new Result(outcome, $"{outcome}_code", Message: "Done"))),
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);

        await viewModel.InstallAsync(Assert.Single(viewModel.Items), CancellationToken.None);

        var item = Assert.Single(viewModel.Items);
        Assert.Null(item.ActiveOperation);
        Assert.Equal(outcome, item.LatestOperation?.Result?.Outcome);
        Assert.Equal($"{outcome}_code", item.LatestOperation?.Result?.Code);
    }

    private static HomeViewModel CreateViewModel(FakeClient client)
        => new(client, new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore()), new OperationTracker(client));

    private static OptionalInstallItem Item(
        string itemName,
        string displayName,
        string? description = null,
        string catalog = "catalog",
        string installerLocation = "installer.msi",
        string? targetVersion = "2.0",
        string legacyVersion = "legacy-version",
        Observation? observation = null,
        Policy? policy = null,
        ActionDecision? install = null,
        ActionDecision? remove = null)
        => new(itemName, displayName, legacyVersion, catalog, "msi", "package", installerLocation,
            true, false, OptionalInstallStatus.NotInstalled, Now, null, targetVersion, observation, Policy: policy,
            Actions: new Actions(install ?? new ActionDecision(true, ""), remove ?? new ActionDecision(false, "not_installed")),
            Description: description);

    private static Observation Observation(ObservedState state, string? installedVersion)
        => new(state, installedVersion, Now, "detail_code", RequirementState.Unknown);

    private static async IAsyncEnumerable<OperationStatusEvent> StatusStream(params OperationStatusEvent[] events)
    {
        foreach (var operation in events)
        {
            await Task.Yield();
            yield return operation;
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
        private int _catalogCall;
        public IReadOnlyList<IReadOnlyList<OptionalInstallItem>> Catalogs { get; init; } = [[]];
        public IReadOnlyList<OperationStatusEvent> Operations { get; init; } = [];
        public Func<string, CancellationToken, IAsyncEnumerable<OperationStatusEvent>> StreamAsync { get; init; } = (_, _) => StatusStream();

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            var index = Math.Min(_catalogCall++, Catalogs.Count - 1);
            return Task.FromResult(Catalogs[index]);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => Task.FromResult(new OperationAccepted("op-1", true, Now));
        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => Task.FromResult(new OperationAccepted("op-2", true, Now));
        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken) => Task.FromResult(Operations);
        public IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(string operationId, CancellationToken cancellationToken)
            => StreamAsync(operationId, cancellationToken);
    }
}
