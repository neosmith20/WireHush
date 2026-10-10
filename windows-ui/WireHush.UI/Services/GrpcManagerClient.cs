using Grpc.Core;
using Grpc.Net.Client;
using WireHush.UI.Models;
using P = WireHush.Protocol;

namespace WireHush.UI.Services;

internal sealed class GrpcManagerClient : IManagerClient
{
    private readonly CancellationTokenSource _lifetime = new();
    private readonly object _gate = new();
    private GrpcChannel? _channel;
    private P.Manager.ManagerClient? _rpc;
    private P.SnapshotReply _snapshot = new();
    private Task? _events;
    private volatile bool _connected;
    public bool Connected => _connected;
    public bool DeviceBusyForAnotherUser { get { lock (_gate) return _snapshot.DeviceBusyForAnotherUser; } }
    private string _managerVersion = "Unavailable";
    public bool MayEditMachineSettings { get; private set; }
    public event Action? SnapshotChanged;
    public event Action? ConnectionLost;
    private P.Manager.ManagerClient Rpc => _rpc ?? throw new InvalidOperationException("WireHush Manager is unavailable.");
    private static DateTime Deadline => DateTime.UtcNow.AddSeconds(10);

    public async Task ConnectAsync(CancellationToken cancellationToken)
    {
        _channel = P.LocalManagerChannel.Create(WireHushService.CurrentProcessId);
        _rpc = new P.Manager.ManagerClient(_channel);
        var handshake = await Rpc.HandshakeAsync(new P.HandshakeRequest { ProtocolMajor = 1, ProtocolMinor = 0 }, deadline: Deadline, cancellationToken: cancellationToken);
        if (handshake.ProtocolMajor != 1) throw new InvalidOperationException("The installed UI and Manager versions do not match.");
        foreach (var capability in new[] { P.Capability.Tunnels, P.Capability.EncryptedDns, P.Capability.BootstrapSettings, P.Capability.Events, P.Capability.Export, P.Capability.SessionExit })
            if (!handshake.Capabilities.Contains(capability)) throw new InvalidOperationException("Install matching UI and Manager versions to use the required V1 features.");
        _managerVersion = handshake.ProductVersion.Length <= 128 && System.Text.RegularExpressions.Regex.IsMatch(handshake.ProductVersion, @"^\d+\.\d+\.\d+(\+(?:[0-9a-f]{40}|unstamped))?$") ? handshake.ProductVersion : "Unavailable";
        MayEditMachineSettings = handshake.MayEditMachineSettings;
        var snapshot = await Rpc.SnapshotAsync(new P.Empty(), deadline: Deadline, cancellationToken: cancellationToken);
        lock (_gate) _snapshot = snapshot;
        _connected = true;
        _events = ReceiveEventsAsync();
    }

    private async Task ReceiveEventsAsync()
    {
        try
        {
            using var subscription = Rpc.Subscribe(new P.Empty(), cancellationToken: _lifetime.Token);
            while (await subscription.ResponseStream.MoveNext(_lifetime.Token).ConfigureAwait(false))
            {
                var update = subscription.ResponseStream.Current;
                lock (_gate)
                {
                    if (update.InstanceId != _snapshot.InstanceId || update.Revision > _snapshot.Revision) _snapshot = update;
                }
                SnapshotChanged?.Invoke();
            }
        }
        catch (Exception) when (_lifetime.IsCancellationRequested) { return; }
        catch { UiStartupLog.Write("manager-unavailable"); }
        _connected = false;
        lock (_gate)
        {
            var unavailable = _snapshot.Clone();
            foreach (var tunnel in unavailable.Tunnels)
            {
                tunnel.State = P.TunnelState.Unknown;
                tunnel.ClearRxBytes(); tunnel.ClearTxBytes(); tunnel.ClearLatestHandshakeUnix(); tunnel.ClearDnsReady();
            }
            _snapshot = unavailable;
        }
        ConnectionLost?.Invoke();
    }

