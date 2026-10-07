using System.Collections.Generic;
using System.Diagnostics;
using Microsoft.UI;
using Microsoft.UI.Windowing;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Shapes;
using Windows.Foundation;
using Windows.Graphics;
using WinRT.Interop;
using WireHush.UI.Models;
using WireHush.UI.Services;

namespace WireHush.UI;

public sealed partial class MainWindow : Window
{
    private readonly AppWindow _appWindow;
    private readonly WindowPreferences _windowPreferences;

    private readonly CancellationTokenSource _lifetime = new();
    private readonly List<TunnelSummary> _tunnels = [];
    private readonly TrafficHistory _traffic = new();
    private readonly DispatcherTimer _refreshTimer = new() { Interval = TimeSpan.FromSeconds(2) };
    private IManagerClient? _managerClient;
    private StackPanel? _headerCommands;
    private StackPanel? _tunnelList;
    private ContentControl? _detailHost;
    private InfoBar? _managerInfo;
    private TextBlock? _connectionStatus;
    private Button? _railDelete;
    private AutoSuggestBox? _search;
    private string? _selectedTunnel;
    private TunnelDetails? _selectedDetails;
    private string _section = "Overview";
    private bool _connecting;
    private bool _tunnelActionPending;
    private bool _allowClose;
    private bool _exiting;
    private bool _refreshing;




    public MainWindow()
    {
        try
        {
            InitializeComponent();
            Title = "WireHush"; ExtendsContentIntoTitleBar = true;
            var hwnd = WindowNative.GetWindowHandle(this);
            _appWindow = AppWindow.GetFromWindowId(Win32Interop.GetWindowIdFromWindow(hwnd));
            _windowPreferences = new WindowPreferences(hwnd, _appWindow); ConfigureTitleBar(); _appWindow.Closing += OnAppWindowClosing; Closed += OnClosed;
            RootGrid.SizeChanged += (_, _) => { if (!_exiting) RenderDetails(); };
            BuildLiveShell(); _refreshTimer.Interval = TimeSpan.FromSeconds(1); _refreshTimer.Tick += async (_, _) =>
            {
                try { if (_managerClient is null || !_managerClient.Connected) await RefreshAsync(); }
                catch (Exception) { SetConnectionStatus("Manager refresh failed — retrying."); }
            }; _refreshTimer.Start(); _ = ConnectAndLoadAsync();
        }
        catch (Exception)
        {
            throw;
        }
    }

    private void ConfigureTitleBar()
    {
        if (!AppWindowTitleBar.IsCustomizationSupported()) return;
        var bar = _appWindow.TitleBar;
        bar.ButtonBackgroundColor = Colors.Transparent; bar.ButtonInactiveBackgroundColor = Colors.Transparent;
        bar.ButtonHoverBackgroundColor = Windows.UI.Color.FromArgb(0x35, 0xFF, 0xFF, 0xFF); bar.ButtonPressedBackgroundColor = Windows.UI.Color.FromArgb(0x22, 0xFF, 0xFF, 0xFF);
        bar.ButtonForegroundColor = Windows.UI.Color.FromArgb(0xFF, 0xF3, 0xF7, 0xFA); bar.ButtonInactiveForegroundColor = Windows.UI.Color.FromArgb(0xA0, 0xF3, 0xF7, 0xFA);
        RootGrid.SizeChanged += (_, _) => { if (_headerCommands is not null) _headerCommands.Margin = new Thickness(0, 0, bar.RightInset + 8, 0); };
    }

