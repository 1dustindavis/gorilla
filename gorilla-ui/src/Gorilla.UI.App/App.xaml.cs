using System;
using System.IO;
using Gorilla.UI.App.Services;
using Gorilla.UI.Core.Services;
using Gorilla.UI.Core.ViewModels;
using Microsoft.UI.Xaml;

namespace Gorilla.UI.App
{
    public partial class App : Application
    {
        // Source E2E tests use an isolated service identity. Production always
        // uses NamedPipeClientOptions.Default (gorilla-service).
        private const string E2EPipeNameVariable = "GORILLA_UI_E2E_PIPE_NAME";
        private const string CachePathOverrideVariable = "GORILLA_UI_CACHE_PATH";
        private Window? _window;
        private AppCatalogSession? _session;

        public App()
        {
            InitializeComponent();
        }

        internal static AppCatalogSession CurrentSession
            => ((App)Current)._session ?? throw new InvalidOperationException("App Catalog session is not initialized.");

        protected override void OnLaunched(Microsoft.UI.Xaml.LaunchActivatedEventArgs args)
        {
            var cacheFilePath = BuildCacheFilePath();
            var pipeName = Environment.GetEnvironmentVariable(E2EPipeNameVariable);
            var services = GorillaUiServices.Create(cacheFilePath, pipeName);
            var operationTracker = new OperationTracker(services.Client);
            var homeViewModel = new HomeViewModel(services.Client, services.CacheCoordinator, operationTracker);
            _session = new AppCatalogSession(homeViewModel);

            _window = new MainWindow();
            _window.Closed += Window_Closed;
            _window.Activate();
        }

        private void Window_Closed(object sender, WindowEventArgs args)
        {
            if (_window is not null)
            {
                _window.Closed -= Window_Closed;
            }
            _session?.Dispose();
            _session = null;
        }

        private static string BuildCacheFilePath()
        {
            var overridePath = Environment.GetEnvironmentVariable(CachePathOverrideVariable);
            if (!string.IsNullOrWhiteSpace(overridePath))
            {
                var fullPath = Path.GetFullPath(overridePath);
                var overrideDirectory = Path.GetDirectoryName(fullPath);
                if (!string.IsNullOrWhiteSpace(overrideDirectory))
                {
                    Directory.CreateDirectory(overrideDirectory);
                }
                return fullPath;
            }

            var localAppData = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
            var cacheDirectory = Path.Combine(localAppData, "Gorilla", "ui");
            Directory.CreateDirectory(cacheDirectory);
            return Path.Combine(cacheDirectory, "optional-installs-cache.json");
        }
    }
}
