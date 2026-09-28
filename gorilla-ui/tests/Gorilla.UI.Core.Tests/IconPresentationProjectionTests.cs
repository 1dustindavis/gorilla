using Gorilla.UI.Client;
using Gorilla.UI.Core;
using Gorilla.UI.Core.Models;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Xunit;

namespace Gorilla.UI.Core.Tests;

public sealed class IconPresentationProjectionTests
{
    private static readonly DateTimeOffset Now = DateTimeOffset.Parse("2026-09-27T00:00:00Z");

    [Fact]
    public async Task InitializeAsync_ProjectsResolvedIconPath()
    {
        var client = new FakeClient
        {
            Catalog = [ProtocolItem(@"C:\cache\catalog-icons\app.png")],
        };
        var viewModel = CreateViewModel(client);

        await viewModel.InitializeAsync(CancellationToken.None);

        var item = Assert.Single(viewModel.Items);
        Assert.Equal(@"C:\cache\catalog-icons\app.png", item.IconPath);
    }

    [Fact]
    public async Task RefreshCatalogAsync_ClearsPreviousIconPathOnSameCanonicalItem()
    {
        var client = new FakeClient
        {
            Catalog = [ProtocolItem(@"C:\cache\catalog-icons\app.png")],
        };
        var viewModel = CreateViewModel(client);
        await viewModel.InitializeAsync(CancellationToken.None);
        var canonical = Assert.Single(viewModel.Items);

        client.Catalog = [ProtocolItem(null)];
        await viewModel.RefreshCatalogAsync(CancellationToken.None);

        Assert.Same(canonical, Assert.Single(viewModel.Items));
        Assert.Null(canonical.IconPath);
    }

    [Theory]
    [InlineData(null)]
    [InlineData("")]
    public void UiOptionalInstallItem_AllowsMissingIconPath(string? iconPath)
    {
        var item = new UiOptionalInstallItem
        {
            ItemName = "App",
            DisplayName = "App",
            IconPath = iconPath,
        };

        Assert.Equal(iconPath, item.IconPath);
    }

    private static HomeViewModel CreateViewModel(FakeClient client)
    {
        var coordinator = new OptionalInstallsCacheCoordinator(client, new InMemoryCacheStore());
        return new HomeViewModel(client, coordinator, new OperationTracker(client));
    }

    private static OptionalInstallItem ProtocolItem(string? iconPath) => new(
        "App",
        "App",
        "1.0.0",
        "integration",
        "ps1",
        "App",
        "packages/app.ps1",
        true,
        false,
        OptionalInstallStatus.NotInstalled,
        Now,
        null,
        IconPath: iconPath
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
        public IReadOnlyList<OptionalInstallItem> Catalog { get; set; } = [];

        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
            => Task.FromResult(Catalog);

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
            => throw new NotSupportedException();

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
}
