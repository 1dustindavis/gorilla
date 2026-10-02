using Gorilla.UI.Client.AppCatalog;

namespace Gorilla.UI.Core.Models;

internal static class CatalogObservationPresentation
{
    public static string StatusText(UiOptionalInstallItem item)
    {
        if (item.ObservedState == ObservedState.Unknown)
        {
            return item.Observation.InstallRequirement switch
            {
                RequirementState.Satisfied => "Installed",
                RequirementState.NotSatisfied => "Install or update needed",
                _ => "Status unavailable",
            };
        }

        return item.ObservedState switch
        {
            ObservedState.Absent => "Not installed",
            ObservedState.Installed => "Installed",
            ObservedState.UpdateAvailable => "Update available",
            ObservedState.DetectionFailed => "Unable to determine status",
            _ => "Status unavailable",
        };
    }

    public static bool PresentsAsInstalled(UiOptionalInstallItem item)
        => item.ObservedState == ObservedState.Installed ||
            (item.ObservedState == ObservedState.Unknown &&
             item.Observation.InstallRequirement == RequirementState.Satisfied);
}
