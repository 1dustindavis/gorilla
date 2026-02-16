using Gorilla.UI.Client;

namespace Gorilla.UI.App.Services;

public static class GorillaUiServices
{
    public static (IGorillaServiceClient Client, OptionalInstallsCacheCoordinator CacheCoordinator) Create(string cacheFilePath)
    {
        var options = NamedPipeClientOptions.Default;
        var client = new NamedPipeGorillaServiceClient(options);
        var cacheStore = new JsonFileOptionalInstallsCacheStore(cacheFilePath);
        var coordinator = new OptionalInstallsCacheCoordinator(client, cacheStore);

        return (client, coordinator);
    }
}
