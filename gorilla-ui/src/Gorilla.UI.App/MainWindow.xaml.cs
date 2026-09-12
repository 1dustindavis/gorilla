using Gorilla.UI.App.Views;
using Microsoft.UI.Xaml;

namespace Gorilla.UI.App
{
    public sealed partial class MainWindow : Window
    {
        public MainWindow()
        {
            InitializeComponent();
            RootFrame.Navigate(typeof(HomePage));
        }
    }
}
