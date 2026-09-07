using System.Text.Json;
using System.Text.Json.Serialization;
using Gorilla.UI.Client.AppCatalog;
using Xunit;

namespace Gorilla.UI.Client.Tests;

public sealed class AppCatalogContractTests
{
    // Isolated from the live v1 serializer. This is the planned v2 contract.
    private static readonly JsonSerializerOptions Options = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow,
        Converters = { new JsonStringEnumConverter(allowIntegerValues: false) },
    };

    private static JsonDocument LoadExamples() => JsonDocument.Parse(
        File.ReadAllText(Path.Combine(AppContext.BaseDirectory, "app-catalog-contract.json"))
    );

    [Fact]
    public void RemoveIsAnOperationButNotAPersistentSelection()
    {
        Assert.Equal(Gorilla.UI.Client.AppCatalog.Action.Remove,
            JsonSerializer.Deserialize<Gorilla.UI.Client.AppCatalog.Action>("\"Remove\"", Options));
        Assert.Throws<JsonException>(() => JsonSerializer.Deserialize<Selection>("\"Remove\"", Options));
        Assert.Equal(Selection.None, JsonSerializer.Deserialize<Selection>("\"None\"", Options));
    }

    [Fact]
    public void SharedGoExamplesPreservePresenceSelectionAndOperationIndependently()
    {
        using var examples = LoadExamples();
        var items = examples.RootElement.GetProperty("items").Deserialize<Item[]>(Options)!;
        var installed = Assert.Single(items, item => item.ItemName == "Example");
        Assert.Equal(ObservedState.Installed, installed.Observation.State);
        Assert.Null(installed.Observation.InstalledVersion);
        Assert.Equal("2.0", installed.TargetVersion);
        Assert.Equal(Selection.Install, installed.Policy.Selection);
        Assert.False(installed.Actions.Install.Allowed);
        Assert.Equal("already_selected", installed.Actions.Install.Reason);
        Assert.True(installed.Actions.Remove.Allowed);
        Assert.Equal(Outcome.Succeeded, installed.LastOperation!.Result!.Outcome);
        Assert.Null(installed.ActiveOperation);

        var outdated = Assert.Single(items, item => item.ItemName == "Older");
        Assert.Equal(ObservedState.UpdateAvailable, outdated.Observation.State);
        Assert.Equal("1.0", outdated.Observation.InstalledVersion);
        Assert.Equal("2.0", outdated.TargetVersion);

        var pending = Assert.Single(items, item => item.ItemName == "Pending");
        Assert.Equal(ObservedState.Absent, pending.Observation.State);
        Assert.Equal(Selection.Install, pending.Policy.Selection);
        Assert.Equal(OperationPhase.Queued, pending.ActiveOperation!.Phase);
        Assert.Null(pending.ActiveOperation.ProgressPercent);
        Assert.Null(pending.ActiveOperation.Result);
        Assert.False(pending.Actions.Install.Allowed);
        Assert.False(pending.Actions.Remove.Allowed);

        var unknown = Assert.Single(items, item => item.ItemName == "Uncertain");
        Assert.Equal(ObservedState.Unknown, unknown.Observation.State);
        Assert.Null(unknown.Observation.CheckedAtUtc);
        Assert.Null(unknown.TargetVersion);
        Assert.Equal("state_unknown", unknown.Actions.Install.Reason);
        var failed = Assert.Single(items, item => item.ItemName == "FailedCheck");
        Assert.Equal(ObservedState.DetectionFailed, failed.Observation.State);
        Assert.NotNull(failed.Observation.CheckedAtUtc);
        Assert.Equal("check_failed", failed.Observation.DetailCode);
    }

    [Fact]
    public void SharedResultExamplesExposeEveryOutcomeToTheClient()
    {
        using var examples = LoadExamples();
        var results = examples.RootElement.GetProperty("resultCases").EnumerateArray()
            .Select(example => example.GetProperty("expected").Deserialize<Result>(Options)!)
            .ToArray();
        foreach (var outcome in Enum.GetValues<Outcome>())
        {
            Assert.Contains(results, result => result.Outcome == outcome);
        }
        Assert.All(results.Where(result => result.Outcome is Outcome.Failed or Outcome.Unverified or Outcome.Interrupted),
            result => Assert.False(string.IsNullOrWhiteSpace(result.Code)));
    }

    [Fact]
    public void ContractSerializationKeepsNullEvidenceAndUsesNamedStates()
    {
        using var examples = LoadExamples();
        foreach (var source in examples.RootElement.GetProperty("items").EnumerateArray())
        {
            var item = source.Deserialize<Item>(Options)!;
            var json = JsonSerializer.Serialize(item, Options);
            Assert.Equal(item, JsonSerializer.Deserialize<Item>(json, Options));
            using var output = JsonDocument.Parse(json);
            Assert.Equal(source.GetProperty("observation").GetProperty("state").GetString(),
                output.RootElement.GetProperty("observation").GetProperty("state").GetString());
            Assert.Equal(source.GetProperty("observation").GetProperty("installedVersion").ValueKind,
                output.RootElement.GetProperty("observation").GetProperty("installedVersion").ValueKind);
            Assert.Equal(source.GetProperty("activeOperation").ValueKind,
                output.RootElement.GetProperty("activeOperation").ValueKind);
        }
    }
}
