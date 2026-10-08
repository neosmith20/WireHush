# Known limitations

- Native x64 installation, uninstall/reinstall, fixed-path installer UI, service registration, service privileges, protected ProgramData ACLs, Start Menu registration, named-pipe creation, no-manager-TCP-listener behavior, and upstream WireGuard/TunnelMint coexistence have been exercised on Windows 11 25H2 build 26200. Native ARM64 execution is still pending.
- A newly added `WireHush Users` membership requires a fresh Windows logon token before ordinary-user manager authentication can be accepted. Fresh-token group authentication and the full two-account Private/Shared authorization matrix remain owner acceptance items.
- No real VPN peer or tunnel configuration was available. Real handshake, traffic, plain DNS, DoH full-tunnel, DoH split-tunnel, packet capture, and DNS leak observation remain manual verification items.
- IPv6, sleep/wake, and Ethernet/Wi-Fi transition behavior were not exercised in this environment.
- The broad upstream `conf`, DPAPI, and adapter test suites depend on protected storage, DPAPI, or a usable Windows adapter and did not complete reliably here.
- The release-candidate executables and MSIs are unsigned development artifacts. Local WiX ICE validation runs successfully with the documented ICE03 and ICE61 exclusions; remote CI, signing, native ARM64 execution, and final publication remain pending.
- A TunnelMint tunnel currently supports one configured DoH endpoint per activation. The DNS proxy accepts IPv4 loopback by default; its implementation also supports IPv6 loopback where the host configuration uses it.
