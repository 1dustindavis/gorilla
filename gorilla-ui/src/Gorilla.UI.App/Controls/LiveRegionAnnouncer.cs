using System;
using System.Runtime.CompilerServices;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation.Peers;
using Microsoft.UI.Xaml.Controls;

namespace Gorilla.UI.App.Controls;

/// <summary>
/// Adds explicit UIA live-region notifications to ordinary WinUI TextBlocks.
///
/// AutomationProperties.LiveSetting communicates politeness to accessibility
/// clients, but WinUI does not reliably raise LiveRegionChanged when bound text
/// and visibility change together. This attached behavior observes the native
/// TextBlock and raises one event for each distinct, visible, non-empty message.
/// Keeping the native TextBlock avoids replacing framework semantics with a
/// custom control solely for announcement delivery.
/// </summary>
public static class LiveRegionAnnouncer
{
    public static readonly DependencyProperty IsEnabledProperty = DependencyProperty.RegisterAttached(
        "IsEnabled",
        typeof(bool),
        typeof(LiveRegionAnnouncer),
        new PropertyMetadata(false, OnIsEnabledChanged)
    );

    private static readonly ConditionalWeakTable<TextBlock, Subscription> Subscriptions = new();

    public static bool GetIsEnabled(DependencyObject element)
        => (bool)element.GetValue(IsEnabledProperty);

    public static void SetIsEnabled(DependencyObject element, bool value)
        => element.SetValue(IsEnabledProperty, value);

    private static void OnIsEnabledChanged(DependencyObject dependencyObject, DependencyPropertyChangedEventArgs args)
    {
        if (dependencyObject is not TextBlock textBlock)
        {
            return;
        }

        if (args.NewValue is true)
        {
            if (!Subscriptions.TryGetValue(textBlock, out _))
            {
                Subscriptions.Add(textBlock, new Subscription(textBlock));
            }
            return;
        }

        if (Subscriptions.TryGetValue(textBlock, out var subscription))
        {
            subscription.Dispose();
            Subscriptions.Remove(textBlock);
        }
    }

    private sealed class Subscription : IDisposable
    {
        private readonly TextBlock _textBlock;
        private long _textCallbackToken;
        private long _visibilityCallbackToken;
        private string? _lastAnnouncedText;
        private bool _disposed;

        public Subscription(TextBlock textBlock)
        {
            _textBlock = textBlock;
            _textBlock.Loaded += TextBlock_Loaded;
            _textBlock.Unloaded += TextBlock_Unloaded;

            if (_textBlock.IsLoaded)
            {
                StartObserving();
            }
        }

        public void Dispose()
        {
            if (_disposed)
            {
                return;
            }

            _disposed = true;
            StopObserving();
            _textBlock.Loaded -= TextBlock_Loaded;
            _textBlock.Unloaded -= TextBlock_Unloaded;
        }

        private void TextBlock_Loaded(object sender, RoutedEventArgs e)
        {
            StartObserving();
        }

        private void TextBlock_Unloaded(object sender, RoutedEventArgs e)
        {
            StopObserving();
        }

        private void StartObserving()
        {
            if (_disposed)
            {
                return;
            }

            if (_textCallbackToken == 0)
            {
                _textCallbackToken = _textBlock.RegisterPropertyChangedCallback(
                    TextBlock.TextProperty,
                    (_, _) => QueueAnnouncement()
                );
            }

            if (_visibilityCallbackToken == 0)
            {
                _visibilityCallbackToken = _textBlock.RegisterPropertyChangedCallback(
                    UIElement.VisibilityProperty,
                    (_, _) => QueueAnnouncement()
                );
            }

            QueueAnnouncement();
        }

        private void StopObserving()
        {
            if (_textCallbackToken != 0)
            {
                _textBlock.UnregisterPropertyChangedCallback(TextBlock.TextProperty, _textCallbackToken);
                _textCallbackToken = 0;
            }

            if (_visibilityCallbackToken != 0)
            {
                _textBlock.UnregisterPropertyChangedCallback(UIElement.VisibilityProperty, _visibilityCallbackToken);
                _visibilityCallbackToken = 0;
            }
        }

        private void QueueAnnouncement()
        {
            if (_disposed || !_textBlock.IsLoaded || _textBlock.Visibility != Visibility.Visible)
            {
                return;
            }

            var message = _textBlock.Text?.Trim();
            if (string.IsNullOrWhiteSpace(message) ||
                string.Equals(message, _lastAnnouncedText, StringComparison.Ordinal))
            {
                return;
            }

            _textBlock.DispatcherQueue.TryEnqueue(() =>
            {
                if (_disposed || !_textBlock.IsLoaded || _textBlock.Visibility != Visibility.Visible)
                {
                    return;
                }

                var currentMessage = _textBlock.Text?.Trim();
                if (string.IsNullOrWhiteSpace(currentMessage) ||
                    string.Equals(currentMessage, _lastAnnouncedText, StringComparison.Ordinal))
                {
                    return;
                }

                _lastAnnouncedText = currentMessage;
                var peer = FrameworkElementAutomationPeer.FromElement(_textBlock) ??
                    FrameworkElementAutomationPeer.CreatePeerForElement(_textBlock);
                peer?.RaiseAutomationEvent(AutomationEvents.LiveRegionChanged);
            });
        }
    }
}
