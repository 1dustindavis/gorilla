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
            session.FocusForKeyboard(card);

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
    public void KeyboardEmbeddedActionStartsMutationWithoutOpeningDetailsAndKeepsLogicalFocus()
    {
        RunWithDiagnostics(nameof(KeyboardEmbeddedActionStartsMutationWithoutOpeningDetailsAndKeepsLogicalFocus), session =>
        {
            var home = new HomePageDriver(session);
            EnsureSlowFixtureAbsent(session, home);
            home.EnsureItemVisible(SlowFixtureItemName);

            var action = home.PrimaryActionButton(SlowFixtureItemName);
            Assert.Equal(ControlType.Button, action.ControlType);
            Assert.Equal("Install", action.Name);
            session.FocusForKeyboard(action);
            Keyboard.Type(VirtualKeyShort.SPACE);

            home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
            Assert.Null(ById(session, "AppDetailsRoot"));

            var currentAction = home.PrimaryActionButton(SlowFixtureItemName);
            Assert.False(currentAction.IsEnabled);
            var logicalCard = home.WaitForItem(SlowFixtureItemName);
            session.WaitUntil(() => HasKeyboardFocus(logicalCard));
            Assert.Equal(SlowFixtureItemName, logicalCard.AutomationId);

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
            session.FocusForKeyboard(target);
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
            home.EnsureItemVisible(SlowFixtureItemName);

            var action = home.PrimaryActionButton(SlowFixtureItemName);
            session.FocusForKeyboard(action);
            Keyboard.Type(VirtualKeyShort.ENTER);
            home.WaitForOperationContaining(SlowFixtureItemName, "Installing", TimeSpan.FromSeconds(30));
            var operationId = home.OperationId(SlowFixtureItemName);
            Assert.False(string.IsNullOrWhiteSpace(operationId));

            var activityButton = session.WaitFor(() => ById(session, "ActivityNavigationButton"));
            session.FocusForKeyboard(activityButton);
            Keyboard.Type(VirtualKeyShort.ENTER);
            _ = session.WaitFor(() => ById(session, "ActivityPageRoot"));
            var activityBack = session.WaitFor(() => ById(session, "ActivityBackButton"));
            session.WaitUntil(() => HasKeyboardFocus(activityBack));

            var activity = new ActivityPageDriver(session);
            var row = activity.WaitForOperation(operationId, TimeSpan.FromSeconds(30));
            row.Patterns.ScrollItem.PatternOrDefault?.ScrollIntoView();
            row = activity.WaitForOperation(operationId);
            session.FocusForKeyboard(row);
            Keyboard.Type(VirtualKeyShort.ENTER);
            _ = session.WaitFor(() => ById(session, "AppDetailsRoot"));

            var detailsBack = session.WaitFor(() => ById(session, "DetailsBackButton"));
            session.WaitUntil(() => HasKeyboardFocus(detailsBack));
            Keyboard.Type(VirtualKeyShort.ENTER);
            _ = session.WaitFor(() => ById(session, "ActivityPageRoot"));
            var restored = activity.WaitForOperation(operationId);
            session.WaitUntil(() => HasKeyboardFocus(restored));

            // This test starts a service-owned operation. Do not let it leak into
            // a later test merely because navigation validation completes first.
            session.WaitUntil(() => File.Exists(RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH")), TimeSpan.FromSeconds(30));
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
            home.EnsureItemVisible(SlowFixtureItemName);

            var events = new ConcurrentQueue<(string AutomationId, string Name)>();
            var handler = session.MainWindow.RegisterAutomationEvent(
                AutomationObjectIds.LiveRegionChangedEvent,
                TreeScope.Descendants,
                (element, _) => events.Enqueue((SafeAutomationId(element), SafeName(element)))
            );

            try
            {
                var action = home.PrimaryActionButton(SlowFixtureItemName);
                session.FocusForKeyboard(action);
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

            // Operation ownership belongs to the service, so closing this UI session
            // would not cancel the install. Drain it to a stable observed state before
            // yielding the shared fixture to the next test.
            session.WaitUntil(() => File.Exists(RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH")), TimeSpan.FromSeconds(30));
            home.WaitForItemStatus(SlowFixtureItemName, "Installed", TimeSpan.FromSeconds(30));
        });
    }

    private static void EnsureSlowFixtureAbsent(GorillaAppSession session, HomePageDriver home)
    {
        var markerPath = RequiredPath("GORILLA_UI_E2E_SLOW_MARKER_PATH");
        _ = home.WaitForItem(SlowFixtureItemName);

        // A previous UI session can legitimately disappear while a service-owned
        // install/remove continues. Never normalize the fixture from a transient
        // observation; first wait for any known in-flight state to settle.
        session.WaitUntil(
            () => !IsSlowFixtureOperationInFlight(home.OperationText(SlowFixtureItemName)),
            TimeSpan.FromSeconds(30)
        );

        if (string.Equals(home.ItemStatus(SlowFixtureItemName), "Installed", StringComparison.OrdinalIgnoreCase))
        {
            home.EnsureItemVisible(SlowFixtureItemName);
            home.RemoveButton(SlowFixtureItemName).Invoke();
            session.WaitUntil(() => !File.Exists(markerPath), TimeSpan.FromSeconds(30));
            session.WaitUntil(
                () => !IsSlowFixtureOperationInFlight(home.OperationText(SlowFixtureItemName)),
                TimeSpan.FromSeconds(30)
            );
        }

        home.WaitForItemStatus(SlowFixtureItemName, "Not installed", TimeSpan.FromSeconds(30));
    }

    private static bool IsSlowFixtureOperationInFlight(string operationText)
    {
        return operationText.Contains("Queued", StringComparison.OrdinalIgnoreCase) ||
            operationText.Contains("Installing", StringComparison.OrdinalIgnoreCase) ||
            operationText.Contains("Removing", StringComparison.OrdinalIgnoreCase);
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
