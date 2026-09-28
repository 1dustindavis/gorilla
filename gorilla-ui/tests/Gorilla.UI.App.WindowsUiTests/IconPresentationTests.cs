using System.Text.Json;
using System.Text.RegularExpressions;
using FlaUI.Core.AutomationElements;
using Xunit;

namespace Gorilla.UI.App.WindowsUiTests;

public sealed class IconPresentationTests
{
    private const string IconItemName = "Ps1V1";
    private const string FallbackItemName = "Ps1Failure";

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
        File.WriteAllText(catalogPath, AddIconMetadata(originalCatalog));

        try
        {
            RunWithDiagnostics(nameof(CatalogAndDetailsUseCustomIconWithFallbackAndRuntimeFailureRecovery), session =>
            {
                var home = new HomePageDriver(session);
                var shell = new CatalogShellDriver(session);

                shell.Refresh();
                shell.WaitForRefreshComplete(TimeSpan.FromSeconds(30));

                Assert.Equal("Custom", WaitForIconState(session, home.WaitForCard(IconItemName), "CatalogIcon"));
                Assert.Equal("Fallback", WaitForIconState(session, home.WaitForCard(FallbackItemName), "CatalogIcon"));
                Assert.Equal("Ps1V1", home.WaitForItem(IconItemName).Name);
                Assert.Equal("Ps1Failure", home.WaitForItem(FallbackItemName).Name);
                session.CaptureCheckpoint("catalog-icons-mixed", includeAutomationTree: true);

                home.OpenDetails(IconItemName);
                Assert.Equal("Custom", WaitForIconState(session, session.MainWindow, "DetailsIcon"));
                session.CaptureCheckpoint("details-custom-icon", includeAutomationTree: true);

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

    private static string AddIconMetadata(string catalog)
    {
        var entry = new Regex(
            @"(?m)^(Ps1V1:\r?\n\s+display_name:\s*Ps1V1\s*)$",
            RegexOptions.CultureInvariant
        );
        var updated = entry.Replace(
            catalog,
            "$1" + Environment.NewLine + "  icon: icons/catalog-icon-test.png",
            count: 1
        );
        if (string.Equals(updated, catalog, StringComparison.Ordinal))
        {
            throw new InvalidOperationException("Unable to add icon metadata to Ps1V1 fixture.");
        }
        return updated;
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
