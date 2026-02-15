using System;
using System.IO;
using Gorilla.UI.App.Services;
using Gorilla.UI.App.ViewModels;
using Gorilla.UI.App.Views;
using Microsoft.UI.Xaml;

namespace Gorilla.UI.App
{
    public partial class App : Application
    {
        private Window? _window;

        public App()
        {
            InitializeComponent();
        }

        protected override void OnLaunched(Microsoft.UI.Xaml.LaunchActivatedEventArgs args)
        {
            var cacheFilePath = BuildCacheFilePath();
            var services = GorillaUiServices.Create(cacheFilePath);
            var operationTracker = new OperationTracker(services.Client);
            var homeViewModel = new HomeViewModel(services.Client, services.CacheCoordinator, operationTracker);
            var homePage = new HomePage(homeViewModel);

            _window = new MainWindow(homePage);
            _window.Activate();
        }

        private static string BuildCacheFilePath()
        {
            var localAppData = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
            var cacheDirectory = Path.Combine(localAppData, "Gorilla", "ui");
            Directory.CreateDirectory(cacheDirectory);
            return Path.Combine(cacheDirectory, "optional-installs-cache.json");
        }
    }
}
