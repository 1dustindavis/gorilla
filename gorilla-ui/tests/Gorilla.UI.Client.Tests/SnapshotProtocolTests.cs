using System.Text.Json;
using Gorilla.UI.Client;
using Xunit;

namespace Gorilla.UI.Client.Tests;

public sealed class SnapshotProtocolTests
{
    private static readonly DateTimeOffset GeneratedAt = DateTimeOffset.Parse("2026-09-30T18:00:00Z");

    [Theory]
    [InlineData(true)]
    [InlineData(false)]
    public void ListOptionalInstallsRequest_SerializesExplicitRefreshIntent(bool refresh)
    {
        var json = JsonSerializer.Serialize(new ListOptionalInstallsRequest(refresh), ProtocolJson.Options);
        using var document = JsonDocument.Parse(json);

        Assert.Equal(refresh, document.RootElement.GetProperty("refresh").GetBoolean());
        Assert.Single(document.RootElement.EnumerateObject());
    }

    [Theory]
    [InlineData("Idle", CatalogRefreshState.Idle)]
    [InlineData("Queued", CatalogRefreshState.Queued)]
    [InlineData("Running", CatalogRefreshState.Running)]
    [InlineData("Failed", CatalogRefreshState.Failed)]
    public void ValidateListResponse_MapsKnownRefreshStates(string wireState, CatalogRefreshState expected)
    {
        var payload = Response(
            snapshotAvailable: true,
            generatedAt: GeneratedAt,
            state: wireState,
            errorCode: wireState == "Failed" ? "refresh_failed" : null
        );

        Assert.Equal(expected, ProtocolValidation.ValidateListOptionalInstallsResponse(payload));
    }

    [Fact]
    public void ValidateListResponse_RejectsUnknownRefreshState()
    {
        var payload = Response(true, GeneratedAt, "Waiting");

        Assert.Throws<ProtocolValidationException>(
            () => ProtocolValidation.ValidateListOptionalInstallsResponse(payload)
        );
    }

    [Fact]
    public void ValidateListResponse_RejectsAvailableSnapshotWithoutGenerationTime()
    {
        var payload = Response(true, null, "Idle");

        Assert.Throws<ProtocolValidationException>(
            () => ProtocolValidation.ValidateListOptionalInstallsResponse(payload)
        );
    }

    [Fact]
    public void UnavailableSnapshot_OmittedGenerationTime_DeserializesAndValidates()
    {
        const string json = "{\"items\":[],\"snapshotAvailable\":false,\"refreshState\":\"Running\"}";

        var payload = JsonSerializer.Deserialize<ListOptionalInstallsResponse>(json, ProtocolJson.Options);

        Assert.NotNull(payload);
        Assert.Null(payload!.SnapshotGeneratedAtUtc);
        Assert.Equal(
            CatalogRefreshState.Running,
            ProtocolValidation.ValidateListOptionalInstallsResponse(payload)
        );
    }

    [Fact]
    public void ValidateListResponse_RejectsUnavailableSnapshotWithItems()
    {
        var payload = new ListOptionalInstallsResponse(
            Items: [Item()],
            SnapshotAvailable: false,
            SnapshotGeneratedAtUtc: null,
            RefreshState: "Running"
        );

        Assert.Throws<ProtocolValidationException>(
            () => ProtocolValidation.ValidateListOptionalInstallsResponse(payload)
        );
    }

    [Fact]
    public void ValidateListResponse_RejectsUnavailableSnapshotWithGenerationTime()
    {
        var payload = Response(false, GeneratedAt, "Running");

        Assert.Throws<ProtocolValidationException>(
            () => ProtocolValidation.ValidateListOptionalInstallsResponse(payload)
        );
    }

    [Fact]
    public void ValidateListResponse_AcceptsAuthoritativeEmptySnapshot()
    {
        var payload = Response(true, GeneratedAt, "Idle");

        Assert.Equal(CatalogRefreshState.Idle, ProtocolValidation.ValidateListOptionalInstallsResponse(payload));
        Assert.Empty(payload.Items);
    }

    [Fact]
    public void ValidateListResponse_PreservesSafeFailedCode()
    {
        var payload = Response(true, GeneratedAt, "Failed", "refresh_failed");

        Assert.Equal(CatalogRefreshState.Failed, ProtocolValidation.ValidateListOptionalInstallsResponse(payload));
        Assert.Equal("refresh_failed", payload.RefreshErrorCode);
    }

    [Fact]
    public void ValidateListResponse_RejectsUnexpectedFailedCode()
    {
        var payload = Response(true, GeneratedAt, "Failed", "internal_details");

        Assert.Throws<ProtocolValidationException>(
            () => ProtocolValidation.ValidateListOptionalInstallsResponse(payload)
        );
    }

    [Fact]
    public void ValidateListResponse_RejectsErrorCodeOutsideFailedState()
    {
        var payload = Response(true, GeneratedAt, "Idle", "refresh_failed");

        Assert.Throws<ProtocolValidationException>(
            () => ProtocolValidation.ValidateListOptionalInstallsResponse(payload)
        );
    }

    [Fact]
    public void ListResponse_MissingRequiredSnapshotMetadata_IsRejectedByDeserializer()
    {
        const string json = "{\"items\":[]}";

        Assert.Throws<JsonException>(
            () => JsonSerializer.Deserialize<ListOptionalInstallsResponse>(json, ProtocolJson.Options)
        );
    }

    [Fact]
    public void CompleteMetadata_RoundTrips()
    {
        var requestedAt = GeneratedAt.AddMinutes(-2);
        var completedAt = GeneratedAt.AddMinutes(1);
        var payload = new ListOptionalInstallsResponse(
            Items: [Item()],
            SnapshotAvailable: true,
            SnapshotGeneratedAtUtc: GeneratedAt,
            RefreshState: "Idle",
            RefreshRequestedAtUtc: requestedAt,
            RefreshCompletedAtUtc: completedAt,
            RefreshErrorCode: null
        );

        var json = JsonSerializer.Serialize(payload, ProtocolJson.Options);
        var copy = JsonSerializer.Deserialize<ListOptionalInstallsResponse>(json, ProtocolJson.Options)!;

        Assert.True(copy.SnapshotAvailable);
        Assert.Equal(GeneratedAt, copy.SnapshotGeneratedAtUtc);
        Assert.Equal("Idle", copy.RefreshState);
        Assert.Equal(requestedAt, copy.RefreshRequestedAtUtc);
        Assert.Equal(completedAt, copy.RefreshCompletedAtUtc);
        Assert.Single(copy.Items);
    }

    private static ListOptionalInstallsResponse Response(
        bool snapshotAvailable,
        DateTimeOffset? generatedAt,
        string state,
        string? errorCode = null
    ) => new(
        Items: [],
        SnapshotAvailable: snapshotAvailable,
        SnapshotGeneratedAtUtc: generatedAt,
        RefreshState: state,
        RefreshErrorCode: errorCode
    );

    private static OptionalInstallItem Item() => new(
        ItemName: "Example",
        DisplayName: "Example",
        Version: "1.0",
        Catalog: "test",
        InstallerType: "msi",
        InstallerPackageId: "Example",
        InstallerLocation: "example.msi",
        IsManaged: false,
        IsInstalled: false,
        Status: OptionalInstallStatus.NotInstalled,
        StatusUpdatedAtUtc: GeneratedAt,
        LastOperationId: null
    );
}
