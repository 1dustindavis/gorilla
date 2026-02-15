using Gorilla.UI.Client;
using Xunit;

namespace Gorilla.UI.Client.Tests;

public class OptionalInstallsCacheTests
{
    [Fact]
    public async Task JsonFileOptionalInstallsCacheStore_RoundTripsDocument()
    {
        var tempDir = Path.Combine(Path.GetTempPath(), "gorilla-ui-cache-tests", Guid.NewGuid().ToString("N"));
        var cachePath = Path.Combine(tempDir, "optional-installs.json");

        var store = new JsonFileOptionalInstallsCacheStore(cachePath);
        var now = DateTimeOffset.Parse("2026-02-14T18:10:00Z");
        var doc = new OptionalInstallsCacheDocument(
            CachedAtUtc: now,
            Items:
            [
                new OptionalInstallItem(
                    ItemName: "GoogleChrome",
                    DisplayName: "Google Chrome",
                    Version: "68.0.3440.106",
                    Catalog: "testcatalog",
                    InstallerType: "nupkg",
                    InstallerPackageId: "GoogleChrome",
                    InstallerLocation: "packages/google-chrome/GoogleChrome.68.0.3440.106.nupkg",
                    IsManaged: true,
                    IsInstalled: false,
                    Status: OptionalInstallStatus.NotInstalled,
                    StatusUpdatedAtUtc: now,
                    LastOperationId: null
                ),
            ]
        );

        await store.SaveAsync(doc, CancellationToken.None);

        var loaded = await store.LoadAsync(CancellationToken.None);

        Assert.NotNull(loaded);
        Assert.Equal(now, loaded!.CachedAtUtc);
        Assert.Single(loaded.Items);
        Assert.Equal("GoogleChrome", loaded.Items[0].ItemName);

        Directory.Delete(tempDir, recursive: true);
    }

    [Fact]
    public async Task OptionalInstallsCacheCoordinator_RefreshSavesAndReturnsFreshData()
    {
        var client = new FakeGorillaServiceClient();

        var tempDir = Path.Combine(Path.GetTempPath(), "gorilla-ui-cache-tests", Guid.NewGuid().ToString("N"));
        var cachePath = Path.Combine(tempDir, "optional-installs.json");
        var store = new JsonFileOptionalInstallsCacheStore(cachePath);
        var coordinator = new OptionalInstallsCacheCoordinator(client, store);

        var cachedBefore = await coordinator.LoadCachedAsync(CancellationToken.None);
        Assert.Null(cachedBefore);

        var refreshed = await coordinator.RefreshAsync(CancellationToken.None);

        Assert.Single(refreshed.Items);
        Assert.Equal("VLC", refreshed.Items[0].ItemName);

        var cachedAfter = await coordinator.LoadCachedAsync(CancellationToken.None);
        Assert.NotNull(cachedAfter);
        Assert.Single(cachedAfter!.Items);
        Assert.Equal("VLC", cachedAfter.Items[0].ItemName);

        Directory.Delete(tempDir, recursive: true);
    }

    [Fact]
    public async Task JsonFileOptionalInstallsCacheStore_MalformedJson_ReturnsNull()
    {
        var tempDir = Path.Combine(Path.GetTempPath(), "gorilla-ui-cache-tests", Guid.NewGuid().ToString("N"));
        var cachePath = Path.Combine(tempDir, "optional-installs.json");
        Directory.CreateDirectory(tempDir);
        await File.WriteAllTextAsync(cachePath, "{\"cachedAtUtc\":\"2026-02-14T18:10:00Z\",\"items\":[", CancellationToken.None);

        var store = new JsonFileOptionalInstallsCacheStore(cachePath);
        var loaded = await store.LoadAsync(CancellationToken.None);

        Assert.Null(loaded);
        Directory.Delete(tempDir, recursive: true);
    }

    [Fact]
    public async Task JsonFileOptionalInstallsCacheStore_NullItems_CoercesToEmpty()
    {
        var tempDir = Path.Combine(Path.GetTempPath(), "gorilla-ui-cache-tests", Guid.NewGuid().ToString("N"));
        var cachePath = Path.Combine(tempDir, "optional-installs.json");
        Directory.CreateDirectory(tempDir);
        await File.WriteAllTextAsync(
            cachePath,
            "{\"cachedAtUtc\":\"2026-02-14T18:10:00Z\",\"items\":null}",
            CancellationToken.None
        );

        var store = new JsonFileOptionalInstallsCacheStore(cachePath);
        var loaded = await store.LoadAsync(CancellationToken.None);

        Assert.NotNull(loaded);
        Assert.Empty(loaded!.Items);
        Directory.Delete(tempDir, recursive: true);
    }

    private sealed class FakeGorillaServiceClient : IGorillaServiceClient
    {
        public Task<IReadOnlyList<OptionalInstallItem>> ListOptionalInstallsAsync(CancellationToken cancellationToken)
        {
            var now = DateTimeOffset.Parse("2026-02-14T18:10:00Z");
            IReadOnlyList<OptionalInstallItem> items =
            [
                new OptionalInstallItem(
                    ItemName: "VLC",
                    DisplayName: "VLC media player",
                    Version: "3.0.21",
                    Catalog: "testcatalog",
                    InstallerType: "nupkg",
                    InstallerPackageId: "VLC",
                    InstallerLocation: "packages/vlc/VLC.3.0.21.nupkg",
                    IsManaged: true,
                    IsInstalled: true,
                    Status: OptionalInstallStatus.Installed,
                    StatusUpdatedAtUtc: now,
                    LastOperationId: "op-1"
                ),
            ];

            return Task.FromResult(items);
        }

        public Task<OperationAccepted> InstallItemAsync(string itemName, CancellationToken cancellationToken)
        {
            throw new NotSupportedException();
        }

        public Task<OperationAccepted> RemoveItemAsync(string itemName, CancellationToken cancellationToken)
        {
            throw new NotSupportedException();
        }

        public async IAsyncEnumerable<OperationStatusEvent> StreamOperationStatusAsync(
            string operationId,
            [System.Runtime.CompilerServices.EnumeratorCancellation] CancellationToken cancellationToken
        )
        {
            await Task.CompletedTask;
            yield break;
        }
    }
}
