using System;
using System.Threading;
using System.Threading.Tasks;
using Gorilla.UI.Core.ViewModels;

namespace Gorilla.UI.App.Services;

internal sealed class AppCatalogSession : IDisposable
{
    private readonly CancellationTokenSource _lifetime = new();
    private readonly object _initializationLock = new();
    private Task? _initialization;

    public AppCatalogSession(HomeViewModel viewModel)
    {
        ViewModel = viewModel;
    }

    public HomeViewModel ViewModel { get; }

    public CancellationToken LifetimeToken => _lifetime.Token;

    public Task EnsureInitializedAsync()
    {
        lock (_initializationLock)
        {
            return _initialization ??= ViewModel.InitializeAsync(_lifetime.Token);
        }
    }

    public void Dispose()
    {
        _lifetime.Cancel();
        _lifetime.Dispose();
    }
}
