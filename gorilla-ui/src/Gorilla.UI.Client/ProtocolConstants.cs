namespace Gorilla.UI.Client;

public static class ProtocolConstants
{
    public const string Version = "v1";

    public static class Operation
    {
        public const string ListOptionalInstalls = "ListOptionalInstalls";
        public const string InstallItem = "InstallItem";
        public const string RemoveItem = "RemoveItem";
        public const string StreamOperationStatus = "StreamOperationStatus";
    }

    public static readonly ISet<string> AllOperations = new HashSet<string>(StringComparer.Ordinal)
    {
        Operation.ListOptionalInstalls,
        Operation.InstallItem,
        Operation.RemoveItem,
        Operation.StreamOperationStatus,
    };
}
