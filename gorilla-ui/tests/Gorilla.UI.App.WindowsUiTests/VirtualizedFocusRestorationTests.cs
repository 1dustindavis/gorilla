using System.Text.RegularExpressions;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Input;
using FlaUI.Core.WindowsAPI;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class VirtualizedFocusRestorationTests
{
    private const string BasicFixtureItemName = "Ps1V1";
    private const string FailureFixtureItemName = "Ps1Failure";
    private const int CatalogExpansionCount = 30;
    private const int ActivityOperationCount = 10;

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DeepVirtualizedCatalogItemRestoresFocusByItemName()
    {
        var cachePath = RequiredPath("GORILLA_UI_E2E_CACHE_PATH");
        var workRoot = Directory.GetParent(Directory.GetParent(cachePath)!.FullName)!.FullName;
        var catalogPath = Path.Combine(workRoot, "fixture", "repo", "catalogs", "integration.yaml");
        var manifestPath = Path.Combine(workRoot, "fixture", "repo", "manifests", "ui-e2e.yaml");
        var originalCatalog = File.ReadAllText(catalogPath);
        var originalManifest = File.ReadAllText(manifestPath);
        var targetItemName = $"ZZVirtualizationFixture{CatalogExpansionCount:00}";

        try
        {
            ExpandCatalogForVirtualization(
                catalogPath,
                manifestPath,
                originalCatalog,
                originalManifest
            );

            RunWithDiagnostics(nameof(DeepVirtualizedCatalogItemRestoresFocusByItemName), session =>
            {
                var home = new HomePageDriver(session);
                var refresh = ById(session, "CatalogRefreshButton")
                    ?? throw new InvalidOperationException("Catalog refresh button was not found.");
                refresh.AsButton().Invoke();

                // Prove the service has consumed the temporary catalog without using
                // the deep full-list container as our navigation mechanism.
                home.Search($"ZZ Virtualization Fixture {CatalogExpansionCount:00}");
                _ = home.WaitForItem(targetItemName, TimeSpan.FromSeconds(30));
                home.Search(string.Empty);

                var first = home.WaitForItem(BasicFixtureItemName, TimeSpan.FromSeconds(30));
                first.Patterns.ScrollItem.PatternOrDefault?.ScrollIntoView();
                first = home.WaitForItem(BasicFixtureItemName);

                // With 30 extra cards, the last logical item must not already have a
                // realized UIA container when the viewport is back at the beginning.
                session.WaitUntil(
                    () => home.CatalogItems.FindFirstDescendant(cf => cf.ByAutomationId(targetItemName)) is null,
                    TimeSpan.FromSeconds(5)
                );

                session.FocusForKeyboard(first);
                Keyboard.Type(VirtualKeyShort.END);
                var deepCard = session.WaitFor(() =>
                {
                    var focused = session.FocusedElement();
                    return string.Equals(focused.AutomationId, targetItemName, StringComparison.Ordinal)
                        ? focused
                        : null;
                }, TimeSpan.FromSeconds(10));

                Keyboard.Type(VirtualKeyShort.ENTER);
                _ = session.WaitFor(() => ById(session, "AppDetailsRoot"));
                var detailsBack = session.WaitFor(() => ById(session, "DetailsBackButton"));
                session.WaitUntil(() => detailsBack.Properties.HasKeyboardFocus.ValueOrDefault);
                Keyboard.Type(VirtualKeyShort.ENTER);

                _ = session.WaitFor(() => ById(session, "HomeHeading"));
                session.WaitUntil(
                    () => string.Equals(
                        session.FocusedElement().AutomationId,
                        targetItemName,
                        StringComparison.Ordinal
                    ),
                    TimeSpan.FromSeconds(10)
                );
                Assert.Equal(targetItemName, session.FocusedElement().AutomationId);
            });
        }
        finally
        {
            File.WriteAllText(catalogPath, originalCatalog);
            File.WriteAllText(manifestPath, originalManifest);
        }
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void DeepVirtualizedActivityOperationRestoresFocusByOperationId()
    {
        RunWithDiagnostics(nameof(DeepVirtualizedActivityOperationRestoresFocusByOperationId), session =>
        {
            var home = new HomePageDriver(session);
            var createdOperationIds = new List<string>();
            string? previousOperationId = null;

            for (var i = 0; i < ActivityOperationCount; i++)
            {
                home.EnsureItemVisible(FailureFixtureItemName);
                var action = home.PrimaryActionButton(FailureFixtureItemName);
                session.WaitUntil(() => action.IsEnabled, TimeSpan.FromSeconds(30));
                action.Invoke();

                var operationId = session.WaitFor(() =>
                {
                    var candidate = home.OperationId(FailureFixtureItemName);
                    if (string.IsNullOrWhiteSpace(candidate) ||
                        string.Equals(candidate, previousOperationId, StringComparison.Ordinal))
                    {
                        return null;
                    }

                    return home.OperationText(FailureFixtureItemName)
                        .Contains("failed", StringComparison.OrdinalIgnoreCase)
                            ? candidate
                            : null;
                }, TimeSpan.FromSeconds(60));

                createdOperationIds.Add(operationId);
                previousOperationId = operationId;
            }

            var activityButton = session.WaitFor(() => ById(session, "ActivityNavigationButton"));
            session.FocusForKeyboard(activityButton);
            Keyboard.Type(VirtualKeyShort.ENTER);
            _ = session.WaitFor(() => ById(session, "ActivityPageRoot"));

            var activity = new ActivityPageDriver(session);
            var newestId = createdOperationIds[^1];
            var targetId = createdOperationIds[0];
            var newest = activity.WaitForOperation(newestId, TimeSpan.FromSeconds(30));
            session.FocusForKeyboard(newest);

            // Walk through the ten newly retained rows using real keyboard ListView
            // navigation. This necessarily scrolls/recycles the viewport on the compact
            // CI window before reaching our oldest generated operation.
            for (var i = 1; i < ActivityOperationCount; i++)
            {
                Keyboard.Type(VirtualKeyShort.DOWN);
            }

            var targetAutomationId = $"ActivityOperation-{targetId}";
            session.WaitUntil(
                () => string.Equals(
                    session.FocusedElement().AutomationId,
                    targetAutomationId,
                    StringComparison.Ordinal
                ),
                TimeSpan.FromSeconds(10)
            );

            Keyboard.Type(VirtualKeyShort.ENTER);
            _ = session.WaitFor(() => ById(session, "AppDetailsRoot"));
            var detailsBack = session.WaitFor(() => ById(session, "DetailsBackButton"));
            session.WaitUntil(() => detailsBack.Properties.HasKeyboardFocus.ValueOrDefault);
            Keyboard.Type(VirtualKeyShort.ENTER);

            _ = session.WaitFor(() => ById(session, "ActivityPageRoot"));
            session.WaitUntil(
                () => string.Equals(
                    session.FocusedElement().AutomationId,
                    targetAutomationId,
                    StringComparison.Ordinal
                ),
                TimeSpan.FromSeconds(10)
            );
            Assert.Equal(targetAutomationId, session.FocusedElement().AutomationId);
        });
    }

    private static void ExpandCatalogForVirtualization(
        string catalogPath,
        string manifestPath,
        string originalCatalog,
        string originalManifest
    )
    {
        var templateMatch = Regex.Match(
            originalCatalog,
            @"(?ms)^Ps1V1:\r?\n.*?(?=^[A-Za-z0-9_-]+:\r?$|\z)"
        );
        if (!templateMatch.Success)
        {
            throw new InvalidOperationException("Unable to locate Ps1V1 catalog template.");
        }

        var catalog = originalCatalog.TrimEnd();
        var manifest = originalManifest.TrimEnd();
        for (var i = 1; i <= CatalogExpansionCount; i++)
        {
            var itemName = $"ZZVirtualizationFixture{i:00}";
            var displayName = $"ZZ Virtualization Fixture {i:00}";
            var entry = templateMatch.Value
                .Replace("Ps1V1:", $"{itemName}:", StringComparison.Ordinal)
                .Replace("display_name: Ps1V1", $"display_name: {displayName}", StringComparison.Ordinal);
            catalog += Environment.NewLine + Environment.NewLine + entry.TrimEnd();
            manifest += Environment.NewLine + $"  - {itemName}";
        }

        File.WriteAllText(catalogPath, catalog);
        File.WriteAllText(manifestPath, manifest);
    }

    private static AutomationElement? ById(GorillaAppSession session, string automationId)
        => session.MainWindow.FindFirstDescendant(cf => cf.ByAutomationId(automationId));

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
