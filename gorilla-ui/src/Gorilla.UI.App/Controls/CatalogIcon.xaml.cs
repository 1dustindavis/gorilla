using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media.Imaging;
using Windows.Storage;
using Windows.Storage.Streams;

namespace Gorilla.UI.App.Controls;

public sealed partial class CatalogIcon : UserControl
{
    private long _loadGeneration;

    public CatalogIcon()
    {
        InitializeComponent();
        SizeChanged += (_, _) => ApplySize();
        ApplySize();
    }

    public static readonly DependencyProperty IconPathProperty = DependencyProperty.Register(
        nameof(IconPath),
        typeof(string),
        typeof(CatalogIcon),
        new PropertyMetadata(null, OnIconPathChanged)
    );

    public static readonly DependencyProperty IconSizeProperty = DependencyProperty.Register(
        nameof(IconSize),
        typeof(double),
        typeof(CatalogIcon),
        new PropertyMetadata(38d, OnIconSizeChanged)
    );

    public string? IconPath
    {
        get => (string?)GetValue(IconPathProperty);
        set => SetValue(IconPathProperty, value);
    }

    public double IconSize
    {
        get => (double)GetValue(IconSizeProperty);
        set => SetValue(IconSizeProperty, value);
    }

    private static void OnIconPathChanged(DependencyObject dependencyObject, DependencyPropertyChangedEventArgs args)
    {
        var control = (CatalogIcon)dependencyObject;
        _ = control.LoadIconAsync(args.NewValue as string);
    }

    private static void OnIconSizeChanged(DependencyObject dependencyObject, DependencyPropertyChangedEventArgs args)
        => ((CatalogIcon)dependencyObject).ApplySize();

    private void ApplySize()
    {
        var size = Math.Max(1, IconSize);
        Width = size;
        Height = size;
        IconRoot.Width = size;
        IconRoot.Height = size;
        FallbackTile.Width = size;
        FallbackTile.Height = size;
        FallbackTile.CornerRadius = new CornerRadius(Math.Max(4, size * 0.18));
        FallbackGlyph.FontSize = Math.Max(12, size * 0.49);

        // Preserve the established footprint while avoiding disproportionate upscaling
        // of tiny artwork. Larger source images can naturally fill the available area.
        CustomImage.MaxWidth = size;
        CustomImage.MaxHeight = size;
    }

    private async Task LoadIconAsync(string? path)
    {
        var generation = Interlocked.Increment(ref _loadGeneration);
        ShowFallback();

        if (string.IsNullOrWhiteSpace(path))
        {
            return;
        }

        try
        {
            var file = await StorageFile.GetFileFromPathAsync(path);
            using IRandomAccessStream stream = await file.OpenAsync(FileAccessMode.Read);

            var bitmap = new BitmapImage();
            await bitmap.SetSourceAsync(stream);

            if (generation != Volatile.Read(ref _loadGeneration)
                || !string.Equals(path, IconPath, StringComparison.Ordinal))
            {
                return;
            }

            CustomImage.Source = bitmap;
            CustomImage.Visibility = Visibility.Visible;
            FallbackTile.Visibility = Visibility.Collapsed;
        }
        catch
        {
            if (generation == Volatile.Read(ref _loadGeneration)
                && string.Equals(path, IconPath, StringComparison.Ordinal))
            {
                ShowFallback();
            }
        }
    }

    private void ShowFallback()
    {
        CustomImage.Source = null;
        CustomImage.Visibility = Visibility.Collapsed;
        FallbackTile.Visibility = Visibility.Visible;
    }
}
