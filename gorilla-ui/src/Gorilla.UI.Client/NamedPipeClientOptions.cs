namespace Gorilla.UI.Client;

public sealed record NamedPipeClientOptions(
    string PipeName,
    TimeSpan ConnectTimeout,
    TimeSpan RequestTimeout
)
{
    public static NamedPipeClientOptions Default =>
        new(PipeName: "gorilla-service", ConnectTimeout: TimeSpan.FromSeconds(5), RequestTimeout: TimeSpan.FromSeconds(30));
}
