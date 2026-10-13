using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using WinRT.Interop;
using WireHush.UI.Models;

namespace WireHush.UI;

public sealed partial class MainWindow
{
    private readonly SemaphoreSlim _dialogGate = new(1, 1);
    private async Task<ContentDialogResult> ShowDialogAsync(ContentDialog dialog)
    {
        // A repeated close/settings/menu click cannot open overlapping dialogs
        // and crash the process before networking cleanup is acknowledged.
        if (!await _dialogGate.WaitAsync(0)) return ContentDialogResult.None;
        try
        {
            // Keep every auxiliary surface inside this window's Xaml root. This makes
            // it follow WireHush when the main window is moved and prevents each dialog
            // from choosing an unrelated desktop position.
            dialog.XamlRoot ??= RootGrid.XamlRoot;
            dialog.RequestedTheme = ElementTheme.Dark;
            dialog.FullSizeDesired = false;
            // WinUI's default ContentDialog template places an in-root dialog at
            // the top-left even when its alignment is Center. Position after its
            // first measure so all auxiliary dialogs remain centered as this
            // window moves or resizes.
            dialog.HorizontalAlignment = HorizontalAlignment.Left;
            dialog.VerticalAlignment = VerticalAlignment.Top;
            dialog.Loaded += (_, _) =>
            {
                if (dialog.ActualWidth <= 0 || dialog.ActualHeight <= 0) return;
                dialog.Margin = new Thickness(
                    Math.Max(0, (RootGrid.ActualWidth - dialog.ActualWidth) / 2),
                    Math.Max(0, (RootGrid.ActualHeight - dialog.ActualHeight) / 2), 0, 0);
            };
            dialog.Resources["ContentDialogMaxWidth"] = Math.Min(780, Math.Max(320, RootGrid.ActualWidth - 64));
            dialog.MaxWidth = Math.Min(780, Math.Max(320, RootGrid.ActualWidth - 64));
            dialog.MaxHeight = Math.Min(720, Math.Max(320, RootGrid.ActualHeight - 64));
            return await dialog.ShowAsync(ContentDialogPlacement.Popup);
        }
        finally { _dialogGate.Release(); }
    }
    private async Task EditTunnelAsync(string id)
    {
        if (_managerClient is null || _tunnels.FirstOrDefault(t => t.Id == id)?.MayEdit != true) return;
        try
        {
            // Populate from the manager's current stored record before showing the
            // dialog. Runtime snapshots omit private fields and must never seed an editor.
            var stored = await _managerClient.GetEditableTunnelAsync(id, _lifetime.Token);
            var name = new TextBox { Header = "Tunnel name" };
            var config = new TextBox { Header = "Configuration (contains private keys)", AcceptsReturn = true, TextWrapping = TextWrapping.Wrap, MinHeight = 240, MaxHeight = 380 };
            var error = new InfoBar { Severity = InfoBarSeverity.Error };
            var form = new StackPanel { Spacing = 12, MinWidth = 440 }; form.Children.Add(name); form.Children.Add(config); form.Children.Add(Text("Saving an active tunnel disconnects it, applies your changes, then reconnects. A disconnected tunnel stays disconnected.", 14)); form.Children.Add(error);
            // Assign after the controls are part of the dialog content so WinUI
            // renders the authoritative stored values rather than empty defaults.
            name.Text = stored.Name;
            config.Text = stored.Configuration;
            var dialog = new ContentDialog { Title = "Edit tunnel", Content = form, PrimaryButtonText = "Save", CloseButtonText = "Cancel", XamlRoot = RootGrid.XamlRoot };
            dialog.PrimaryButtonClick += async (_, args) =>
            {
                var deferral = args.GetDeferral();
                try { await _managerClient.UpdateAsync(id, name.Text.Trim(), config.Text, _lifetime.Token); }
                catch (Exception ex) { args.Cancel = true; error.Message = SafeError(ex); error.IsOpen = true; }
                finally { deferral.Complete(); }
            };
            await ShowDialogAsync(dialog);
            config.Text = "";
            await LoadTunnelsAsync();
        }
        catch (Exception ex) { ShowNotice(SafeError(ex)); }
    }

    private async Task ExportTunnelAsync(string id)
    {
        if (_managerClient is null) return;
        try
        {
            var warning = new ContentDialog { Title = "Export configuration?", Content = "The exported file contains private keys. Save it in a location you trust and keep it protected.", PrimaryButtonText = "Choose location", CloseButtonText = "Cancel", XamlRoot = RootGrid.XamlRoot };
            if (await ShowDialogAsync(warning) != ContentDialogResult.Primary) return;
            var picker = new Windows.Storage.Pickers.FileSavePicker { SuggestedFileName = _tunnels.First(t => t.Id == id).Name };
            picker.FileTypeChoices.Add("WireGuard configuration", [".conf"]);
            InitializeWithWindow.Initialize(picker, WindowNative.GetWindowHandle(this));
            var file = await picker.PickSaveFileAsync();
            if (file is not null) await Windows.Storage.FileIO.WriteTextAsync(file, await _managerClient.ExportAsync(id, _lifetime.Token));
        }
        catch (Exception ex) { ShowNotice(SafeError(ex)); }
    }

