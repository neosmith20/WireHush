

namespace WireHush.UI.Models;

public sealed record TunnelSummary(
    string Name,
    string State, string Id, bool MayEdit, string ScopeLabel);

public sealed record PeerDetails(
    string PublicKeyDisplay,
    string PublicKey,
    string EndpointDisplay,
    IReadOnlyList<string> AllowedIPs,
    int? PersistentKeepaliveSeconds,
    DateTimeOffset? LatestHandshakeUtc,
    ulong? RxBytes,
    ulong? TxBytes);

public sealed record DnsDetails(
    string Mode,
    string ServerDisplay,
    string AddressFamily,
    IReadOnlyList<string> BootstrapResolvers,
    bool FallbackEnabled,
    bool Ready);

public sealed record TunnelDetails(
    string Name,
    string State,
    IReadOnlyList<string> Ipv4Addresses,
    IReadOnlyList<string> Ipv6Addresses,
    string EndpointDisplay,
    IReadOnlyList<string> AllowedIPs,
    int? ListenPort,
    string InterfaceName,
    DateTimeOffset? StartedAtUtc,
    DateTimeOffset? LatestHandshakeUtc,
    ulong RxBytes,
    ulong TxBytes,
    DnsDetails Dns,
    IReadOnlyList<PeerDetails> Peers, string Id, bool MayEdit, bool MetricsAvailable);

public sealed record BootstrapResolver(
    string Address,
    bool Enabled,
    bool Custom);

public sealed record ManagerError(
    string Code,
    string Message);