    private static string State(P.TunnelState state) => state switch
    {
        P.TunnelState.Connected => "connected", P.TunnelState.Stopped => "disconnected",
        P.TunnelState.Starting => "connecting", P.TunnelState.Stopping => "disconnecting", _ => "unknown"
    };
    private P.TunnelSnapshot Find(string id)
    {
        lock (_gate) return _snapshot.Tunnels.FirstOrDefault(t => t.Tunnel.TunnelId == id)?.Clone() ?? throw new InvalidOperationException("The selected tunnel is unavailable. Refresh the list.");
    }
    public Task<IReadOnlyList<TunnelSummary>> ListTunnelsAsync(CancellationToken cancellationToken)
    {
        cancellationToken.ThrowIfCancellationRequested();
        lock (_gate) return Task.FromResult<IReadOnlyList<TunnelSummary>>(_snapshot.Tunnels.Select(t => new TunnelSummary(t.Name, State(t.State), t.Tunnel.TunnelId, t.MayEdit, t.Tunnel.Scope == P.Scope.Private ? "Private" : "Shared")).ToArray());
    }
    public Task<TunnelDetails> GetTunnelAsync(string id, CancellationToken cancellationToken) => GetRuntimeAsync(id, cancellationToken);
    public Task<TunnelDetails> GetRuntimeAsync(string id, CancellationToken cancellationToken)
    {
        cancellationToken.ThrowIfCancellationRequested();
        var tunnel = Find(id);
        // Metadata views never silently export private keys or privileged shared
        // configuration. Unreported values remain unavailable rather than guessed.
        var network = tunnel.Network;
        var peers = network?.Peers.Select(p => new PeerDetails(p.PublicKey, p.PublicKey, p.EndpointDisplay, p.AllowedIps.ToArray(),
            p.HasKeepaliveSeconds ? (int)p.KeepaliveSeconds : null,
            p.HasLatestHandshakeUnix ? DateTimeOffset.FromUnixTimeSeconds(p.LatestHandshakeUnix) : null, p.HasRxBytes ? p.RxBytes : null, p.HasTxBytes ? p.TxBytes : null)).ToArray() ?? [];
        var details = new TunnelDetails(tunnel.Name, State(tunnel.State), network?.Ipv4Addresses.ToArray() ?? [], network?.Ipv6Addresses.ToArray() ?? [],
            network?.EndpointDisplay ?? "", network?.AllowedIps.ToArray() ?? [], network is { HasListenPort: true } ? (int)network.ListenPort : null, network?.InterfaceName ?? "", null,
            tunnel.HasLatestHandshakeUnix ? DateTimeOffset.FromUnixTimeSeconds(tunnel.LatestHandshakeUnix) : null,
            tunnel.RxBytes, tunnel.TxBytes,
            new DnsDetails(tunnel.EncryptedDns ? "DNS over HTTPS (DoH)" : network is null ? "Unavailable" : network.DnsServers.Count > 0 ? "DNS" : "Not configured", string.Join(", ", network?.DnsServers.ToArray() ?? []), "", [], false, tunnel.HasDnsReady && tunnel.DnsReady),
            peers, id, tunnel.MayEdit, tunnel.HasRxBytes && tunnel.HasTxBytes);
        return Task.FromResult(details);
    }
    private async Task RefreshAsync(CancellationToken cancellationToken)
    {
        var snapshot = await Rpc.SnapshotAsync(new P.Empty(), deadline: Deadline, cancellationToken: cancellationToken);
        lock (_gate) _snapshot = snapshot;
    }
    public async Task StartTunnelAsync(string id, CancellationToken cancellationToken) { await Rpc.StartTunnelAsync(Find(id).Tunnel, deadline: Deadline, cancellationToken: cancellationToken); await RefreshAsync(cancellationToken); }
    public async Task StopTunnelAsync(string id, CancellationToken cancellationToken) { await Rpc.StopTunnelAsync(Find(id).Tunnel, deadline: Deadline, cancellationToken: cancellationToken); await RefreshAsync(cancellationToken); }
    public async Task DeleteTunnelAsync(string id, CancellationToken cancellationToken) { await Rpc.DeleteTunnelAsync(Find(id).Tunnel, deadline: Deadline, cancellationToken: cancellationToken); await RefreshAsync(cancellationToken); }
    public async Task<string> ImportTunnelAsync(string name, string content, CancellationToken cancellationToken, bool shared = false)
    {
        var created = await Rpc.CreateTunnelAsync(new P.CreateTunnelRequest { Name = name, Scope = shared ? P.Scope.Shared : P.Scope.Private, WgQuickText = content }, deadline: Deadline, cancellationToken: cancellationToken);
        await RefreshAsync(cancellationToken);
        return created.Tunnel.TunnelId;
    }
    public async Task<string> ExportAsync(string id, CancellationToken cancellationToken) => (await Rpc.ReadConfigurationAsync(Find(id).Tunnel, deadline: Deadline, cancellationToken: cancellationToken)).WgQuickText;
    public async Task UpdateAsync(string id, string name, string configuration, CancellationToken cancellationToken)
    {
        await Rpc.UpdateTunnelAsync(new P.UpdateTunnelRequest { Tunnel = Find(id).Tunnel, Name = name, WgQuickText = configuration }, deadline: Deadline, cancellationToken: cancellationToken);
        await RefreshAsync(cancellationToken);
    }
    public async Task<IReadOnlyList<BootstrapResolver>> GetBootstrapAsync(CancellationToken cancellationToken) => (await Rpc.ReadBootstrapAsync(new P.Empty(), deadline: Deadline, cancellationToken: cancellationToken)).Resolvers.Select(r => new BootstrapResolver(r.Address, r.Enabled, r.Custom)).ToArray();
    public async Task SaveBootstrapAsync(IReadOnlyList<BootstrapResolver> resolvers, CancellationToken cancellationToken)
    {
        var settings = new P.BootstrapSettings();
        settings.Resolvers.AddRange(resolvers.Select(r => new P.BootstrapResolver { Address = r.Address, Enabled = r.Enabled, Custom = r.Custom }));
        await Rpc.SaveBootstrapAsync(settings, deadline: Deadline, cancellationToken: cancellationToken);
    }
    public Task<IReadOnlyList<string>> GetLogSnapshotAsync(CancellationToken cancellationToken)
    {
        cancellationToken.ThrowIfCancellationRequested();
        lock (_gate) return Task.FromResult<IReadOnlyList<string>>([$"Time (UTC): {DateTimeOffset.UtcNow:O}", $"WireHush UI: {typeof(App).Assembly.GetCustomAttributes(typeof(System.Reflection.AssemblyInformationalVersionAttribute), false).OfType<System.Reflection.AssemblyInformationalVersionAttribute>().FirstOrDefault()?.InformationalVersion ?? "Unstamped"}", $"WireHush Manager: {_managerVersion}", $"Current manager instance: {_snapshot.InstanceId}", "Evidence: current authenticated WireHush session only; historical crash logs are not included.", "Protocol: 1.0", $"Runtime: {System.Runtime.InteropServices.RuntimeInformation.FrameworkDescription}", $"Architecture: {System.Runtime.InteropServices.RuntimeInformation.ProcessArchitecture}", $"Authenticated manager connection: {Connected}", $"Visible tunnel count: {_snapshot.Tunnels.Count}", $"Manager closing: {_snapshot.ManagerClosing}", "Tunnel identities, configuration, addresses, endpoints and owners are omitted."]);
    }
    public async Task ShutdownAsync(CancellationToken cancellationToken)
    {
        var reply = await Rpc.ExitSessionAsync(new P.Empty(), deadline: Deadline, cancellationToken: cancellationToken);
        if (!reply.CleanupComplete) throw new InvalidOperationException("Networking cleanup is incomplete. Keep WireHush open and retry.");
    }
    public async ValueTask DisposeAsync()
    {
        _lifetime.Cancel();
        _channel?.Dispose();
        if (_events is not null) await _events.ConfigureAwait(false);
        _lifetime.Dispose();
    }
}