    private void BuildLiveShell()
    {
        RootGrid.Children.Clear(); RootGrid.RowDefinitions.Clear(); RootGrid.Background = Brush("WireHushCanvasBrush");
        RootGrid.RowDefinitions.Add(new RowDefinition { Height = new GridLength(108) }); RootGrid.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
        var header = new Grid { Padding = new Thickness(32, 12, 20, 10), Background = Brush("WireHushHeaderBrush") };
        header.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto }); header.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) }); header.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        var brand = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 18, VerticalAlignment = VerticalAlignment.Center };
        brand.Children.Add(new Image { Source = new Microsoft.UI.Xaml.Media.Imaging.BitmapImage(new Uri("ms-appx:///Assets/WireHush_Icon.png")), Width = 96, Height = 72, Stretch = Stretch.Uniform });
        var brandText = new StackPanel { VerticalAlignment = VerticalAlignment.Center }; brandText.Children.Add(Text("WireHush", 40, true)); brandText.Children.Add(Text("Private network control", 18)); brand.Children.Add(brandText); header.Children.Add(brand);
        var drag = new Grid(); Grid.SetColumn(drag, 1); header.Children.Add(drag); SetTitleBar(drag);
        _headerCommands = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 12, VerticalAlignment = VerticalAlignment.Center };
        _headerCommands.Children.Add(HeaderButton("Settings", "\uE713", async (_, _) => await ShowSettingsAsync())); _headerCommands.Children.Add(HeaderButton("Log", "\uE8A5", async (_, _) => await ShowLogAsync())); _headerCommands.Children.Add(HeaderButton("Help", "\uE897", (_, _) => ShowHelp())); Grid.SetColumn(_headerCommands, 2); header.Children.Add(_headerCommands); RootGrid.Children.Add(header);
        var body = new Grid(); body.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(342) }); body.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) }); Grid.SetRow(body, 1); RootGrid.Children.Add(body); body.Children.Add(BuildRail());
        var main = new Grid(); main.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto }); main.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
        _managerInfo = new InfoBar { IsOpen = true, Severity = InfoBarSeverity.Informational, Title = "WireHush Manager", Message = "Connecting to WireHush Manager…", Margin = new Thickness(34, 14, 38, 0) }; main.Children.Add(_managerInfo);
        _detailHost = new ContentControl { Padding = new Thickness(34, 16, 38, 28) }; Grid.SetRow(_detailHost, 1); main.Children.Add(_detailHost); Grid.SetColumn(main, 1); body.Children.Add(main); RenderDetails();
    }

    private UIElement BuildRail()
    {
        var rail = new Grid { Padding = new Thickness(20, 20, 20, 28), Background = Brush("WireHushRailBrush") };
        rail.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto }); rail.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto }); rail.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) }); rail.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        rail.Children.Add(Text("Connections", 27, true)); _search = new AutoSuggestBox { Height = 48, PlaceholderText = "Search tunnels...", QueryIcon = new SymbolIcon(Symbol.Find), FontSize = 16, Margin = new Thickness(0, 0, 0, 14) }; _search.TextChanged += (_, _) => RenderTunnelList(); Grid.SetRow(_search, 1); rail.Children.Add(_search);
        _tunnelList = new StackPanel { Spacing = 6 }; var scroll = new ScrollViewer { Content = _tunnelList, VerticalScrollBarVisibility = ScrollBarVisibility.Auto }; Grid.SetRow(scroll, 2); rail.Children.Add(scroll);
        var commands = new StackPanel { Spacing = 12 }; commands.Children.Add(ActionButton("Add Tunnel", "\uE710", true, async (_, _) => await AddTunnelAsync())); commands.Children.Add(ActionButton("Import Tunnel(s)", "\uE8A5", false, async (_, _) => await ImportTunnelAsync())); _railDelete = ActionButton("Delete", "\uE74D", false, async (_, _) => { if (_selectedTunnel is not null) await DeleteTunnelAsync(_selectedTunnel); }); _railDelete.IsEnabled = false; commands.Children.Add(_railDelete); _connectionStatus = Text("Connecting to WireHush Manager…", 13, false, Brush("WireHushMutedBrush")); commands.Children.Add(_connectionStatus); Grid.SetRow(commands, 3); rail.Children.Add(commands); return rail;
    }

    private static TextBlock Text(string text, double size, bool strong = false, Brush? foreground = null) => new() { Text = text, FontSize = size, FontWeight = strong ? Microsoft.UI.Text.FontWeights.SemiBold : Microsoft.UI.Text.FontWeights.Normal, Foreground = foreground ?? Brush("WireHushTextBrush"), TextWrapping = TextWrapping.Wrap, FontFamily = new FontFamily("Segoe UI Variable Text") };
    private static Brush Brush(string key) => (Brush)Application.Current.Resources[key];
    private static Button HeaderButton(string text, string glyph, RoutedEventHandler handler) { var b = new Button { Style = (Style)Application.Current.Resources["WireHushHeaderCommandStyle"] }; var s = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 9 }; s.Children.Add(new FontIcon { Glyph = glyph, FontSize = 22 }); s.Children.Add(new TextBlock { Text = text, FontSize = 16, VerticalAlignment = VerticalAlignment.Center }); b.Content = s; b.Click += handler; return b; }
    private static Button ActionButton(string text, string glyph, bool primary, RoutedEventHandler handler) { var b = new Button { Style = (Style)Application.Current.Resources[primary ? "WireHushPrimaryButtonStyle" : "WireHushSecondaryButtonStyle"], HorizontalAlignment = HorizontalAlignment.Stretch }; var s = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 10, HorizontalAlignment = HorizontalAlignment.Center }; s.Children.Add(new FontIcon { Glyph = glyph, FontSize = 20 }); s.Children.Add(new TextBlock { Text = text, VerticalAlignment = VerticalAlignment.Center }); b.Content = s; b.Click += handler; return b; }

    private async Task ConnectAndLoadAsync()
    {
        if (_connecting) return;
        _connecting = true;
        try
        {
            SetConnectionStatus("Connecting to WireHush Manager…"); await WireHushService.EnsureRunningAsync(_lifetime.Token); var delay = TimeSpan.FromMilliseconds(250);
            for (var attempt = 0; attempt < 6 && !_lifetime.IsCancellationRequested; attempt++)
            {
                try { await _managerClient.DisposeAsyncIfPresent(); var client = new GrpcManagerClient(); try { await client.ConnectAsync(_lifetime.Token); } catch { await client.DisposeAsync(); throw; } _managerClient = client; client.SnapshotChanged += () => DispatcherQueue.TryEnqueue(async () => await RefreshAsync()); client.ConnectionLost += () => DispatcherQueue.TryEnqueue(async () => await RefreshAsync()); UiStartupLog.Write("manager-connected"); SetConnectionStatus("Connected to Manager"); await LoadTunnelsAsync(); return; }
                catch (Exception ex) when (ex is IOException or InvalidOperationException or TimeoutException or Grpc.Core.RpcException) { await Task.Delay(delay, _lifetime.Token); delay = TimeSpan.FromMilliseconds(Math.Min(delay.TotalMilliseconds * 2, 5000)); }
            }
            SetConnectionStatus("Manager unavailable — retrying in the background.");
        }
        catch (OperationCanceledException) { }
        catch (Exception ex) { SetConnectionStatus(SafeError(ex)); }
        finally { _connecting = false; }
    }

    private async Task RefreshAsync()
    {
        if (_lifetime.IsCancellationRequested || _exiting || _refreshing) return;
        if (_managerClient is null) { await ConnectAndLoadAsync(); return; }
        _refreshing = true;
        try
        {
            if (!_managerClient.Connected) throw new IOException("Manager unavailable");
            await LoadTunnelsAsync();
        }
        catch (Exception)
        {
            SetConnectionStatus("Manager connection was lost — reconnecting.");
            await _managerClient.DisposeAsyncIfPresent();
            _managerClient = null;
			_traffic.Clear();
            for (var i = 0; i < _tunnels.Count; i++) _tunnels[i] = _tunnels[i] with { State = "unknown" };
            RenderTunnelList();
            _selectedDetails = _selectedDetails is null ? null : _selectedDetails with { State = "unknown", MetricsAvailable = false };
            RenderDetails();
        }
        finally { _refreshing = false; }
    }

    private async Task LoadTunnelsAsync()
    {
        if (_managerClient is null) return;
        var incoming = await _managerClient.ListTunnelsAsync(_lifetime.Token);
        var changed = !_tunnels.SequenceEqual(incoming);
        _tunnels.Clear(); _tunnels.AddRange(incoming);
        if (_selectedTunnel is null || !_tunnels.Any(t => t.Id == _selectedTunnel))
        {
            _selectedTunnel = _tunnels.FirstOrDefault()?.Id;
            ResetTraffic(); changed = true;
        }
        if (changed) RenderTunnelList();
        if (_managerClient.DeviceBusyForAnotherUser)
            SetConnectionStatus("Another Windows account is using this device's tunnel. That account must disconnect before you can connect.");
        else if (_managerInfo?.Message.StartsWith("Another Windows account", StringComparison.Ordinal) == true)
            SetConnectionStatus("Connected to Manager");
        if (_selectedTunnel is null) RenderDetails();
        else await LoadSelectedTunnelAsync(_selectedTunnel);
    }
    private async Task LoadSelectedTunnelAsync(string name)
    {
        if (_managerClient is null || name != _selectedTunnel) return; var summary = _tunnels.FirstOrDefault(t => t.Id == name);
        _selectedDetails = NormalizeDetails(summary?.State == "connected" ? await GetRuntimeOrStoredAsync(name) : await _managerClient.GetTunnelAsync(name, _lifetime.Token)); if (_selectedDetails.MetricsAvailable) AddTrafficSample(_selectedDetails); else _traffic.Clear(); RenderDetails(); }
    private async Task<TunnelDetails> GetRuntimeOrStoredAsync(string name) { try { return await _managerClient!.GetRuntimeAsync(name, _lifetime.Token); } catch (InvalidOperationException) { return await _managerClient!.GetTunnelAsync(name, _lifetime.Token); } }
    private static TunnelDetails NormalizeDetails(TunnelDetails details)
    {
        var dns = details.Dns ?? new DnsDetails("Unavailable", "Not configured", "—", Array.Empty<string>(), false, false);
        var peers = (details.Peers ?? Array.Empty<PeerDetails>()).Select(peer => peer with { AllowedIPs = peer.AllowedIPs ?? Array.Empty<string>() }).ToArray();
        return details with
        {
            Ipv4Addresses = details.Ipv4Addresses ?? Array.Empty<string>(),
            Ipv6Addresses = details.Ipv6Addresses ?? Array.Empty<string>(),
            AllowedIPs = details.AllowedIPs ?? Array.Empty<string>(),
            Dns = dns with { BootstrapResolvers = dns.BootstrapResolvers ?? Array.Empty<string>() },
            Peers = peers
        };
    }

    private string? FocusedTag() => RootGrid.XamlRoot is null ? null :
        (Microsoft.UI.Xaml.Input.FocusManager.GetFocusedElement(RootGrid.XamlRoot) as FrameworkElement)?.Tag as string;
    private static void RestoreFocusTag(DependencyObject root, string? tag)
    {
        if (tag is null) return;
        if (root is Control control && Equals(control.Tag, tag) && control.IsEnabled)
        { control.Focus(FocusState.Keyboard); return; }
        for (var index = 0; index < VisualTreeHelper.GetChildrenCount(root); index++)
            RestoreFocusTag(VisualTreeHelper.GetChild(root, index), tag);
    }
    private void RenderTunnelList()
    {
        if (_tunnelList is null) return;
        var focused = FocusedTag();
        _tunnelList.Children.Clear();
        var query = _search?.Text?.Trim() ?? string.Empty;
        foreach (var tunnel in _tunnels.Where(t => string.IsNullOrEmpty(query) || t.Name.Contains(query, StringComparison.OrdinalIgnoreCase)))
        {
            var selected = tunnel.Id == _selectedTunnel;
            var row = new Grid { Height = 76, Padding = new Thickness(selected ? 12 : 16, 0, 4, 0), Background = selected ? Brush("WireHushSelectedBrush") : new SolidColorBrush(Colors.Transparent), CornerRadius = new CornerRadius(7) };
            row.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
            row.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(38) });
            var body = new Grid();
            body.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(30) });
            body.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
            body.Children.Add(new Ellipse { Width = 16, Height = 16, Fill = StateBrush(tunnel.State), VerticalAlignment = VerticalAlignment.Center });
            var label = new StackPanel { VerticalAlignment = VerticalAlignment.Center, Spacing = 2 };
            var name = Text(tunnel.Name, 18, true); name.TextWrapping = TextWrapping.NoWrap; name.TextTrimming = TextTrimming.CharacterEllipsis;
            ToolTipService.SetToolTip(name, tunnel.Name);
            label.Children.Add(name);
            var duplicate = _tunnels.Count(t => t.Name == tunnel.Name && t.ScopeLabel == tunnel.ScopeLabel) > 1;
            label.Children.Add(Text($"{tunnel.ScopeLabel} · {StateText(tunnel.State)}{(duplicate ? " · " + tunnel.Id[..8] : "")}", 13, false, Brush("WireHushMutedBrush")));
            Grid.SetColumn(label, 1); body.Children.Add(label);
            var select = new Button { Content = body, Tag = "select:" + tunnel.Id, Background = new SolidColorBrush(Colors.Transparent), BorderThickness = new Thickness(0), Padding = new Thickness(0), HorizontalAlignment = HorizontalAlignment.Stretch, HorizontalContentAlignment = HorizontalAlignment.Stretch };
            Microsoft.UI.Xaml.Automation.AutomationProperties.SetName(select, $"{tunnel.Name}, {tunnel.ScopeLabel}, {StateText(tunnel.State)}");
            select.Click += async (_, _) =>
            {
                try { _selectedTunnel = tunnel.Id; _section = "Overview"; ResetTraffic(); RenderTunnelList(); await LoadSelectedTunnelAsync(tunnel.Id); }
                catch (Exception ex) { ShowNotice(SafeError(ex)); }
            };
            row.Children.Add(select);
            var menu = new Button { Tag = "menu:" + tunnel.Id, Background = new SolidColorBrush(Colors.Transparent), BorderThickness = new Thickness(0), Padding = new Thickness(0), Content = new FontIcon { Glyph = "\uE712", FontSize = 20, Foreground = Brush("WireHushMutedBrush") } };
            ToolTipService.SetToolTip(menu, "Tunnel actions");
            Microsoft.UI.Xaml.Automation.AutomationProperties.SetName(menu, $"Actions for {tunnel.Name}");
            menu.Click += (_, _) => ShowTunnelMenu(tunnel.Id);
            Grid.SetColumn(menu, 1); row.Children.Add(menu);
            if (focused == select.Tag as string || focused == menu.Tag as string) row.Loaded += (_, _) => RestoreFocusTag(row, focused);
            _tunnelList.Children.Add(row);
        }
        if (_railDelete is not null) _railDelete.IsEnabled = _selectedTunnel is not null && _managerClient?.Connected == true && _tunnels.FirstOrDefault(t => t.Id == _selectedTunnel)?.MayEdit == true;
    }
    private Brush StateBrush(string state) => state == "connected" ? Brush("WireHushGreenBrush") : state is "connecting" or "disconnecting" ? Brush("WireHushCyanBrush") : Brush("WireHushMutedBrush");
    private static string StateText(string state) => state switch { "connected" => "Connected", "connecting" => "Connecting", "disconnecting" => "Disconnecting", "disconnected" => "Disconnected", _ => "Status unavailable" };

    private void RenderDetails()
    {
        if (_detailHost is null || _section == "Help") return; var focused = FocusedTag(); if (_selectedDetails is null) { _detailHost.Content = EmptyDetails(); return; }
        var root = new Grid(); root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto }); root.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto }); root.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) });
        root.Children.Add(SummaryCard());
        var tabs = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 8, Margin = new Thickness(0, 26, 0, 22) }; foreach (var section in new[] { "Overview", "Network", "DNS", "Peer", "Allowed IPs" }) { var s = section; var tab = new Button { Content = s, Tag = "tab:" + s, Padding = new Thickness(14, 8, 14, 8), Background = _section == s ? Brush("WireHushSelectedBrush") : new SolidColorBrush(Colors.Transparent), Foreground = Brush("WireHushTextBrush"), BorderBrush = Brush("WireHushBorderBrush"), CornerRadius = new CornerRadius(6) }; tab.Click += (_, _) => { _section = s; RenderDetails(); }; tabs.Children.Add(tab); } Grid.SetRow(tabs, 1); root.Children.Add(tabs);
        var content = _section switch { "Network" => NetworkPage(), "DNS" => DnsPage(), "Peer" => PeerPage(), "Allowed IPs" => AllowedPage(), _ => OverviewPage() }; var scroll = new ScrollViewer { Content = content, VerticalScrollBarVisibility = ScrollBarVisibility.Auto }; Grid.SetRow(scroll, 2); root.Children.Add(scroll); _detailHost.Content = root; root.Loaded += (_, _) => RestoreFocusTag(root, focused);
    }
    private UIElement SummaryCard()
    {
        var border = new Border { Style = (Style)Application.Current.Resources["WireHushCardStyle"], Padding = new Thickness(22), Margin = new Thickness(0, 0, 0, 0) };
        var stack = new StackPanel { Spacing = 20 }; var heading = new Grid(); heading.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) }); heading.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
        var title = new StackPanel { Spacing = 4 }; title.Children.Add(Text(_selectedDetails!.Name, 32, true)); var status = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 8 }; status.Children.Add(new Ellipse { Width = 10, Height = 10, Fill = StateBrush(_selectedDetails.State), VerticalAlignment = VerticalAlignment.Center }); status.Children.Add(Text(StateText(_selectedDetails.State), 16, false, StateBrush(_selectedDetails.State))); status.Children.Add(Text("|", 16, false, Brush("WireHushMutedBrush"))); status.Children.Add(Text(_selectedDetails.State == "connected" ? Uptime(_selectedDetails.StartedAtUtc) : _selectedDetails.State == "unknown" ? "Networking state unavailable" : StateText(_selectedDetails.State), 16, false, Brush("WireHushMutedBrush"))); title.Children.Add(status); heading.Children.Add(title);
        var actionText = _tunnelActionPending ? _selectedDetails.State == "connected" ? "Disconnecting…" : "Connecting…" : _selectedDetails.State == "connected" ? "Disconnect" : "Connect";
        var toggle = ActionButton(actionText, _selectedDetails.State == "connected" ? "\uE71A" : "\uE768", _selectedDetails.State != "connected", async (_, _) => await ToggleTunnelAsync()); toggle.Tag = "action:connect"; toggle.IsEnabled = (_selectedDetails.State == "connected" || _managerClient?.DeviceBusyForAnotherUser != true) && _managerClient?.Connected == true && !_tunnelActionPending && _selectedDetails.State is ("connected" or "disconnected"); toggle.Width = 255; toggle.Height = 68; Grid.SetColumn(toggle, 1); heading.Children.Add(toggle); stack.Children.Add(heading);
        var columns = RootGrid.ActualWidth < 1400 ? 2 : 4; var tiles = new Grid(); for (var i = 0; i < columns; i++) tiles.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) }); for (var i = 0; i < (4 + columns - 1) / columns; i++) tiles.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        var values = new[] { ("VPN IP (IPv4)", _selectedDetails.Ipv4Addresses.Count == 0 ? "Unavailable" : _selectedDetails.Ipv4Addresses[0]), ("VPN IP (IPv6)", _selectedDetails.Ipv6Addresses.Count == 0 ? "Unavailable" : _selectedDetails.Ipv6Addresses[0]), ("Endpoint", string.IsNullOrWhiteSpace(_selectedDetails.EndpointDisplay) ? "Unavailable" : _selectedDetails.EndpointDisplay), ("Latest Handshake", _selectedDetails.LatestHandshakeUtc is null ? "Not reported" : RelativeTime(_selectedDetails.LatestHandshakeUtc)) };
        for (var i = 0; i < values.Length; i++) { var tile = new StackPanel { Spacing = 6, Padding = new Thickness(12, 8, 12, 8) }; tile.Children.Add(Text(values[i].Item1, 14, false, Brush("WireHushMutedBrush"))); tile.Children.Add(Text(values[i].Item2, 17, true)); Grid.SetColumn(tile, i % columns); Grid.SetRow(tile, i / columns); tiles.Children.Add(tile); }
        stack.Children.Add(tiles); border.Child = stack; return border;
    }
    private UIElement EmptyDetails() { var s = new StackPanel { HorizontalAlignment = HorizontalAlignment.Center, VerticalAlignment = VerticalAlignment.Center, Spacing = 10 }; s.Children.Add(Text("No tunnel selected", 28, true)); s.Children.Add(Text("Add or import a WireGuard tunnel to begin.", 16, false, Brush("WireHushMutedBrush"))); return s; }
    private UIElement OverviewPage()
    {
        var grid = new Grid(); grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) }); grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) }); var left = new StackPanel { Spacing = 16 }; left.Children.Add(Card("Connection", new[] { ("Endpoint", Value(_selectedDetails!.EndpointDisplay)), ("Latest handshake", RelativeTime(_selectedDetails.LatestHandshakeUtc)), ("Tunnel uptime", Uptime(_selectedDetails.StartedAtUtc)), ("Interface", Value(_selectedDetails.InterfaceName)) })); left.Children.Add(Card("DNS", new[] { ("Mode", Value(_selectedDetails.Dns.Mode)), ("Resolver", Value(_selectedDetails.Dns.ServerDisplay)), ("Status", DnsStatus()) })); grid.Children.Add(left); var right = new StackPanel { Spacing = 16 }; right.Children.Add(TrafficCard()); right.Children.Add(Card("Network", new[] { ("IPv4", Join(_selectedDetails.Ipv4Addresses)), ("IPv6", Join(_selectedDetails.Ipv6Addresses)), ("Allowed IPs", _selectedDetails.MayEdit ? $"{_selectedDetails.AllowedIPs.Count} routes" : "Unavailable") })); Grid.SetColumn(right, 1); grid.Children.Add(right); return grid;
    }
    private UIElement NetworkPage() => Card("Network", new[] { ("IPv4 addresses", Join(_selectedDetails!.Ipv4Addresses)), ("IPv6 addresses", Join(_selectedDetails.Ipv6Addresses)), ("Listen port", _selectedDetails.ListenPort?.ToString() ?? "—"), ("Interface", Value(_selectedDetails.InterfaceName)), ("Endpoint", Value(_selectedDetails.EndpointDisplay)) });
    private UIElement DnsPage() { var v = new List<(string, string)> { ("Mode", Value(_selectedDetails!.Dns.Mode)), ("Resolver", Value(_selectedDetails.Dns.ServerDisplay)), ("Address family", Value(_selectedDetails.Dns.AddressFamily)), ("Tunnel readiness", DnsStatus()), ("Plain DNS fallback", _selectedDetails.Dns.Mode == "DNS over HTTPS (DoH)" ? "Disabled" : "Not applicable"), ("Bootstrap resolvers", _selectedDetails.Dns.Mode == "DNS over HTTPS (DoH)" ? "See Settings" : "Not applicable") }; return Card("DNS", v); }
    private string DnsStatus()
    {
        if (_selectedDetails!.State == "unknown") return "Networking state unavailable";
        if (_selectedDetails.State != "connected") return "Not active";
        return _selectedDetails.Dns.Mode switch
        {
            "DNS over HTTPS (DoH)" => _selectedDetails.Dns.Ready ? "Ready through tunnel" : "Encrypted DNS configured; live health unavailable",
            "DNS" => "Windows DNS configured",
            "Not configured" => "No tunnel DNS configured",
            _ => "DNS details unavailable"
        };
    }
    private UIElement PeerPage() { var s = new StackPanel { Spacing = 14 }; if (_selectedDetails!.Peers.Count == 0) s.Children.Add(Card("Peers", new[] { ("Status", "Peer details are unavailable for this account.") })); foreach (var p in _selectedDetails.Peers) s.Children.Add(Card("Peer", new[] { ("Public key", Value(p.PublicKeyDisplay)), ("Endpoint", Value(p.EndpointDisplay)), ("Latest handshake", RelativeTime(p.LatestHandshakeUtc)), ("Received", FormatBytes(p.RxBytes)), ("Sent", FormatBytes(p.TxBytes)), ("Keepalive", p.PersistentKeepaliveSeconds?.ToString() ?? "—") })); return s; }
    private UIElement AllowedPage() => Card("Allowed IPs", _selectedDetails!.AllowedIPs.Count == 0 ? new[] { ("Routes", "Route details are unavailable for this account.") } : _selectedDetails.AllowedIPs.Select((ip, i) => ($"Route {i + 1}", ip)));
    private UIElement Card(string title, IEnumerable<(string Label, string Value)> values) { var border = new Border { Style = (Style)Application.Current.Resources["WireHushCardStyle"], Padding = new Thickness(20), Margin = new Thickness(0, 0, 8, 0) }; var stack = new StackPanel { Spacing = 12 }; stack.Children.Add(Text(title, 20, true)); foreach (var (label, value) in values) { var row = new Grid(); row.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(115) }); row.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) }); row.Children.Add(Text(label, 15, false, Brush("WireHushMutedBrush"))); var vb = Text(value, 15); Grid.SetColumn(vb, 1); row.Children.Add(vb); stack.Children.Add(row); } border.Child = stack; return border; }
    private UIElement TrafficCard()
    {
        if (_selectedDetails?.MetricsAvailable != true) return Card("Traffic", new[] { ("Status", "Runtime traffic measurements are unavailable.") });
        var border = new Border { Style = (Style)Application.Current.Resources["WireHushCardStyle"], Padding = new Thickness(20), Margin = new Thickness(0, 0, 8, 0) };
        var content = new StackPanel { Spacing = 10 };
        content.Children.Add(Text("Traffic", 20, true));
        content.Children.Add(Text("Recent receive and send rate — up to five minutes", 14, false, Brush("WireHushMutedBrush")));
        var canvas = new Canvas { Height = 160, Background = new SolidColorBrush(Windows.UI.Color.FromArgb(0x30, 0x24, 0x36, 0x42)) };
        void Draw()
        {
            canvas.Children.Clear();
            var samples = _traffic.Samples;
            var width = Math.Max(0, canvas.ActualWidth);
            canvas.Clip = new RectangleGeometry { Rect = new Rect(0, 0, width, 160) };
            if (samples.Count < 2 || width == 0) return;
            var max = Math.Max(1, samples.Max(x => Math.Max(x.Rx, x.Tx)));
            var first = samples[0].Tick; var span = Math.Max(1, samples[^1].Tick - first);
            canvas.Children.Add(Graph(samples.Select(x => new Point((x.Tick - first) * width / span, 150 - x.Rx / max * 135)), Windows.UI.Color.FromArgb(0xFF, 0x04, 0xC1, 0xF3)));
            canvas.Children.Add(Graph(samples.Select(x => new Point((x.Tick - first) * width / span, 150 - x.Tx / max * 135)), Windows.UI.Color.FromArgb(0xFF, 0x17, 0xDB, 0x55)));
        }
        canvas.SizeChanged += (_, _) => Draw();
        content.Children.Add(canvas);
        var current = _traffic.Current;
        content.Children.Add(Text(current is null ? "Collecting traffic measurements…" : $"Receive {FormatRate(current.Rx)}   Send {FormatRate(current.Tx)}", 14, false, Brush("WireHushMutedBrush")));
        border.Child = content; return border;
    }
    private static string FormatBytes(ulong? value) => value is null ? "—" : FormatBytes(value.Value);
    private static Polyline Graph(IEnumerable<Point> points, Windows.UI.Color color) { var collection = new PointCollection(); foreach (var point in points) collection.Add(point); return new Polyline { Points = collection, Stroke = new SolidColorBrush(color), StrokeThickness = 2 }; }
    private void AddTrafficSample(TunnelDetails details) => _traffic.Add(details.RxBytes, details.TxBytes, Stopwatch.GetTimestamp());
    private void ResetTraffic() { _traffic.Clear(); _selectedDetails = null; }

    private async Task ToggleTunnelAsync()
    {
        if (_managerClient is null || _selectedDetails is null || _tunnelActionPending) return;
        _tunnelActionPending = true; RenderDetails();
        try { if (_selectedDetails.State == "connected") await _managerClient.StopTunnelAsync(_selectedDetails.Id, _lifetime.Token); else await _managerClient.StartTunnelAsync(_selectedDetails.Id, _lifetime.Token); await Task.Delay(500, _lifetime.Token); await LoadTunnelsAsync(); }
        catch (Exception ex) { ShowNotice(SafeError(ex)); }
        finally { _tunnelActionPending = false; RenderDetails(); }
    }
    private void ShowTunnelMenu(string name) { var menu = new MenuFlyout(); var action = new MenuFlyoutItem { Text = _tunnels.FirstOrDefault(t => t.Id == name)?.State == "connected" ? "Disconnect" : "Connect" }; action.IsEnabled = _managerClient?.Connected == true && (_tunnels.FirstOrDefault(t => t.Id == name)?.State == "connected" || _managerClient?.DeviceBusyForAnotherUser != true); action.Click += async (_, _) => { try { _selectedTunnel = name; _section = "Overview"; await LoadSelectedTunnelAsync(name); await ToggleTunnelAsync(); } catch (Exception ex) { ShowNotice(SafeError(ex)); } }; menu.Items.Add(action); var delete = new MenuFlyoutItem { Text = "Delete", Foreground = new SolidColorBrush(Colors.IndianRed) }; delete.Click += async (_, _) => await DeleteTunnelAsync(name); delete.IsEnabled = _tunnels.FirstOrDefault(t => t.Id == name)?.MayEdit == true && _managerClient?.Connected == true; menu.Items.Add(delete); var edit = new MenuFlyoutItem { Text = "Edit", IsEnabled = delete.IsEnabled }; edit.Click += async (_, _) => await EditTunnelAsync(name); menu.Items.Add(edit); var export = new MenuFlyoutItem { Text = "Export", IsEnabled = delete.IsEnabled }; export.Click += async (_, _) => await ExportTunnelAsync(name); menu.Items.Add(export); menu.ShowAt(Content as FrameworkElement); }
    private async Task DeleteTunnelAsync(string name) { if (_managerClient is null) return; var d = new ContentDialog { Title = $"Delete {_tunnels.FirstOrDefault(t => t.Id == name)?.Name}?", Content = "This removes the stored tunnel configuration.", PrimaryButtonText = "Delete", CloseButtonText = "Cancel", XamlRoot = RootGrid.XamlRoot }; if (await ShowDialogAsync(d) != ContentDialogResult.Primary) return; try { await _managerClient.DeleteTunnelAsync(name, _lifetime.Token); if (_selectedTunnel == name) _selectedTunnel = null; await LoadTunnelsAsync(); } catch (Exception ex) { ShowNotice(SafeError(ex)); } }
    private async Task AddTunnelAsync()
    {
        if (_managerClient is null) return;
        var name = new TextBox { PlaceholderText = "Tunnel name" };
        var configuration = new TextBox { PlaceholderText = "Paste a standard WireGuard configuration", AcceptsReturn = true, TextWrapping = TextWrapping.Wrap, MinHeight = 240, FontFamily = new FontFamily("Cascadia Mono") };
        var form = new StackPanel { Spacing = 10 }; form.Children.Add(name); form.Children.Add(configuration);
        var shared = new CheckBox { Content = "Shared tunnel (available to authorized users of this device)", IsEnabled = _managerClient.MayEditMachineSettings };
        form.Children.Add(Text("New tunnels are private to your Windows account by default.", 14));
        if (_managerClient.MayEditMachineSettings) form.Children.Add(shared);
        var d = new ContentDialog { Title = "Add Tunnel", Content = form, PrimaryButtonText = "Add", CloseButtonText = "Cancel", XamlRoot = RootGrid.XamlRoot };
        if (await ShowDialogAsync(d) != ContentDialogResult.Primary) return;
        try { _selectedTunnel = await _managerClient.ImportTunnelAsync(name.Text.Trim(), configuration.Text, _lifetime.Token, shared.IsChecked == true); await LoadTunnelsAsync(); }
        catch (Exception ex) { ShowNotice(SafeError(ex)); }
    }
    private async Task ImportTunnelAsync()
    {
        if (_managerClient is null || !_managerClient.Connected) return;
        var picker = new Windows.Storage.Pickers.FileOpenPicker();
        picker.FileTypeFilter.Add(".conf"); picker.FileTypeFilter.Add(".wg");
        InitializeWithWindow.Initialize(picker, WindowNative.GetWindowHandle(this));
        var files = await picker.PickMultipleFilesAsync();
        var failures = new List<string>();
        foreach (var file in files)
        {
            try
            {
                var properties = await file.GetBasicPropertiesAsync();
                if (properties.Size > 900_000) throw new InvalidOperationException("The selected configuration is too large.");
                var content = await Windows.Storage.FileIO.ReadTextAsync(file);
                _selectedTunnel = await _managerClient.ImportTunnelAsync(System.IO.Path.GetFileNameWithoutExtension(file.Name), content, _lifetime.Token);
            }
            catch (Exception ex) { failures.Add(file.Name + ": " + SafeError(ex)); }
        }
        await LoadTunnelsAsync();
        if (failures.Count > 0) ShowNotice(string.Join(Environment.NewLine, failures));
    }
    private static Button SmallButton(string text, RoutedEventHandler handler) { var b = new Button { Content = text, Padding = new Thickness(8, 4, 8, 4), Margin = new Thickness(2, 0, 0, 0) }; b.Click += handler; return b; }
    private static IEnumerable<BootstrapResolver> DefaultBootstrap() => new[] { "1.1.1.1", "1.0.0.1", "8.8.8.8", "8.8.4.4", "4.2.2.1", "4.2.2.2", "2606:4700:4700::1111", "2606:4700:4700::1001", "2001:4860:4860::8888", "2001:4860:4860::8844", "2620:fe::11", "2620:fe::fe:11" }.Select(x => new BootstrapResolver(x, true, false));
    private async Task ShowLogAsync()
    {
        if (_managerClient is null) { ShowNotice("WireHush Manager is unavailable."); return; }
        try
        {
            var lines = await _managerClient.GetLogSnapshotAsync(_lifetime.Token); var box = new TextBox { IsReadOnly = true, TextWrapping = TextWrapping.Wrap, AcceptsReturn = true, FontFamily = new FontFamily("Cascadia Mono"), MinWidth = 760, MinHeight = 380 };
            void Filter(string query) => box.Text = string.Join(Environment.NewLine, lines.Where(line => string.IsNullOrWhiteSpace(query) || line.Contains(query, StringComparison.OrdinalIgnoreCase)));
            Filter(string.Empty); var panel = new StackPanel { Spacing = 8 }; var filter = new AutoSuggestBox { PlaceholderText = "Filter displayed log…", QueryIcon = new SymbolIcon(Symbol.Find) }; filter.TextChanged += (_, _) => Filter(filter.Text); panel.Children.Add(filter); panel.Children.Add(box);
            var dialog = new ContentDialog { Title = "Diagnostics (configuration omitted)", Content = panel, PrimaryButtonText = "Copy", CloseButtonText = "Close", XamlRoot = Content.XamlRoot }; dialog.PrimaryButtonClick += (_, _) => { var package = new Windows.ApplicationModel.DataTransfer.DataPackage(); package.SetText(box.Text); Windows.ApplicationModel.DataTransfer.Clipboard.SetContent(package); }; await ShowDialogAsync(dialog);
        }
        catch (Exception ex) { ShowNotice(SafeError(ex)); }
    }
    private void ShowHelp()
    {
        _section = "Help"; var panel = new StackPanel { Spacing = 16 };
        panel.Children.Add(Card("Help", new[] { ("Connect", "Select a tunnel and choose Connect."), ("Encrypted DNS", "Use DNS = https://… in a tunnel configuration. WireHush fails closed if it cannot establish that resolver."), ("Bootstrap", "Settings controls only the IP resolvers used to reach the encrypted DNS endpoint.") }));
        panel.Children.Add(ActionButton("Back to tunnel", "\uE72B", false, (_, _) => { _section = "Overview"; RenderDetails(); }));
        panel.Children.Add(ActionButton("Exit WireHush", "\uE7E8", false, async (_, _) => await ConfirmExitAsync()));
        _detailHost!.Content = panel;
    }
    private static string SafeError(Exception error) => error is Grpc.Core.RpcException rpc ? rpc.Status.Detail : error is InvalidOperationException or TimeoutException ? error.Message : "WireHush could not complete the operation. Refresh state and retry.";
    private void ShowNotice(string message) { SetConnectionStatus(message); }
    private void SetConnectionStatus(string message)
    {
        if (_connectionStatus is not null) _connectionStatus.Text = message;
        if (_managerInfo is not null)
        {
            _managerInfo.Message = message;
            _managerInfo.Severity = message.StartsWith("Connected", StringComparison.Ordinal) ? InfoBarSeverity.Success : InfoBarSeverity.Warning;
            _managerInfo.IsOpen = !message.StartsWith("Connected", StringComparison.Ordinal);
        }
    }
    private async void OnAppWindowClosing(AppWindow sender, AppWindowClosingEventArgs args)
    {
        if (_allowClose) return;
        args.Cancel = true;
        await ConfirmExitAsync();
    }
    private async Task ConfirmExitAsync()
    {
        if (_exiting) return;
        var dialog = new ContentDialog { Title = "Exit WireHush?", Content = "If this is the last connected session, your authorized active tunnel will be disconnected before exit. Other connected sessions and private tunnels belonging to another account keep running.", PrimaryButtonText = "Exit WireHush", CloseButtonText = "Cancel", XamlRoot = RootGrid.XamlRoot };
        if (await ShowDialogAsync(dialog) == ContentDialogResult.Primary) await ExitWireHushAsync();
    }
    private async Task ExitWireHushAsync()
    {
        if (_exiting) return;
        _exiting = true; SetConnectionStatus("Closing WireHush…"); try
        {
            if (_managerClient is null || !_managerClient.Connected) { await ConnectAndLoadAsync(); }
            if (_managerClient is null || !_managerClient.Connected) throw new InvalidOperationException("Cleanup could not be verified. Keep WireHush open and retry when the Manager is available.");
            await _managerClient.ShutdownAsync(_lifetime.Token);

            UiStartupLog.Write("session-exit-complete"); _allowClose = true; Close();
        }
        catch (Exception ex) { _exiting = false; UiStartupLog.Write("session-exit-failed"); ShowNotice(SafeError(ex)); }
    }
    private void OnClosed(object sender, WindowEventArgs args) { _windowPreferences.Dispose(); _refreshTimer.Stop(); _lifetime.Cancel(); _ = _managerClient.DisposeAsyncIfPresent(); }
    private static string Join(IReadOnlyList<string> values) => values.Count == 0 ? "—" : string.Join(", ", values); private static string Value(string? value) => string.IsNullOrWhiteSpace(value) ? "—" : value; private static string RelativeTime(DateTimeOffset? time) => time is null ? "—" : time.Value > DateTimeOffset.UtcNow ? "Clock difference detected" : $"{Math.Max(0, (int)(DateTimeOffset.UtcNow - time.Value).TotalSeconds)} seconds ago"; private static string Uptime(DateTimeOffset? time) => time is null ? "—" : (DateTimeOffset.UtcNow - time.Value).ToString("d' days 'h' hours 'm' minutes'"); private static string FormatBytes(ulong value) => value < 1024 ? $"{value} B" : value < 1024 * 1024 ? $"{value / 1024d:F1} KiB" : $"{value / 1024d / 1024d:F1} MiB"; private static string FormatRate(double value) => value < 1024 ? $"{value:F0} bps" : value < 1024 * 1024 ? $"{value / 1024:F1} Kbps" : $"{value / 1024 / 1024:F1} Mbps";

}

internal static class ManagerClientExtensions { internal static async Task DisposeAsyncIfPresent(this IManagerClient? client) { if (client is not null) await client.DisposeAsync(); } }
