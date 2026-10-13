namespace WireHush.UI.Services;

internal enum TunnelHealth
{
    Disconnected,
    Starting,
    AwaitingHandshake,
    RecentHandshake,
    StaleHandshake,
    ClockDifference,
    Disconnecting,
    Unknown
}

/// <summary>
/// WireGuard reports a running tunnel interface separately from its peer's
/// most recent handshake. Neither a running interface nor a handshake proves
/// that all internet destinations are reachable.
/// </summary>
internal static class TunnelHealthStatus
{
    internal static readonly TimeSpan RecentHandshakeWindow = TimeSpan.FromMinutes(3);

    internal static TunnelHealth Classify(
        string state, DateTimeOffset? latestHandshakeUtc, DateTimeOffset utcNow)
    {
        if (state == "connecting") return TunnelHealth.Starting;
        if (state == "disconnecting") return TunnelHealth.Disconnecting;
        if (state == "disconnected") return TunnelHealth.Disconnected;
        if (state != "connected") return TunnelHealth.Unknown;
        if (latestHandshakeUtc is null) return TunnelHealth.AwaitingHandshake;
        var age = utcNow - latestHandshakeUtc.Value;
        if (age < TimeSpan.FromSeconds(-30)) return TunnelHealth.ClockDifference;
        if (age > RecentHandshakeWindow) return TunnelHealth.StaleHandshake;
        return TunnelHealth.RecentHandshake;
    }

    internal static string Label(TunnelHealth state) => state switch
    {
        TunnelHealth.AwaitingHandshake => "Tunnel active — awaiting handshake",
        TunnelHealth.RecentHandshake => "Tunnel active — recent handshake",
        TunnelHealth.StaleHandshake => "Tunnel active — last handshake is stale",
        TunnelHealth.ClockDifference => "Tunnel active — clock difference detected",
        TunnelHealth.Starting => "Starting tunnel",
        TunnelHealth.Disconnecting => "Disconnecting",
        TunnelHealth.Disconnected => "Disconnected",
        _ => "Status unavailable"
    };
}
