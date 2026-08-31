using Gorilla.UI.Client;
using Gorilla.UI.Core;

namespace Gorilla.UI.App.Services;

public static class GorillaUiServices
{
    public static (IGorillaServiceClient Client, OptionalInstallsCacheCoordinator CacheCoordinator) Create(
        string cacheFilePath,
        string? pipeName = null
    )
    {
        var options = NamedPipeClientOptions.Default;
        if (!string.IsNullOrWhiteSpace(pipeName))
        {
            options = options with { PipeName = pipeName };
        }

        var client = new NamedPipeGorillaServiceClient(options);
        var cacheStore = new JsonFileOptionalInstallsCacheStore(cacheFilePath);
        var coordinator = new OptionalInstallsCacheCoordinator(client, cacheStore);

        return (client, coordinator);
    }
}
