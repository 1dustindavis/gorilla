namespace Gorilla.UI.Client;

public sealed class ServiceErrorException : InvalidOperationException
{
    public ServiceErrorException(string errorCode, string errorMessage)
        : base($"{errorCode}: {errorMessage}")
    {
        ErrorCode = errorCode;
        ErrorMessage = errorMessage;
    }

    public string ErrorCode { get; }
    public string ErrorMessage { get; }
}
