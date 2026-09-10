using System.Reflection;
using System.Text.Json;
using Gorilla.UI.Client;
using Xunit;

namespace Gorilla.UI.Client.Tests;

public class MutationProtocolTests
{
    [Fact]
    public void InstallItemRequest_SerializesStableMutationIdentity()
    {
        var json = JsonSerializer.Serialize(
            new InstallItemRequest("Example", "mutation-1"),
            ProtocolJson.Options
        );
        using var doc = JsonDocument.Parse(json);

        Assert.Equal("Example", doc.RootElement.GetProperty("itemName").GetString());
        Assert.Equal("mutation-1", doc.RootElement.GetProperty("mutationId").GetString());
    }

    [Fact]
    public void RemoveItemRequest_SerializesStableMutationIdentity()
    {
        var json = JsonSerializer.Serialize(
            new RemoveItemRequest("Example", "mutation-2"),
            ProtocolJson.Options
        );
        using var doc = JsonDocument.Parse(json);

        Assert.Equal("Example", doc.RootElement.GetProperty("itemName").GetString());
        Assert.Equal("mutation-2", doc.RootElement.GetProperty("mutationId").GetString());
    }

    [Theory]
    [InlineData(typeof(IOException), true)]
    [InlineData(typeof(TimeoutException), true)]
    [InlineData(typeof(OperationCanceledException), true)]
    [InlineData(typeof(JsonException), true)]
    [InlineData(typeof(InvalidOperationException), false)]
    public void MutationRetry_OnlyTreatsTransportUncertaintyAsRetryable(Type exceptionType, bool expected)
    {
        var method = typeof(NamedPipeGorillaServiceClient).GetMethod(
            "IsUncertainMutationAcknowledgement",
            BindingFlags.NonPublic | BindingFlags.Static
        );
        Assert.NotNull(method);

        var exception = (Exception)Activator.CreateInstance(exceptionType)!;
        using var cts = new CancellationTokenSource();
        var actual = (bool)method!.Invoke(null, new object[] { exception, cts.Token })!;

        Assert.Equal(expected, actual);
    }

    [Fact]
    public void MutationRetry_DoesNotRetryCallerCancellation()
    {
        var method = typeof(NamedPipeGorillaServiceClient).GetMethod(
            "IsUncertainMutationAcknowledgement",
            BindingFlags.NonPublic | BindingFlags.Static
        );
        Assert.NotNull(method);

        using var cts = new CancellationTokenSource();
        cts.Cancel();
        var actual = (bool)method!.Invoke(
            null,
            new object[] { new OperationCanceledException(), cts.Token }
        )!;

        Assert.False(actual);
    }
}
