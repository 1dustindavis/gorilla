using Gorilla.UI.Core.Models;

namespace Gorilla.UI.Core.ViewModels;

public sealed partial class HomeViewModel
{
    public void ReportInfrastructureWarning(
        string message,
        string context,
        Exception exception,
        string? operationId = null,
        string? itemName = null,
        string? expectedAction = null
    )
    {
        SetInfrastructureWarning(
            message,
            context,
            exception,
            operationId,
            itemName,
            expectedAction
        );
    }
}
