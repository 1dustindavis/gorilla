using System.Collections.Concurrent;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Definitions;
using FlaUI.Core.Input;
using FlaUI.Core.WindowsAPI;
using FlaUI.UIA3.Identifiers;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class AccessibilityAutomationTests
{
    private const string BasicFixtureItemName = "Ps1V1";
    private const string SlowFixtureItemName = "SlowInstallFixture";

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void KeyboardCardActivationAndBackRestoreLogicalCardFocus()
    {
        RunWithDiagnostics(nameof(KeyboardCardActivationAndBackRestoreLogicalCardFocus), session =>
        {
            var home = new HomePageDriver(session);
            var card = home.WaitForItem(BasicFixtureItemName);
            card.Patterns.ScrollItem.PatternOrDefault?.ScrollIntoView();
            card = home.WaitForItem(BasicFixtureItemName);
            card.Focus();

            Keyboard.Type(VirtualKeyShort.ENTER);
            _ = session.WaitFor(() => ById(session, "AppDetailsRoot"));

            var detailsBack = session.WaitFor(() => ById(session, "DetailsBackButton"));
            session.WaitUntil(() => HasKeyboardFocus(detailsBack));
            Keyboard.Type(VirtualKeyShort.ENTER);

            _ = session.WaitFor(() => ById(session, "HomeHeading"));
            var restored = home.WaitForItem(BasicFixtureItemName);
            session.WaitUntil(() => HasKeyboardFocus(restored));
            Assert.Equal(BasicFixtureItemName, restored.AutomationId);
            session.CaptureCheckpoint("stage7-keyboard-card-back-focus", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void KeyboardEmbeddedActionStartsMutationWithoutOpeningDetailsAndKeepsFocus()
    {
        RunWithDiagnostics(nameof(KeyboardEmbeddedActionStartsMutationWithoutOpeningDetailsAndKeepsFocus), session =>
        {
            var home = new HomePageDriver(session);
            EnsureSlowFixtureAbsent(session, home);

            var action = home.PrimaryActionButton(SlowFixtureItemName);
            Assert.Equal(ControlType.Button, action.ControlType);
            Assert.Equal("Install", action.Name);
            action.Focus();
            Keyboard.Type(VirtualKeyShort.SPACE);

            home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
            Assert.Null(ById(session, "AppDetailsRoot"));

            var currentAction = home.PrimaryActionButton(SlowFixtureItemName);
            Assert.False(currentAction.IsEnabled);
            session.WaitUntil(() => HasKeyboardFocus(currentAction));
            Assert.Equal("PrimaryActionButton", currentAction.AutomationId);

            session.WaitUntil(() => File.Exists(RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH")), TimeSpan.FromSeconds(30));
            home.WaitForItemStatus(SlowFixtureItemName, "Installed", TimeSpan.FromSeconds(30));
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void KeyboardActivationAfterScrollTargetsCurrentItemIdentity()
    {
        RunWithDiagnostics(nameof(KeyboardActivationAfterScrollTargetsCurrentItemIdentity), session =>
        {
            var home = new HomePageDriver(session);
            _ = home.WaitForItem(BasicFixtureItemName);
            var target = home.WaitForItem(SlowFixtureItemName);

            target.Patterns.ScrollItem.PatternOrDefault?.ScrollIntoView();
            target = home.WaitForItem(SlowFixtureItemName);
            target.Focus();
            Assert.Equal(SlowFixtureItemName, target.AutomationId);

            Keyboard.Type(VirtualKeyShort.ENTER);
            _ = session.WaitFor(() => ById(session, "AppDetailsRoot"));
            var displayName = session.WaitFor(() => ById(session, "DetailsDisplayName"));
            Assert.Contains("Slow Install Fixture", displayName.Name, StringComparison.OrdinalIgnoreCase);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void ActivityAndDetailsNavigationRemainKeyboardReachableAndRestoreOperationFocus()
    {
        RunWithDiagnostics(nameof(ActivityAndDetailsNavigationRemainKeyboardReachableAndRestoreOperationFocus), session =>
        {
            var home = new HomePageDriver(session);
            EnsureSlowFixtureAbsent(session, home);
            var action = home.PrimaryActionButton(SlowFixtureItemName);
            action.Focus();
            Keyboard.Type(VirtualKeyShort.ENTER);
            home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
            var operationId = home.OperationId(SlowFixtureItemName);
            Assert.False(string.IsNullOrWhiteSpace(operationId));

            var activityButton = session.WaitFor(() => ById(session, "ActivityNavigationButton"));
            activityButton.Focus();
            Keyboard.Type(VirtualKeyShort.ENTER);
            _ = session.WaitFor(() => ById(session, "ActivityPageRoot"));
            var activityBack = session.WaitFor(() => ById(session, "ActivityBackButton"));
            session.WaitUntil(() => HasKeyboardFocus(activityBack));

            var activity = new ActivityPageDriver(session);
            var row = activity.WaitForOperation(operationId, TimeSpan.FromSeconds(30));
            row.Patterns.ScrollItem.PatternOrDefault?.ScrollIntoView();
            row = activity.WaitForOperation(operationId);
            row.Focus();
            Keyboard.Type(VirtualKeyShort.ENTER);
            _ = session.WaitFor(() => ById(session, "AppDetailsRoot"));

            var detailsBack = session.WaitFor(() => ById(session, "DetailsBackButton"));
            session.WaitUntil(() => HasKeyboardFocus(detailsBack));
            Keyboard.Type(VirtualKeyShort.ENTER);
            _ = session.WaitFor(() => ById(session, "ActivityPageRoot"));
            var restored = activity.WaitForOperation(operationId);
            session.WaitUntil(() => HasKeyboardFocus(restored));
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void PrincipalAutomationSurfaceExposesStableNamesRolesAndEnabledState()
    {
        RunWithDiagnostics(nameof(PrincipalAutomationSurfaceExposesStableNamesRolesAndEnabledState), session =>
        {
            var home = new HomePageDriver(session);
            var item = home.WaitForItem(BasicFixtureItemName);
            Assert.Equal(BasicFixtureItemName, item.AutomationId);
            Assert.Equal(ControlType.ListItem, item.ControlType);
            Assert.False(string.IsNullOrWhiteSpace(item.Name));

            var primary = home.PrimaryActionButton(BasicFixtureItemName);
            Assert.Equal("PrimaryActionButton", primary.AutomationId);
            Assert.Equal(ControlType.Button, primary.ControlType);
            Assert.False(string.IsNullOrWhiteSpace(primary.Name));

            var activity = session.WaitFor(() => ById(session, "ActivityNavigationButton"));
            Assert.Equal("Activity", activity.Name);
            var refresh = session.WaitFor(() => ById(session, "CatalogRefreshButton"));
            Assert.Equal("Refresh App Catalog", refresh.Name);

            session.CaptureCheckpoint("stage7-accessible-automation-contract", includeAutomationTree: true);
        });
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void OperationStartRaisesLiveRegionEventFromStatusElementWithoutParentDuplicate()
    {
        RunWithDiagnostics(nameof(OperationStartRaisesLiveRegionEventFromStatusElementWithoutParentDuplicate), session =>
        {
            var home = new HomePageDriver(session);
            EnsureSlowFixtureAbsent(session, home);
            var events = new ConcurrentQueue<(string AutomationId, string Name)>();
            var handler = session.MainWindow.RegisterAutomationEvent(
                AutomationObjectIds.LiveRegionChangedEvent,
                TreeScope.Descendants,
                (element, _) => events.Enqueue((SafeAutomationId(element), SafeName(element)))
            );

            try
            {
                var action = home.PrimaryActionButton(SlowFixtureItemName);
                action.Focus();
                Keyboard.Type(VirtualKeyShort.ENTER);
                home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
                session.WaitUntil(
                    () => events.Any(entry =>
                        entry.AutomationId == "CatalogOperationStatus" &&
                        entry.Name.Contains("Installing", StringComparison.OrdinalIgnoreCase)),
                    TimeSpan.FromSeconds(30)
                );

                Assert.DoesNotContain(events, entry => entry.AutomationId == "CatalogOperation");
            }
            finally
            {
                session.MainWindow.FrameworkAutomationElement.UnregisterAutomationEventHandler(handler);
            }
        });
    }

    private static void EnsureSlowFixtureAbsent(GorillaAppSession session, HomePageDriver home)
    {
        var markerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
        _ = home.WaitForItem(SlowFixtureItemName);
        if (string.Equals(home.ItemStatus(SlowFixtureItemName), "Installed", StringComparison.OrdinalIgnoreCase))
        {
            home.RemoveButton(SlowFixtureItemName).Invoke();
            session.WaitUntil(() => !File.Exists(markerPath), TimeSpan.FromSeconds(30));
        }
        home.WaitForItemStatus(SlowFixtureItemName, "Not installed", TimeSpan.FromSeconds(30));
    }

    private static AutomationElement? ById(GorillaAppSession session, string automationId)
        => session.MainWindow.FindFirstDescendant(cf => cf.ByAutomationId(automationId));

    private static bool HasKeyboardFocus(AutomationElement element)
        => element.Properties.HasKeyboardFocus.ValueOrDefault;

    private static string SafeAutomationId(AutomationElement element)
    {
        try
        {
            return element.AutomationId;
        }
        catch
        {
            return string.Empty;
        }
    }

    private static string SafeName(AutomationElement element)
    {
        try
        {
            return element.Name;
        }
        catch
        {
            return string.Empty;
        }
    }

    private static string RequiredPath(string variableName)
    {
        var value = Environment.GetEnvironmentVariable(variableName);
        if (string.IsNullOrWhiteSpace(value))
        {
            throw new InvalidOperationException($"{variableName} must be set by the E2E harness.");
        }
        return value;
    }

    private static void RunWithDiagnostics(string testName, Action<GorillaAppSession> test)
    {
        using var session = GorillaAppSession.Launch();
        try
        {
            test(session);
        }
        catch (Exception ex)
        {
            session.CaptureFailure(ex, testName);
            throw;
        }
    }
}
