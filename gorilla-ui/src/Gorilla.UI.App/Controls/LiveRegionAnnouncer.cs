using System;
using System.Runtime.CompilerServices;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Automation.Peers;
using Microsoft.UI.Xaml.Controls;

namespace Gorilla.UI.App.Controls;

/// <summary>
/// Adds explicit UIA live-region notifications to ordinary WinUI TextBlocks.
///
/// The semantic announcement is AutomationProperties.Name when one is supplied;
/// otherwise the visible Text is used. Existing content is baselined after the
/// initial load/binding turn so navigating to or realizing an element does not
/// announce stale state as though it just changed. Subsequent semantic changes
/// raise one deduplicated LiveRegionChanged event.
///
/// If an enabled live element is hidden through its own Visibility property,
/// semantic changes remain silent while hidden. When that element later becomes
/// visible, its current message is announced once. This intentionally treats the
/// appearance of a new status/banner as a user-facing transition while avoiding
/// announcements merely because a page or virtualized row was loaded.
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
        private long _nameCallbackToken;
        private long _visibilityCallbackToken;
        private string? _lastAnnouncedText;
        private bool _isPriming;
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

            _isPriming = true;

            if (_textCallbackToken == 0)
            {
                _textCallbackToken = _textBlock.RegisterPropertyChangedCallback(
                    TextBlock.TextProperty,
                    (_, _) => QueueAnnouncement()
                );
            }

            if (_nameCallbackToken == 0)
            {
                _nameCallbackToken = _textBlock.RegisterPropertyChangedCallback(
                    AutomationProperties.NameProperty,
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

            // XAML bindings can complete after Loaded. Absorb one dispatcher turn of
            // initial binding/realization churn into the baseline so that content that
            // merely appeared with the page is never raised as a fresh live update.
            if (!_textBlock.DispatcherQueue.TryEnqueue(CompletePriming))
            {
                CompletePriming();
            }
        }

        private void CompletePriming()
        {
            if (_disposed || !_textBlock.IsLoaded)
            {
                return;
            }

            _lastAnnouncedText = _textBlock.Visibility == Visibility.Visible
                ? CurrentSemanticMessage()
                : null;
            _isPriming = false;
        }

        private void StopObserving()
        {
            _isPriming = false;

            if (_textCallbackToken != 0)
            {
                _textBlock.UnregisterPropertyChangedCallback(TextBlock.TextProperty, _textCallbackToken);
                _textCallbackToken = 0;
            }

            if (_nameCallbackToken != 0)
            {
                _textBlock.UnregisterPropertyChangedCallback(AutomationProperties.NameProperty, _nameCallbackToken);
                _nameCallbackToken = 0;
            }

            if (_visibilityCallbackToken != 0)
            {
                _textBlock.UnregisterPropertyChangedCallback(UIElement.VisibilityProperty, _visibilityCallbackToken);
                _visibilityCallbackToken = 0;
            }
        }

        private string? CurrentSemanticMessage()
        {
            var accessibleName = AutomationProperties.GetName(_textBlock)?.Trim();
            if (!string.IsNullOrWhiteSpace(accessibleName))
            {
                return accessibleName;
            }

            var visibleText = _textBlock.Text?.Trim();
            return string.IsNullOrWhiteSpace(visibleText) ? null : visibleText;
        }

        private void QueueAnnouncement()
        {
            if (_disposed || _isPriming || !_textBlock.IsLoaded || _textBlock.Visibility != Visibility.Visible)
            {
                return;
            }

            var message = CurrentSemanticMessage();
            if (string.IsNullOrWhiteSpace(message) ||
                string.Equals(message, _lastAnnouncedText, StringComparison.Ordinal))
            {
                return;
            }

            _textBlock.DispatcherQueue.TryEnqueue(() =>
            {
                if (_disposed || _isPriming || !_textBlock.IsLoaded || _textBlock.Visibility != Visibility.Visible)
                {
                    return;
                }

                var currentMessage = CurrentSemanticMessage();
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
