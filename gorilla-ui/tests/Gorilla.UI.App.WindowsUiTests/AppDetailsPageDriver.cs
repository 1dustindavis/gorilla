using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using FlaUI.Core.Exceptions;

namespace Gorilla.UI.App.WindowsUiTests;

internal sealed class AppDetailsPageDriver
{
    private readonly GorillaAppSession _session;

    public AppDetailsPageDriver(GorillaAppSession session)
    {
        _session = session;
    }

    public AutomationElement Root => WaitById("AppDetailsRoot");
    public AutomationElement DisplayName => WaitById("DetailsDisplayName");
    public AutomationElement Observation => WaitById("DetailsObservation");
    public Button BackButton => WaitById("DetailsBackButton").AsButton();
    public Button PrimaryAction => WaitById("DetailsPrimaryAction").AsButton();

    public string Name => SafeName(DisplayName);
    public string ObservationText => SafeName(Observation);
    public string Description => OptionalText("DetailsDescription");
    public string AvailableVersion => OptionalText("DetailsAvailableVersion");
    public string InstalledVersion => OptionalText("DetailsInstalledVersion");
    public string ActiveOperation => OptionalText("DetailsActiveOperation");
    public string LatestResult => OptionalText("DetailsLatestResult");
    public string ActionExplanation => OptionalText("DetailsActionExplanation");

    public Button SecondaryAction => WaitById("DetailsSecondaryAction").AsButton();

    public void GoBack() => BackButton.Invoke();

    public void WaitForObservation(string expected, TimeSpan? timeout = null)
        => _session.WaitUntil(
            () => ObservationText.Contains(expected, StringComparison.OrdinalIgnoreCase),
            timeout
        );

    public void WaitForActiveOperation(string expected, TimeSpan? timeout = null)
        => _session.WaitUntil(
            () => ActiveOperation.Contains(expected, StringComparison.OrdinalIgnoreCase),
            timeout
        );

    public void WaitForLatestResult(string expected, TimeSpan? timeout = null)
        => _session.WaitUntil(
            () => LatestResult.Contains(expected, StringComparison.OrdinalIgnoreCase),
            timeout
        );

    private AutomationElement WaitById(string automationId)
        => _session.WaitFor(() => _session.MainWindow.FindFirstDescendant(cf => cf.ByAutomationId(automationId)));

    private string OptionalText(string automationId)
    {
        var element = _session.MainWindow.FindFirstDescendant(cf => cf.ByAutomationId(automationId));
        if (element is null)
        {
            return string.Empty;
        }

        var texts = element.FindAllDescendants(cf => cf.ByControlType(ControlType.Text));
        if (texts.Length == 0)
        {
            return SafeName(element);
        }
        return string.Join(" ", texts.Select(SafeName).Where(value => !string.IsNullOrWhiteSpace(value)));
    }

    private static string SafeName(AutomationElement element)
    {
        try
        {
            return element.Name;
        }
        catch (PropertyNotSupportedException)
        {
            return string.Empty;
        }
    }
}
