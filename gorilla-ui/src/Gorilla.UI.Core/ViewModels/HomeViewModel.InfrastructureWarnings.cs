using Gorilla.UI.Core.Models;
using AppCatalog = Gorilla.UI.Client.AppCatalog;

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

    public void ClearCatalogRecoveryInfrastructureWarning()
    {
        var context = InfrastructureWarning.Context;
        if (string.Equals(context, "Unexpected catalog-page initialization failure", StringComparison.Ordinal) ||
            string.Equals(context, "Unexpected App Details initialization failure", StringComparison.Ordinal))
        {
            ClearInfrastructureWarning();
        }
    }

    public void ClearActionStartInfrastructureWarning(
        AppCatalog.Action action,
        string itemName
    )
    {
        var warning = InfrastructureWarning;
        if (!string.Equals(warning.ItemName, itemName, StringComparison.OrdinalIgnoreCase) ||
            !string.Equals(warning.ExpectedAction, action.ToString(), StringComparison.Ordinal))
        {
            return;
        }

        if (warning.Context.Contains("action-start failure", StringComparison.OrdinalIgnoreCase) ||
            string.Equals(warning.Context, "Activity Retry start failure", StringComparison.Ordinal))
        {
            ClearInfrastructureWarning();
        }
    }
}
