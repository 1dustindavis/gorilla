using System.Text.Json;
using System.Text.RegularExpressions;
using FlaUI.Core.AutomationElements;
using FlaUI.Core.Input;
using FlaUI.Core.WindowsAPI;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class IconPresentationTests
{
    private const string IconItemName = "Ps1V1";
    private const string FallbackItemName = "Ps1Failure";
    private const int VirtualizationCount = 30;

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void CatalogAndDetailsUseCustomIconWithFallbackAndRuntimeFailureRecovery()
    {
        var fixtureRoot = RequiredPath("GORILLA_UI_E2E_FIXTURE_ROOT");
        var cachePath = RequiredPath("GORILLA_UI_E2E_CACHE_PATH");
        var catalogPath = Path.Combine(fixtureRoot, "catalogs", "integration.yaml");
        var iconDirectory = Path.Combine(fixtureRoot, "icons");
        var servedIconPath = Path.Combine(iconDirectory, "catalog-icon-test.png");
        var sourceIconPath = Path.Combine(AppContext.BaseDirectory, "catalog-icon-test.png");
        var originalCatalog = File.ReadAllText(catalogPath);

        Directory.CreateDirectory(iconDirectory);
        File.Copy(sourceIconPath, servedIconPath, overwrite: true);
        File.WriteAllText(catalogPath, AddIconMetadata(originalCatalog, IconItemName));

        try
        {
            RunWithDiagnostics(nameof(CatalogAndDetailsUseCustomIconWithFallbackAndRuntimeFailureRecovery), session =>
            {
                var home = new HomePageDriver(session);
                var shell = new CatalogShellDriver(session);

                shell.Refresh();
                shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));

                var customItem = home.WaitForItem(IconItemName);
                session.FocusForKeyboard(customItem);
                Assert.True(customItem.Properties.HasKeyboardFocus.ValueOrDefault);
                Assert.Equal("Custom", WaitForIconState(session, home.WaitForCard(IconItemName), "CatalogIcon"));

                var fallbackItem = home.WaitForItem(FallbackItemName);
                session.FocusForKeyboard(fallbackItem);
                Assert.True(fallbackItem.Properties.HasKeyboardFocus.ValueOrDefault);
                Assert.Equal("Fallback", WaitForIconState(session, home.WaitForCard(FallbackItemName), "CatalogIcon"));
                Assert.Equal("Ps1V1", customItem.Name);
                Assert.Equal("Ps1Failure", fallbackItem.Name);
                session.CaptureCheckpoint("catalog-icons-mixed", includeAutomationTree: true);

                home.OpenDetails(IconItemName);
                Assert.Equal("Custom", WaitForIconState(session, session.MainWindow, "DetailsIcon"));
                session.CaptureCheckpoint("details-custom-icon", includeAutomationTree: true);

                new AppDetailsPageDriver(session).GoBack();
                home.OpenDetails(FallbackItemName);
                Assert.Equal("Fallback", WaitForIconState(session, session.MainWindow, "DetailsIcon"));

                new AppDetailsPageDriver(session).GoBack();
                var resolvedIconPath = WaitForCachedIconPath(cachePath, IconItemName);
                File.Delete(resolvedIconPath);

                home.OpenDetails(IconItemName);
                Assert.Equal("Fallback", WaitForIconState(session, session.MainWindow, "DetailsIcon"));
                session.CaptureCheckpoint("details-missing-icon-fallback", includeAutomationTree: true);

                new AppDetailsPageDriver(session).GoBack();
                File.WriteAllText(catalogPath, originalCatalog);
                shell.Refresh();
                shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));

                Assert.Equal("Fallback", WaitForIconState(session, home.WaitForCard(IconItemName), "CatalogIcon"));
            });
        }
        finally
        {
            File.WriteAllText(catalogPath, originalCatalog);
            File.Delete(servedIconPath);
        }
    }

    [Fact]
    [Trait("E2EPhase", "Healthy")]
    public void VirtualizedCardsDoNotLeakCustomIconStateAcrossRecycledContainers()
    {
        var fixtureRoot = RequiredPath("GORILLA_UI_E2E_FIXTURE_ROOT");
        var catalogPath = Path.Combine(fixtureRoot, "catalogs", "integration.yaml");
        var manifestPath = Path.Combine(fixtureRoot, "manifests", "ui-e2e.yaml");
        var iconDirectory = Path.Combine(fixtureRoot, "icons");
        var servedIconPath = Path.Combine(iconDirectory, "catalog-icon-test.png");
        var sourceIconPath = Path.Combine(AppContext.BaseDirectory, "catalog-icon-test.png");
        var originalCatalog = File.ReadAllText(catalogPath);
        var originalManifest = File.ReadAllText(manifestPath);
        var fallbackTarget = $"ZZIconVirtualization{VirtualizationCount - 1:00}";
        var customTarget = $"ZZIconVirtualization{VirtualizationCount:00}";

        Directory.CreateDirectory(iconDirectory);
        File.Copy(sourceIconPath, servedIconPath, overwrite: true);

        try
        {
            ExpandCatalogForIconVirtualization(
                catalogPath,
                manifestPath,
                originalCatalog,
                originalManifest,
                customTarget
            );

            RunWithDiagnostics(nameof(VirtualizedCardsDoNotLeakCustomIconStateAcrossRecycledContainers), session =>
            {
                var home = new HomePageDriver(session);
                var shell = new CatalogShellDriver(session);
                shell.Refresh();
                shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));

                var first = home.WaitForItem(IconItemName);
                first.Patterns.ScrollItem.PatternOrDefault?.ScrollIntoView();
                session.FocusForKeyboard(first);
                Assert.Equal("Custom", WaitForIconState(session, home.WaitForCard(IconItemName), "CatalogIcon"));

                Keyboard.Type(VirtualKeyShort.END);
                session.WaitUntil(
                    () => string.Equals(session.FocusedElement().AutomationId, customTarget, StringComparison.Ordinal),
                    TimeSpan.FromSeconds(10)
                );
                Assert.Equal("Custom", WaitForIconState(session, home.WaitForCard(customTarget), "CatalogIcon"));

                Keyboard.Type(VirtualKeyShort.UP);
                session.WaitUntil(
                    () => string.Equals(session.FocusedElement().AutomationId, fallbackTarget, StringComparison.Ordinal),
                    TimeSpan.FromSeconds(10)
                );
                Assert.Equal("Fallback", WaitForIconState(session, home.WaitForCard(fallbackTarget), "CatalogIcon"));

                Keyboard.Type(VirtualKeyShort.DOWN);
                session.WaitUntil(
                    () => string.Equals(session.FocusedElement().AutomationId, customTarget, StringComparison.Ordinal),
                    TimeSpan.FromSeconds(10)
                );
                Assert.Equal("Custom", WaitForIconState(session, home.WaitForCard(customTarget), "CatalogIcon"));
                session.CaptureCheckpoint("catalog-icons-virtualized", includeAutomationTree: true);
            });
        }
        finally
        {
            File.WriteAllText(catalogPath, originalCatalog);
            File.WriteAllText(manifestPath, originalManifest);
            File.Delete(servedIconPath);
        }
    }

    private static string AddIconMetadata(string catalog, string itemName)
    {
        var entry = new Regex(
            $@"(?m)^{Regex.Escape(itemName)}:\r?$",
            RegexOptions.CultureInvariant
        );
        var updated = entry.Replace(
            catalog,
            itemName + ":" + Environment.NewLine + "  icon: icons/catalog-icon-test.png",
            count: 1
        );
        if (string.Equals(updated, catalog, StringComparison.Ordinal))
        {
            throw new InvalidOperationException($"Unable to add icon metadata to {itemName} fixture.");
        }
        return updated;
    }

    private static void ExpandCatalogForIconVirtualization(
        string catalogPath,
        string manifestPath,
        string originalCatalog,
        string originalManifest,
        string customTarget)
    {
        var templateMatch = Regex.Match(
            originalCatalog,
            @"(?ms)^Ps1V1:\r?\n.*?(?=^[A-Za-z0-9_-]+:\r?$|\z)"
        );
        if (!templateMatch.Success)
        {
            throw new InvalidOperationException("Unable to locate Ps1V1 catalog template.");
        }

        var catalog = AddIconMetadata(originalCatalog, IconItemName).TrimEnd();
        var manifest = originalManifest.TrimEnd();
        for (var i = 1; i <= VirtualizationCount; i++)
        {
            var itemName = $"ZZIconVirtualization{i:00}";
            var entry = templateMatch.Value
                .Replace("Ps1V1:", $"{itemName}:", StringComparison.Ordinal)
                .Replace("display_name: Ps1V1", $"display_name: ZZ Icon Virtualization {i:00}", StringComparison.Ordinal)
                .TrimEnd();
            if (string.Equals(itemName, customTarget, StringComparison.Ordinal))
            {
                entry = AddIconMetadata(entry, itemName).TrimEnd();
            }

            catalog += Environment.NewLine + Environment.NewLine + entry;
            manifest += Environment.NewLine + $"  - {itemName}";
        }

        File.WriteAllText(catalogPath, catalog);
        File.WriteAllText(manifestPath, manifest);
    }

    private static string WaitForIconState(
        GorillaAppSession session,
        AutomationElement root,
        string automationId)
    {
        return session.WaitFor(() =>
        {
            var icon = root.FindFirstDescendant(cf => cf.ByAutomationId(automationId));
            if (icon is null)
            {
                return null;
            }

            var state = icon.Properties.ItemStatus.ValueOrDefault;
            return state is "Custom" or "Fallback" ? state : null;
        }, TimeSpan.FromSeconds(15));
    }

    private static string WaitForCachedIconPath(string cachePath, string itemName)
    {
        var deadline = DateTime.UtcNow.AddSeconds(15);
        while (DateTime.UtcNow < deadline)
        {
            if (File.Exists(cachePath))
            {
                using var document = JsonDocument.Parse(File.ReadAllText(cachePath));
                foreach (var item in document.RootElement.GetProperty("items").EnumerateArray())
                {
                    if (!string.Equals(item.GetProperty("itemName").GetString(), itemName, StringComparison.Ordinal))
                    {
                        continue;
                    }

                    if (item.TryGetProperty("iconPath", out var iconPath)
                        && !string.IsNullOrWhiteSpace(iconPath.GetString()))
                    {
                        return iconPath.GetString()!;
                    }
                }
            }
            Thread.Sleep(100);
        }

        throw new TimeoutException($"Timed out waiting for cached IconPath for '{itemName}'.");
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
