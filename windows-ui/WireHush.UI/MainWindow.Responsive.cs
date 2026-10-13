using Microsoft.UI;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Shapes;

namespace WireHush.UI;

public sealed partial class MainWindow
{
    private Grid? _responsiveBody;
    private Grid? _responsiveRail;
    private FrameworkElement? _railHeading;
    private StackPanel? _expandedRailCommands;
    private StackPanel? _compactRailCommands;
    private Button? _compactDelete;
    private Ellipse? _compactManagerStatus;
    private bool _railCompact;

    private static Button CompactAction(string glyph, string description, RoutedEventHandler handler)
    {
        var button = new Button
        {
            Content = new FontIcon { Glyph = glyph, FontSize = 21 },
            Height = 44,
            HorizontalAlignment = HorizontalAlignment.Stretch,
            HorizontalContentAlignment = HorizontalAlignment.Center
        };
        Microsoft.UI.Xaml.Automation.AutomationProperties.SetName(button, description);
        ToolTipService.SetToolTip(button, description);
        button.Click += handler;
        return button;
    }

    private StackPanel BuildCompactRailCommands()
    {
        var commands = new StackPanel { Spacing = 8, Visibility = Visibility.Collapsed };
        commands.Children.Add(CompactAction("\uE710", "Add tunnel", async (_, _) => await AddTunnelAsync()));
        commands.Children.Add(CompactAction("\uE8A5", "Import tunnels", async (_, _) => await ImportTunnelAsync()));
        commands.Children.Add(CompactAction("\uE721", "Search tunnels", async (_, _) =>
        {
            var query = new TextBox { Header = "Tunnel name", Text = _search?.Text ?? string.Empty };
            var dialog = new ContentDialog
            {
                Title = "Search connections",
                Content = query,
                PrimaryButtonText = "Search",
                CloseButtonText = "Cancel",
                XamlRoot = RootGrid.XamlRoot
            };
            if (await ShowDialogAsync(dialog) == ContentDialogResult.Primary && _search is not null)
                _search.Text = query.Text;
        }));
        _compactDelete = CompactAction("\uE74D", "Delete selected tunnel", async (_, _) =>
        {
            if (_selectedTunnel is not null) await DeleteTunnelAsync(_selectedTunnel);
        });
        _compactDelete.IsEnabled = false;
        commands.Children.Add(_compactDelete);
        _compactManagerStatus = new Ellipse
        {
            Width = 12,
            Height = 12,
            Fill = Brush("WireHushMutedBrush"),
            HorizontalAlignment = HorizontalAlignment.Center
        };
        ToolTipService.SetToolTip(_compactManagerStatus, "WireHush Manager status");
        commands.Children.Add(_compactManagerStatus);
        return commands;
    }

    private void UpdateResponsiveChrome()
    {
        if (_responsiveBody is null || _responsiveRail is null || _railHeading is null ||
            _expandedRailCommands is null || _compactRailCommands is null || _search is null)
            return;
        var compact = RootGrid.ActualWidth < 1380;
        if (_railCompact == compact) return;
        _railCompact = compact;
        _responsiveBody.ColumnDefinitions[0].Width = new GridLength(compact ? 82 : 300);
        _responsiveRail.Padding = compact ? new Thickness(8, 12, 8, 14) : new Thickness(20, 20, 20, 28);
        _railHeading.Visibility = compact ? Visibility.Collapsed : Visibility.Visible;
        _search.Visibility = compact ? Visibility.Collapsed : Visibility.Visible;
        _expandedRailCommands.Visibility = compact ? Visibility.Collapsed : Visibility.Visible;
        _compactRailCommands.Visibility = compact ? Visibility.Visible : Visibility.Collapsed;
        RenderTunnelList();
    }
}
