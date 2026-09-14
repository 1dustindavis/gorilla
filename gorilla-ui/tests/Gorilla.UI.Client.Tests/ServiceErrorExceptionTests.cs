using System.Reflection;
using System.Text.Json;
using Gorilla.UI.Client;
using Xunit;

namespace Gorilla.UI.Client.Tests;

public class ServiceErrorExceptionTests
{
    [Fact]
    public void ErrorEnvelopePreservesStructuredCodeAndMessage()
    {
        using var doc = JsonDocument.Parse(
            "{\"version\":\"v1\",\"messageType\":\"Error\",\"operation\":\"InstallItem\",\"requestId\":\"req-1\",\"operationId\":\"\",\"timestampUtc\":\"2026-09-14T00:00:00Z\",\"payload\":{\"errorCode\":\"already_selected\",\"errorMessage\":\"Current service truth rejects Install.\"}}"
        );

        var method = typeof(NamedPipeGorillaServiceClient).GetMethod(
            "HandleErrorEnvelopeIfPresent",
            BindingFlags.NonPublic | BindingFlags.Static
        );
        Assert.NotNull(method);

        var invocation = Assert.Throws<TargetInvocationException>(() => method!.Invoke(null, new object[] { doc }));
        var serviceError = Assert.IsType<ServiceErrorException>(invocation.InnerException);
        Assert.Equal("already_selected", serviceError.ErrorCode);
        Assert.Equal("Current service truth rejects Install.", serviceError.ErrorMessage);
        Assert.Contains("already_selected", serviceError.Message, StringComparison.Ordinal);
    }
}
