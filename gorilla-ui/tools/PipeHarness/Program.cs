using System.Text.Json;
using Gorilla.UI.Client;

var (pipeName, commandArgs) = ParseArgs(args);
if (commandArgs.Count == 0)
{
    PrintUsage();
    return 1;
}

var options = NamedPipeClientOptions.Default with { PipeName = pipeName };
var client = new NamedPipeGorillaServiceClient(options);

try
{
    var command = commandArgs[0].ToLowerInvariant();
    using var cts = new CancellationTokenSource();
    Console.CancelKeyPress += (_, e) =>
    {
        e.Cancel = true;
        cts.Cancel();
    };

    switch (command)
    {
        case "list":
            await RunListAsync(client, ParseRefreshOption(commandArgs), cts.Token);
            break;
        case "install":
            RequireArgument(commandArgs, 2, "install <itemName>");
            await RunInstallAsync(client, commandArgs[1], cts.Token);
            break;
        case "remove":
            RequireArgument(commandArgs, 2, "remove <itemName>");
            await RunRemoveAsync(client, commandArgs[1], cts.Token);
            break;
        case "stream":
            RequireArgument(commandArgs, 2, "stream <operationId>");
            await RunStreamAsync(client, commandArgs[1], cts.Token);
            break;
        default:
            Console.Error.WriteLine($"Unknown command: {commandArgs[0]}");
            PrintUsage();
            return 2;
    }

    return 0;
}
catch (OperationCanceledException)
{
    Console.Error.WriteLine("Operation cancelled.");
    return 130;
}
catch (Exception ex)
{
    Console.Error.WriteLine($"Error: {ex.Message}");
    return 1;
}

static async Task RunListAsync(
    IGorillaServiceClient client,
    bool refresh,
    CancellationToken cancellationToken
)
{
    var result = await client.ListOptionalInstallsAsync(refresh, cancellationToken);
    Console.WriteLine($"snapshotAvailable: {result.SnapshotAvailable}");
    Console.WriteLine($"snapshotGeneratedAtUtc: {result.SnapshotGeneratedAtUtc:O}");
    Console.WriteLine($"refreshState: {result.RefreshState}");
    Console.WriteLine($"itemCount: {result.Items.Count}");

    foreach (var item in result.Items)
    {
        Console.WriteLine(JsonSerializer.Serialize(item, ProtocolJson.Options));
    }
}

static bool ParseRefreshOption(IReadOnlyList<string> args)
{
    if (args.Count == 1)
    {
        return false;
    }

    if (args.Count != 2)
    {
        throw new ArgumentException("Usage: list [--refresh|--no-refresh]");
    }

    return args[1] switch
    {
        "--refresh" => true,
        "--no-refresh" => false,
        _ => throw new ArgumentException("Usage: list [--refresh|--no-refresh]"),
    };
}

static async Task RunInstallAsync(IGorillaServiceClient client, string itemName, CancellationToken cancellationToken)
{
    var accepted = await client.InstallItemAsync(itemName, cancellationToken);
    Console.WriteLine($"accepted: {accepted.Accepted}");
    Console.WriteLine($"operationId: {accepted.OperationId}");
    Console.WriteLine($"queuedAtUtc: {accepted.QueuedAtUtc:O}");
}

static async Task RunRemoveAsync(IGorillaServiceClient client, string itemName, CancellationToken cancellationToken)
{
    var accepted = await client.RemoveItemAsync(itemName, cancellationToken);
    Console.WriteLine($"accepted: {accepted.Accepted}");
    Console.WriteLine($"operationId: {accepted.OperationId}");
    Console.WriteLine($"queuedAtUtc: {accepted.QueuedAtUtc:O}");
}

static async Task RunStreamAsync(IGorillaServiceClient client, string operationId, CancellationToken cancellationToken)
{
    await foreach (var update in client.StreamOperationStatusAsync(operationId, cancellationToken))
    {
        Console.WriteLine(JsonSerializer.Serialize(update, ProtocolJson.Options));
    }
}

static void RequireArgument(IReadOnlyList<string> args, int expectedCount, string usage)
{
    if (args.Count >= expectedCount)
    {
        return;
    }

    throw new ArgumentException($"Missing argument. Usage: {usage}");
}

static (string pipeName, List<string> commandArgs) ParseArgs(string[] args)
{
    var pipeName = Environment.GetEnvironmentVariable("GORILLA_PIPE_NAME") ?? NamedPipeClientOptions.Default.PipeName;
    var remaining = new List<string>();

    for (var i = 0; i < args.Length; i++)
    {
        if ((args[i] == "--pipe" || args[i] == "-p") && i+1 < args.Length)
        {
            pipeName = args[++i];
            continue;
        }

        remaining.Add(args[i]);
    }

    return (pipeName, remaining);
}

static void PrintUsage()
{
    Console.WriteLine("Gorilla Pipe Harness");
    Console.WriteLine();
    Console.WriteLine("Usage:");
    Console.WriteLine("  dotnet run --project gorilla-ui/tools/PipeHarness -- [--pipe <name>] list [--refresh|--no-refresh]");
    Console.WriteLine("  dotnet run --project gorilla-ui/tools/PipeHarness -- [--pipe <name>] install <itemName>");
    Console.WriteLine("  dotnet run --project gorilla-ui/tools/PipeHarness -- [--pipe <name>] remove <itemName>");
    Console.WriteLine("  dotnet run --project gorilla-ui/tools/PipeHarness -- [--pipe <name>] stream <operationId>");
    Console.WriteLine();
    Console.WriteLine("Defaults:");
    Console.WriteLine($"  pipe name: {NamedPipeClientOptions.Default.PipeName} (or env GORILLA_PIPE_NAME)");
    Console.WriteLine("  list refresh: false");
}
