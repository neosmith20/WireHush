using System.Diagnostics;
using WireHush.UI.Services;

static long Tick(double seconds) => (long)(seconds * Stopwatch.Frequency);
static void Require(bool condition, string message) { if (!condition) throw new Exception(message); }
Require(ConfigurationText.NormalizeLineEndings("a\rb\r\nc\nd") == "a\nb\nc\nd", "Configuration text must normalize WinUI CR-only line endings before manager validation.");
var history = new TrafficHistory();
history.Add(100, 50, Tick(0));
Require(history.Current is null, "A single counter reading cannot establish a rate.");
history.Add(110, 70, Tick(1));
Require(history.Current is { Rx: 80, Tx: 160 }, "Measured rates must use elapsed time and counter differences.");
history.Add(1, 1, Tick(2));
Require(history.Current is null, "A restarted counter must clear earlier history.");
history.Add(2, 3, Tick(3));
Require(history.Current is { Rx: 8, Tx: 16 }, "A fresh baseline must establish the new rate.");
history.Add(2, 3, Tick(3.1));
Require(history.Samples.Count == 1, "A repeated burst reading must not create a misleading short-interval rate.");
history.Add(3, 4, Tick(4));
Require(history.Current is { Rx: 8, Tx: 8 }, "Ignored burst readings must not reset the measurement baseline.");
history.Add(15, 25, Tick(301));
history.Add(20, 30, Tick(304));
Require(history.Samples.All(point => point.Tick >= Tick(4)), "History must expire by time, not only by sample count.");
history.Add(40, 50, Tick(1000));
Require(history.Current is null, "A long observation gap must not be presented as live traffic.");
history.Add(50, 60, Tick(1001));
Require(history.Current is { Rx: 80, Tx: 80 }, "Measurement must recover after a gap.");
history.Add(100, 100, Tick(999));
Require(history.Current is null, "A non-monotonic observation must not produce a bogus rate.");
history.Clear();
Require(history.Samples.Count == 0, "Selection or connection loss must clear history.");
for (var i = 0; i < 2000; i++) history.Add((ulong)i, (ulong)i, Tick(i * .25));
Require(history.Samples.Count == 600, "High-frequency readings must remain bounded.");
Require(history.Samples.All(point => point.Tick >= Tick(1999 * .25 - 300)), "Bounded history must contain only recent measurements.");
Console.WriteLine("PASS: measured traffic rates, restart/burst/gap handling, monotonic timing, bounded history, and no fabricated first rate.");

var current = new DateTimeOffset(2026, 10, 13, 12, 0, 0, TimeSpan.Zero);
Require(TunnelHealthStatus.Classify("connected", null, current) == TunnelHealth.AwaitingHandshake, "Tunnel startup must not imply a completed handshake.");
Require(TunnelHealthStatus.Classify("connected", current.AddMinutes(-1), current) == TunnelHealth.RecentHandshake, "Recent actual handshake must be recognized.");
Require(TunnelHealthStatus.Classify("connected", current.AddMinutes(-4), current) == TunnelHealth.StaleHandshake, "Old peer handshake must not remain green forever.");
Require(TunnelHealthStatus.Classify("connected", current.AddMinutes(2), current) == TunnelHealth.ClockDifference, "Future timestamps must reveal clock skew.");
Require(TunnelHealthStatus.Classify("disconnected", current, current) == TunnelHealth.Disconnected, "Old handshake must not override stopped tunnel state.");
Require(TunnelHealthStatus.Classify("connected", current.AddMinutes(-3), current) == TunnelHealth.RecentHandshake, "Boundary 3m handshake is still recent.");
Require(TunnelHealthStatus.Classify("connected", current.AddMinutes(-3).AddTicks(-1), current) == TunnelHealth.StaleHandshake, "Older-than-3m handshake is stale.");
Console.WriteLine("PASS: handshake-aware statuses distinguish awaiting, recent, stale, clock skew and stopped states.");


// Validate close-button preferences using a unique temporary profile location.
var preferenceFolder = Path.Combine(Path.GetTempPath(), "WireHush-pref-" + Guid.NewGuid().ToString("N"));
var preferenceFile = Path.Combine(preferenceFolder, "ui-preferences.json");
try
{
    Require(UserWindowBehavior.LoadAt(preferenceFile) == UserWindowBehavior.CloseAction.MinimizeToTray, "Close must default to tray.");
    UserWindowBehavior.SaveAt(preferenceFile, UserWindowBehavior.CloseAction.Exit);
    Require(UserWindowBehavior.LoadAt(preferenceFile) == UserWindowBehavior.CloseAction.Exit, "Exit choice should persist.");
    UserWindowBehavior.SaveAt(preferenceFile, UserWindowBehavior.CloseAction.MinimizeToTray);
    Require(UserWindowBehavior.LoadAt(preferenceFile) == UserWindowBehavior.CloseAction.MinimizeToTray, "Tray choice should persist.");
    File.WriteAllText(preferenceFile, "invalid json");
    Require(UserWindowBehavior.LoadAt(preferenceFile) == UserWindowBehavior.CloseAction.MinimizeToTray, "Invalid preferences must safely default to tray.");
    File.WriteAllText(preferenceFile, new string('a', 1024));
    Require(UserWindowBehavior.LoadAt(preferenceFile) == UserWindowBehavior.CloseAction.MinimizeToTray, "Oversized preferences must safely default to tray.");
    Require(!Directory.GetFiles(preferenceFolder, "*.tmp").Any(), "Saved preferences must leave no temporary files.");
    Console.WriteLine("PASS: X-button choice defaults to tray, persists, and handles corrupt settings.");
}
finally { if (Directory.Exists(preferenceFolder)) Directory.Delete(preferenceFolder, true); }
