using System.ComponentModel;
using System.Runtime.CompilerServices;

namespace Gorilla.UI.App.Models;

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
        init => _status = value;
        set
        {
            _status = value;
            OnPropertyChanged();
        }
    }

    public bool IsInstalled { get; init; }

    public bool IsBusy
    {
        get => _isBusy;
        set
        {
            _isBusy = value;
            OnPropertyChanged();
        }
    }

    public event PropertyChangedEventHandler? PropertyChanged;

    private void OnPropertyChanged([CallerMemberName] string? propertyName = null)
    {
        PropertyChanged?.Invoke(this, new PropertyChangedEventArgs(propertyName));
    }
}
