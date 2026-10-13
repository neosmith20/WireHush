# WireHush code-quality baseline exceptions

This is an engineering record, not a clean bill of health. Exceptions remain release-review items. CI must not hide unexpected analyzer failures.

## Go vet: inherited Windows unsafe-pointer diagnostics

The inherited WireGuard Windows code has three `go vet` `unsafeptr` findings:

- `conf/dnsresolver_windows.go` (line near 72)
- `manager/service.go` (line near 320)
- `manager/servicewatcher.go` (line near 47)

The lint gate runs **all other go vet analyzers** via `go vet -unsafeptr=false`. This is a narrow, explicit exception for native Windows pointer manipulation, **not a claim that it is proven safe**. The Windows pointer transitions need a manual audit before final release acceptance. Remove the exception once those patterns can be safely refactored.

## Staticcheck scope

The mandatory Go correctness gate runs Staticcheck 2026.2.1 with `SA*` diagnostics. Known inherited or compatibility-sensitive non-SA findings from a full default scan include `ST1005` (capitalization in Windows error messages) and `U1000` (unused legacy Windows declarations). They are recorded as clean-up debt rather than quietly labeled clean. The full tool can be run manually with:

```powershell
& .\.deps\gopath\bin\staticcheck.exe ./manager ./protocol ./conf ./dnsproxy ./doh ./dohruntime ./bootstrap ./product ./tunnel ./tunnel/firewall
```

## Remaining runtime and supply-chain checks

Static analysis does not establish that the UI scrolls correctly or a tunnel is safe on real networks. Release sign-off separately requires real Windows native x64/ARM64, multiple DPI factors, active-tunnel close preference, secure IPC boundary, firewall/DNS leak tests, sleep/wake, IPv6, and actual endpoint failure recovery. An unsigned installer is not authenticated by code-quality tests.

These exceptions must be actively reviewed; never remove the text simply to make a dashboard green.
