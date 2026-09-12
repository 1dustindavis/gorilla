using System.ComponentModel;
using Gorilla.UI.Client;
using Gorilla.UI.Client.AppCatalog;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public sealed class HomeViewModelDetailsSelectionTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-11T20:00:00Z");

    [Fact]
    public async Task SelectedDetailsResolveByItemNameAndStayOnCanonicalInstanceAcrossRefresh()
    {
        var client = new FakeClient([
            [Item("Example", "Original", "Original description", "1.0")],
            [Item("Example", "Updated", "Updated description", "2.0")],
        ]);
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);
        Assert.True(viewModel.SelectItem("Example"));
        var selectedBefore = viewModel.SelectedItem;

        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.Equal("Example", viewModel.SelectedItemName);
        Assert.Same(selectedBefore, viewModel.SelectedItem);
        Assert.Equal("Updated", viewModel.SelectedItem?.DisplayName);
        Assert.Equal("Updated description", viewModel.SelectedItem?.DetailsPresentation.Description);
        Assert.Equal("2.0", viewModel.SelectedItem?.DetailsPresentation.AvailableVersion);
    }

    [Fact]
    public async Task SelectionChangesDoNotResetSearchAndCanSelectFilteredCanonicalItem()
    {
        var viewModel = CreateViewModel(new FakeClient([
            [Item("Example", "Example", "Find me", "1.0"), Item("Other", "Other", null, "1.0")],
        ]));
        await viewModel.InitializeAsync(CancellationToken.None);
        viewModel.SearchQuery = "Other";

        Assert.True(viewModel.SelectItem("Example"));

        Assert.Equal("Other", viewModel.SearchQuery);
        Assert.Equal("Example", viewModel.SelectedItemName);
        Assert.Equal("Example", viewModel.SelectedItem?.ItemName);
        Assert.Equal(["Other"], viewModel.Items.Select(item => item.ItemName));
    }

    [Fact]
    public async Task AuthoritativeRemovalClearsSelectionInsteadOfSubstitutingAnotherItem()
    {
        var client = new FakeClient([
            [Item("Example", "Example", null, "1.0"), Item("Other", "Other", null, "1.0")],
            [Item("Other", "Other", null, "1.0")],
        ]);
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);
        Assert.True(viewModel.SelectItem("Example"));

        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.Null(viewModel.SelectedItemName);
        Assert.Null(viewModel.SelectedItem);
        Assert.NotNull(viewModel.FindItem("Other"));
        Assert.Null(viewModel.FindItem("Example"));
    }

    [Fact]
    public async Task AuthoritativeRemovalClearsSelectedItemBeforeVisibleItemsAreRebuilt()
    {
        var client = new FakeClient([
            [Item("Example", "Example", null, "1.0"), Item("Other", "Other", null, "1.0")],
            [Item("Other", "Other", null, "1.0")],
        ]);
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);
        Assert.True(viewModel.SelectItem("Example"));
        var selectedBefore = viewModel.SelectedItem;
        Assert.NotNull(selectedBefore);

        UiOptionalInstallItem? selectedSeenWhenSelectionCleared = selectedBefore;
        bool staleVisibleItemStillPresentWhenSelectionCleared = false;
        viewModel.PropertyChanged += (_, args) =>
        {
            if (args.PropertyName != nameof(HomeViewModel.SelectedItemName) || viewModel.SelectedItemName is not null)
            {
                return;
            }

            selectedSeenWhenSelectionCleared = viewModel.SelectedItem;
            staleVisibleItemStillPresentWhenSelectionCleared = viewModel.Items.Contains(selectedBefore!);
        };

        await viewModel.InitializeAsync(CancellationToken.None);

        Assert.Null(selectedSeenWhenSelectionCleared);
        Assert.True(staleVisibleItemStillPresentWhenSelectionCleared);
        Assert.DoesNotContain(selectedBefore!, viewModel.Items);
    }

    private static HomeViewModel CreateViewModel(FakeClient client)
        => new(client, new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore()), new OperationTracker(client));

    private static OptionalInstallItem Item(string itemName, string displayName, string? description, string targetVersion)
        => new(
            itemName,
            displayName,
            targetVersion,
            "catalog",
            "msi",
            "package",
            "installer.msi",
            true,
            false,
            OptionalInstallStatus.NotInstalled,
            Now,
            null,
            targetVersion,
            new Observation(ObservedState.Absent, null, Now, string.Empty, RequirementState.Unknown),
            Policy: new Policy(true, false, false, false, Selection.None),
            Actions: new Actions(new ActionDecision(true, string.Empty), new ActionDecision(false, "already_absent")),
            Description: description
        );

    private sealed class InMemoryCacheStore : IOptionalInstallsCacheStore
    {
        private OptionalInstallsCacheDocument? _document;

        public Task<OptionalInstallsCacheDocument?> LoadAsync(CancellationToken cancellationToken)
            => Task.FromResult(_document);

        public Task SaveAsync(OptionalInstallsCacheDocument document, CancellationToken cancellationToken)
        {
            _document = document;
            return Task.CompletedTask;
        }
    }

    private sealed class FakeClient : IGorillaServiceClient
    {
        private readonly IReadOnlyList<IReadOnlyList<OptionalInstallItem>> _catalogs;
        private int _catalogCall;

        public FakeClient(IReadOnlyList<IReadOnlyList<OptionalInstallItem>> catalogs)
        {
            _catalogs = catalogs;
        }

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            var index = Math.Min(_catalogCall++, _catalogs.Count - 1);
            return Task.FromResult(_catalogs[index]);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

        public Task<IReadOnlyList<OperationStatusEvent>> ListOperationsAsync(CancellationToken cancellationToken)
            => Task.FromResult<IReadOnlyList<OperationStatusEvent>>([]);

        public async IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            [System.Runtime.CompilerServices.EnumeratorCancellation] CancellationToken cancellationToken)
        {
            await Task.CompletedTask;
            yield break;
        }
    }
}
