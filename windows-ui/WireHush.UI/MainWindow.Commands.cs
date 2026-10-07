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
            dialog.MaxWidth = Math.Min(900, Math.Max(320, RootGrid.ActualWidth - 80));
            dialog.MaxHeight = Math.Min(720, Math.Max(320, RootGrid.ActualHeight - 80));
            return await dialog.ShowAsync();
        }
        finally { _dialogGate.Release(); }
    }
    private async Task EditTunnelAsync(string id)
    {
        if (_managerClient is null || _tunnels.FirstOrDefault(t => t.Id == id)?.MayEdit != true) return;
        try
        {
            var name = new TextBox { Text = _tunnels.First(t => t.Id == id).Name, Header = "Tunnel name" };
            var config = new TextBox { Text = await _managerClient.ExportAsync(id, _lifetime.Token), Header = "Configuration (contains private keys)", AcceptsReturn = true, TextWrapping = TextWrapping.Wrap, MinHeight = 240, MaxHeight = 380 };
            var error = new InfoBar { Severity = InfoBarSeverity.Error };
            var form = new StackPanel { Spacing = 12, MinWidth = 440 }; form.Children.Add(name); form.Children.Add(config); form.Children.Add(error);
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
            var rows = new StackPanel { Spacing = 10 };
            var error = new InfoBar { Severity = InfoBarSeverity.Error };
            var panel = new StackPanel { Spacing = 12, MinWidth = 460 };
            panel.Children.Add(Text("Bootstrap resolvers", 20, true));
            panel.Children.Add(Text(editable ? "These IP addresses are used only while establishing encrypted DNS. Changes are saved for the whole device." : "An administrator can change these device settings.", 14));
            panel.Children.Add(rows); panel.Children.Add(error);
            void Render()
            {
                rows.Children.Clear();
                for (var index = 0; index < entries.Count; index++)
                {
                    var i = index;
                    var row = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 8 };
                    var enabled = new CheckBox { IsChecked = entries[i].Enabled, IsEnabled = editable, VerticalAlignment = VerticalAlignment.Center };
                    enabled.Checked += (_, _) => entries[i] = entries[i] with { Enabled = true };
                    enabled.Unchecked += (_, _) => entries[i] = entries[i] with { Enabled = false };
                    var address = new TextBox { Text = entries[i].Address, Width = 250, IsReadOnly = !editable || !entries[i].Custom, Header = entries[i].Custom ? "Custom IP" : "Default IP" };
                    address.TextChanged += (_, _) => entries[i] = entries[i] with { Address = address.Text.Trim() };
                    row.Children.Add(enabled); row.Children.Add(address);
                    if (editable)
                    {
                        row.Children.Add(SmallButton("↑", (_, _) => { if (i > 0) { (entries[i-1], entries[i]) = (entries[i], entries[i-1]); Render(); } }));
                        row.Children.Add(SmallButton("↓", (_, _) => { if (i+1 < entries.Count) { (entries[i+1], entries[i]) = (entries[i], entries[i+1]); Render(); } }));
                        if (entries[i].Custom) row.Children.Add(SmallButton("Remove", (_, _) => { entries.RemoveAt(i); Render(); }));
                    }
                    rows.Children.Add(row);
                }
                if (editable) rows.Children.Add(SmallButton("Add custom IP", (_, _) => { if (entries.Count < 64) { entries.Add(new BootstrapResolver("", true, true)); Render(); } }));
            }
            Render();
            var dialog = new ContentDialog { Title = "Settings", Content = new ScrollViewer { Content = panel, MaxHeight = 480 }, PrimaryButtonText = editable ? "Save" : "", SecondaryButtonText = editable ? "Restore defaults" : "", CloseButtonText = "Close", XamlRoot = RootGrid.XamlRoot };
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
