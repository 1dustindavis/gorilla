using System;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation.Peers;
using Microsoft.UI.Xaml.Controls;

namespace Gorilla.UI.App.Controls;

/// <summary>
/// TextBlock that turns WinUI live-region metadata into an explicit UIA event.
/// WinUI exposes AutomationProperties.LiveSetting, but bound text that becomes
/// visible as part of the same update is not guaranteed to raise a
/// LiveRegionChanged event on its own. This control raises one event for each
/// distinct, visible, non-empty user-facing message.
/// </summary>
public sealed class LiveRegionTextBlock : TextBlock
{
    private long _textCallbackToken;
    private long _visibilityCallbackToken;
    private string? _lastAnnouncedText;

    public LiveRegionTextBlock()
    {
        Loaded += LiveRegionTextBlock_Loaded;
        Unloaded += LiveRegionTextBlock_Unloaded;
    }

    private void LiveRegionTextBlock_Loaded(object sender, RoutedEventArgs e)
    {
        if (_textCallbackToken == 0)
        {
            _textCallbackToken = RegisterPropertyChangedCallback(
                TextProperty,
                (_, _) => QueueAnnouncement()
            );
        }

        if (_visibilityCallbackToken == 0)
        {
            _visibilityCallbackToken = RegisterPropertyChangedCallback(
                VisibilityProperty,
                (_, _) => QueueAnnouncement()
            );
        }

        QueueAnnouncement();
    }

    private void LiveRegionTextBlock_Unloaded(object sender, RoutedEventArgs e)
    {
        if (_textCallbackToken != 0)
        {
            UnregisterPropertyChangedCallback(TextProperty, _textCallbackToken);
            _textCallbackToken = 0;
        }

        if (_visibilityCallbackToken != 0)
        {
            UnregisterPropertyChangedCallback(VisibilityProperty, _visibilityCallbackToken);
            _visibilityCallbackToken = 0;
        }
    }

    private void QueueAnnouncement()
    {
        if (!IsLoaded || Visibility != Visibility.Visible)
        {
            return;
        }

        var message = Text?.Trim();
        if (string.IsNullOrWhiteSpace(message) ||
            string.Equals(message, _lastAnnouncedText, StringComparison.Ordinal))
        {
            return;
        }

        DispatcherQueue.TryEnqueue(() =>
        {
            if (!IsLoaded || Visibility != Visibility.Visible)
            {
                return;
            }

            var currentMessage = Text?.Trim();
            if (string.IsNullOrWhiteSpace(currentMessage) ||
                string.Equals(currentMessage, _lastAnnouncedText, StringComparison.Ordinal))
            {
                return;
            }

            _lastAnnouncedText = currentMessage;
            var peer = FrameworkElementAutomationPeer.FromElement(this) ??
                FrameworkElementAutomationPeer.CreatePeerForElement(this);
            peer?.RaiseAutomationEvent(AutomationEvents.LiveRegionChanged);
        });
    }
}
