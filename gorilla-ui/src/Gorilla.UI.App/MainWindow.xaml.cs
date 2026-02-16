using Microsoft.UI.Xaml;

namespace Gorilla.UI.App
{
    public sealed partial class MainWindow : Window
    {
        public MainWindow(UIElement startupContent)
        {
            InitializeComponent();
            Content = startupContent;
        }
    }
}
