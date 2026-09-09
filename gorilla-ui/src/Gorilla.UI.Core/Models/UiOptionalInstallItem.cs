using System.ComponentModel;
using System.Runtime.CompilerServices;

namespace Gorilla.UI.Core.Models;

public sealed class UiOptionalInstallItem : INotifyPropertyChanged
{
    private string _status = string.Empty;
    private bool _isBusy;

    public required string ItemName { get; init; }

    public required string DisplayName { get; init; }

    public required string Version { get; init; }

    public required string Status
    {
        get => _status;
        set
        {
            _status = value;
            OnPropertyChanged();
        }
    }

    public bool IsInstalled { get; init; }

    public bool InstallAllowed { get; init; }

    public bool RemoveAllowed { get; init; }

    public string InstallUnavailableReason { get; init; } = string.Empty;

    public string RemoveUnavailableReason { get; init; } = string.Empty;

    public bool CanInstall => InstallAllowed && !IsBusy;

    public bool CanRemove => RemoveAllowed && !IsBusy;

    public bool IsBusy
    {
        get => _isBusy;
        set
        {
            if (_isBusy == value)
            {
                return;
            }

            _isBusy = value;
            OnPropertyChanged();
            OnPropertyChanged(nameof(CanInstall));
            OnPropertyChanged(nameof(CanRemove));
        }
    }

    public event PropertyChangedEventHandler? PropertyChanged;

    private void OnPropertyChanged([CallerMemberName] string? propertyName = null)
    {
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
    }
}
