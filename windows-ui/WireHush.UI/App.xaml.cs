using Microsoft.UI.Xaml;
using Microsoft.Windows.AppLifecycle;
using System.Security.Principal;
using System.Runtime.InteropServices;

namespace WireHush.UI;

public partial class App : Application
{
    private AppInstance? _instance;
    public static MainWindow? MainWindowInstance { get; private set; }

    public App()
    {
        InitializeComponent();
        UnhandledException += (_, _) => Services.UiStartupLog.Write("ui-unhandled-error");
    }

    protected override async void OnLaunched(LaunchActivatedEventArgs args)
    {
        try
        {
            using var identity = WindowsIdentity.GetCurrent();
            if (new WindowsPrincipal(identity).IsInRole(WindowsBuiltInRole.Administrator))
            {
                MessageBox(IntPtr.Zero, "Start WireHush normally, without Run as administrator. The installed Manager handles privileged operations.", "WireHush", 0x30);
                Exit();
                return;
            }
            var activation = AppInstance.GetCurrent().GetActivatedEventArgs();
            var session = System.Diagnostics.Process.GetCurrentProcess().SessionId;
            _instance = AppInstance.FindOrRegisterForKey($"WireHush.UI.Session.{session}");
            if (!_instance.IsCurrent)
            {
                await _instance.RedirectActivationToAsync(activation);
                Exit();
                return;
            }
            MainWindowInstance = new MainWindow();
            _instance.Activated += (_, _) => MainWindowInstance.DispatcherQueue.TryEnqueue(() => MainWindowInstance.Activate());
            MainWindowInstance.Closed += (_, _) => _instance.UnregisterKey();
            MainWindowInstance.Activate();
        }
        catch { Services.UiStartupLog.Write("ui-startup-failed"); Exit(); }
    }
    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    private static extern int MessageBox(IntPtr window, string text, string title, uint flags);
}