    private async Task ShowSettingsAsync()
    {
        if (_managerClient is null || !_managerClient.Connected) { ShowNotice("WireHush Manager is unavailable."); return; }
        try
        {
            var editable = _managerClient.MayEditMachineSettings;
            var entries = (await _managerClient.GetBootstrapAsync(_lifetime.Token)).ToList();
            var rows = new StackPanel { Spacing = 12 };
            var error = new InfoBar { Severity = InfoBarSeverity.Error };
            var panel = new StackPanel { Spacing = 16, MinWidth = 520 };
            panel.Children.Add(Text("Bootstrap resolvers", 20, true));
            panel.Children.Add(Text(editable ? "These IP addresses are used only while establishing encrypted DNS. Changes are saved for the whole device." : "An administrator can change these device settings.", 14, false, Brush("WireHushMutedBrush")));
            panel.Children.Add(rows); panel.Children.Add(error);
            void Render()
            {
                rows.Children.Clear();
                for (var index = 0; index < entries.Count; index++)
                {
                    var i = index;
                    var row = new Grid { ColumnSpacing = 8 };
                    row.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(32) });
                    row.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
                    row.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
                    row.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto });
                    var enabled = new CheckBox { IsChecked = entries[i].Enabled, IsEnabled = editable, VerticalAlignment = VerticalAlignment.Center };
                    ToolTipService.SetToolTip(enabled, "Use this resolver");
                    enabled.Checked += (_, _) => entries[i] = entries[i] with { Enabled = true };
                    enabled.Unchecked += (_, _) => entries[i] = entries[i] with { Enabled = false };
                    var address = new TextBox { Text = entries[i].Address, MinWidth = 320, IsReadOnly = !editable || !entries[i].Custom, Header = entries[i].Custom ? "Custom resolver IP" : "Default resolver IP" };
                    address.TextChanged += (_, _) => entries[i] = entries[i] with { Address = address.Text.Trim() };
                    row.Children.Add(enabled); Grid.SetColumn(address, 1); row.Children.Add(address);
                    if (editable)
                    {
                        var up = SmallButton("↑", (_, _) => { if (i > 0) { (entries[i - 1], entries[i]) = (entries[i], entries[i - 1]); Render(); } });
                        Grid.SetColumn(up, 2); row.Children.Add(up);
                        var down = SmallButton("↓", (_, _) => { if (i + 1 < entries.Count) { (entries[i + 1], entries[i]) = (entries[i], entries[i + 1]); Render(); } });
                        Grid.SetColumn(down, 3); row.Children.Add(down);
                        if (entries[i].Custom) { var remove = SmallButton("Remove", (_, _) => { entries.RemoveAt(i); Render(); }); Grid.SetColumn(remove, 4); row.ColumnDefinitions.Add(new ColumnDefinition { Width = GridLength.Auto }); row.Children.Add(remove); }
                    }
                    rows.Children.Add(row);
                }
                if (editable) rows.Children.Add(SmallButton("Add custom IP", (_, _) => { if (entries.Count < 64) { entries.Add(new BootstrapResolver("", true, true)); Render(); } }));
            }
            Render();
            var dialog = new ContentDialog { Title = "Settings", Content = new ScrollViewer { Content = panel, MaxHeight = 500, HorizontalScrollBarVisibility = ScrollBarVisibility.Disabled, VerticalScrollBarVisibility = ScrollBarVisibility.Auto }, PrimaryButtonText = editable ? "Save" : "", SecondaryButtonText = editable ? "Restore defaults" : "", CloseButtonText = "Close", XamlRoot = RootGrid.XamlRoot };
            dialog.SecondaryButtonClick += (_, args) => { args.Cancel = true; entries = DefaultBootstrap().ToList(); Render(); };
            dialog.PrimaryButtonClick += async (_, args) =>
            {
                var deferral = args.GetDeferral();
                try { await _managerClient.SaveBootstrapAsync(entries, _lifetime.Token); }
                catch (Exception ex) { args.Cancel = true; error.Message = SafeError(ex); error.IsOpen = true; }
                finally { deferral.Complete(); }
            };
            await ShowDialogAsync(dialog);
        }
        catch (Exception ex) { ShowNotice(SafeError(ex)); }
    }
}
