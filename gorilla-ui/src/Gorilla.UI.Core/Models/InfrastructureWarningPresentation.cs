using System.Text;

namespace Gorilla.UI.Core.Models;

public sealed record InfrastructureWarningPresentation(
    string Message,
    string TechnicalDetails,
    string Context = "",
    string? OperationId = null,
    string? ItemName = null,
    string? ExpectedAction = null
)
{
    public static InfrastructureWarningPresentation None { get; } = new(string.Empty, string.Empty);

    public bool HasTechnicalDetails => !string.IsNullOrWhiteSpace(TechnicalDetails);

    public static InfrastructureWarningPresentation Create(
        string message,
        string context,
        Exception? exception = null,
        string? operationId = null,
        string? itemName = null,
        string? expectedAction = null,
        string? additionalTechnicalDetails = null
    )
    {
        ArgumentException.ThrowIfNullOrWhiteSpace(message);
        ArgumentException.ThrowIfNullOrWhiteSpace(context);

        var details = new StringBuilder();
        details.Append("Context: ").Append(context);

        if (!string.IsNullOrWhiteSpace(operationId))
        {
            details.AppendLine().Append("Operation ID: ").Append(operationId);
        }
        if (!string.IsNullOrWhiteSpace(itemName))
        {
            details.AppendLine().Append("Item: ").Append(itemName);
        }
        if (!string.IsNullOrWhiteSpace(expectedAction))
        {
            details.AppendLine().Append("Expected action: ").Append(expectedAction);
        }
        if (exception is not null)
        {
            details.AppendLine().Append("Exception type: ").Append(exception.GetType().FullName);
            details.AppendLine().Append("Exception message: ").Append(exception.Message);
        }
        if (!string.IsNullOrWhiteSpace(additionalTechnicalDetails))
        {
            details.AppendLine().Append(additionalTechnicalDetails.Trim());
        }

        return new InfrastructureWarningPresentation(
            message,
            details.ToString(),
            context,
            operationId,
            itemName,
            expectedAction
        );
    }
}
