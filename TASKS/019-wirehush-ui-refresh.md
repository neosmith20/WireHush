# Task019 – Implement The Approved WireHush UI, Not A Recolored WireGuard Layout

## Owner Status: Current Implementation Rejected

The current Task019 UI attempt is **not accepted**. It is functionally still the old WireGuard-style layout with dark backgrounds and a header added on top. The owner-provided mockup is the actual target.

Do not treat this task as a styling pass. This requires a real composition/layout rewrite of the Windows manager UI while preserving the existing networking/backend behavior.

The owner must visually approve the implemented UI before this task can be merged.

## Branch

Continue on:

`codex/wirehush-ui-refresh`

Do not merge PR #17 until owner approval.

## Mandatory Visual Reference

On the Windows build/test VM:

`C:\TunnelMint-Test\UI-Mockup\UI-Mockup.png`

Open that image while implementing the UI. It is the primary design specification.

The finished main window should be recognizable as that design at a glance: same information hierarchy, same dark dashboard composition, same left connection rail, same selected-tunnel header, same card structure, same dedicated DNS area, and a real traffic graph driven from live WireGuard runtime counters.

Do not invent literal example values from the mockup. Bind every displayed value to real existing data.

---

# 1. STOP REUSING THE OLD VISIBLE WIREFRAME

The following current visible structure is specifically rejected:

- The visible top-level `Tunnels / Log / Settings` `walk.TabWidget` strip.
- A WireGuard-style `TableView` sitting in a white/light rectangle.
- A giant mostly-empty details panel.
- A centered `Import Tunnel(s) From File` button floating in empty space.
- Native white push buttons dropped onto dark surfaces.
- The old Interface/Peer presentation simply recolored.
- Detached branding with a large dead gap between the header and the actual UI.

Do not solve those issues by changing a few brush colors.

The primary shell must be rebuilt.

---

# 2. FILE / COMPONENT ARCHITECTURE

Use the existing Walk/Win32 stack. Do not introduce Electron, WebView, Qt, WPF, or another UI framework.

Refactor the UI into clear components. The exact filenames may vary slightly, but the ownership should look like this:

- `ui/managewindow.go`
  - Window shell only.
  - Branded header.
  - Main-content page switching.
  - No visible `Tunnels / Log / Settings` tab strip.

- `ui/tunnelspage.go`
  - Dashboard page composition.
  - Left connection rail.
  - Right selected-connection content host.
  - Add / Import / Delete actions.

- `ui/connectionrail.go` **new**
  - Custom dark connection list.
  - Do not use the current visible `walk.TableView` presentation.
  - Connection row rendering and selection.

- `ui/confview.go`
  - Refactor into the selected-tunnel dashboard/detail view.
  - Stop using old visible Interface/Peer group-box composition.

- `ui/trafficgraph.go` **new**
  - Live traffic sampler + graph widget.
  - Uses real `RuntimeConfig()` counters.

- `ui/theme.go`
  - Central palette, typography helpers, spacing constants, reusable dark-surface helpers.

- `ui/navbutton.go` or equivalent **new if needed**
  - Reusable owner-drawn dark navigation/action button if standard Walk buttons cannot be made to match the mockup.

Do not keep fighting standard Windows controls that remain bright white in dark mode. If a visible control cannot be made consistent with the approved design using Walk theming, replace that visible control with a small custom-drawn Walk widget rather than accepting a white rectangle.

Walk already provides `walk.NewCustomWidgetPixels(...)`; use it where owner-drawn presentation is required. Use `walk.PaintBuffered` and `SetInvalidatesOnResize(true)` for flicker-free custom drawing.

No new third-party rendering library is required.

---

# 3. WINDOW SHELL – EXACT COMPOSITION

## Window

Default client area target:

- approximately `1400 x 900` at 96 DPI.
- minimum approximately `1100 x 700`.
- resizeable.

Use DPI-aware layout sizing, not hardcoded device pixels. Values below are 96-DPI logical sizes.

Root background:

- `#0A1219` (`RGB 10,18,25`) or extremely close.

There must be **no large dead strip** between the header and body.

## Header

Height target: `88–96` logical px.

Background:

- `#0E1B26` / current `uiHeaderBrush` family.

Left side:

- 48–56 px WireHush logo.
- `WireHush` in Segoe UI Semibold around 22–24 pt-equivalent visual weight.
- subtitle directly underneath: `Private network control` in muted text.

Right side:

- `Log`
- `Settings`
- optional `Help` only if there is already a meaningful Help/About destination.

These should look like dark navigation actions, not white system buttons.

Remove the visible top-level TabWidget tab headers completely.

### Page switching

Do **not** expose `walk.TabWidget` headers as primary navigation.

Either:

1. replace the top-level TabWidget with page composites and show/hide the active page; or
2. retain an internal page container only if its tab strip is not rendered/visible.

Header `Log` and `Settings` actions switch the main content area to those pages. The tunnels/dashboard view is the default main page.

When leaving Log/Settings, provide a clear `Connections`/back navigation action in the header or page heading.

---

# 4. MAIN TUNNELS/DASHBOARD LAYOUT

Immediately below the header, split the content into:

- fixed left rail: `280–300` logical px.
- flexible right content area: all remaining width.

No visible old tab strip above this split.

## Left Rail

Background:

- `#0C1822`.

Padding:

- approximately 18–20 px around headings/actions.

Heading:

- `Connections`
- Semibold, roughly 16–18 pt visual weight.

### Connection list

The current `walk.TableView` presentation is not acceptable because it remains visually stock and, in the rejected build, renders as a giant white rectangle.

Create a purpose-built connection rail.

Each tunnel row:

- height: ~58–64 px.
- full-width hit target.
- 8 px-ish internal radius visually if practical; otherwise clean flat selected surface is acceptable.
- left state dot/icon.
- first line: tunnel name, semibold.
- second line: `Connected`, `Disconnected`, `Connecting`, etc., muted or green when connected.
- selected row background: dark cyan/blue-tinted surface, approximately `#0B5D7E` / `#0C5672` family.
- hover: slightly lighter than rail.
- disconnected dot: muted gray.
- connected dot: `uiHealthyColor`.

A three-dot menu on each row is optional; do not block the task on it. Existing context-menu actions may remain available through right click.

### Connection list behavior

Reuse `manager.IPCClientTunnels()` and the existing tunnel-change callbacks as the source of truth.

The custom rail must support:

- current tunnel selection.
- preserving selection on list refresh when possible.
- tunnel state updates without recreating unrelated UI state.
- selecting the first active tunnel on app raise/start where current behavior expects it.

Do not break import/delete/edit/toggle behavior merely to replace the TableView.

### Search

The mockup contains a search field. Implement it as a local tunnel-name filter if it can be done cleanly without backend changes.

Expected behavior:

- case-insensitive substring match on tunnel name.
- filtering only affects the visible rail.
- never deletes or mutates tunnel configuration.
- clearing restores the full list.

If the native edit control cannot be made acceptably dark, use a dark owner-drawn border/background around a Walk `LineEdit` and apply dark theme to its HWND.

### Bottom rail actions

Pinned visually to the bottom of the rail:

1. primary accent button: `+ Add Tunnel`
2. secondary dark button: `Import Tunnel(s)`
3. secondary/danger-muted button: `Delete`

`Delete` disabled when nothing is selected.

Buttons must not render as bright white rectangles.

Keep existing keyboard shortcuts/context actions if practical.

---

# 5. RIGHT CONTENT – EMPTY STATE

When no tunnel is selected, do **not** show a huge blank field with a tiny centered import button.

Show an intentional empty state centered in the right panel:

- WireHush mark, approximately 56–72 px.
- heading: `No Connection Selected`
- muted description: `Select a connection from the left or import a tunnel to get started.`
- two actions below:
  - primary `Import Tunnel(s)`
  - secondary `Add Tunnel`

Keep the surrounding canvas dark.

---

# 6. RIGHT CONTENT – SELECTED TUNNEL HEADER

When a tunnel is selected, the top of the right panel must mirror the mockup structure.

Use a dashboard header card/surface containing:

### Left

- tunnel name, large Semibold (~24 pt visual weight).
- optional edit pencil/icon next to name when admin.
- connection-state line below:
  - green dot + `Connected` when started.
  - muted state for stopped.
  - `Connecting…` / `Disconnecting…` as appropriate.

### Connection duration

Track a UI-observed connection start timestamp only when the transition to `TunnelStarted` is observed.

- If the UI observed the start: display `Connected for 1h 24m` style text and update it once per second/minute.
- If WireHush opens while an already-active tunnel is running and there is no reliable true start timestamp, **do not fabricate one**. Show only `Connected` until a trustworthy duration is known.
- Reset on stopped state.

### Right

Large action button:

- `Connect` when stopped.
- `Disconnect` when started.
- disabled/progress state while starting/stopping.

This replaces the old visible `Activate/Deactivate` button inside the Interface section.

Tie it to the existing `manager.Tunnel.Toggle()` path; do not create a second connection mechanism.

---

# 7. TOP SUMMARY CARDS

Directly below the selected-tunnel heading, show four compact cards in one responsive row where width permits:

1. `VPN IP (IPv4)`
2. `VPN IP (IPv6)`
3. `Endpoint`
4. `Latest Handshake`

Use real config/runtime values.

### VPN IPv4 / IPv6

Source:

`config.Interface.Addresses`

Split by address family:

- first/all IPv4 address(es) shown in IPv4 card.
- first/all IPv6 address(es) shown in IPv6 card.
- if absent show `Not Assigned`.

Do not label the entire address list as generic `Network` as the rejected attempt does.

### Endpoint

Source from the active peer(s):

`config.Peers[*].Endpoint`

For v1 summary card:

- show first configured non-empty endpoint.
- if multiple peers/endpoints exist, show first plus `+N more` or `Multiple Peers` rather than silently pretending there is only one.

### Latest Handshake

Use most recent non-empty `LastHandshakeTime` across peers from `RuntimeConfig()`.

Show friendly text using the existing handshake formatting.

---

# 8. DETAIL NAVIGATION ROW

Below summary cards, add a flat dark navigation strip matching the mockup:

- `Overview`
- `Network`
- `DNS`
- `Peer`
- `Allowed IPs`

Active item:

- cyan text/icon.
- 2–3 px cyan underline.

Inactive:

- muted text.

Do not use the stock white/light TabWidget renderer.

These must be functional page selectors, not decorative fake tabs.

Use simple show/hide composites under the row.

---

# 9. OVERVIEW PAGE – REQUIRED CARDS

The Overview page is the default selected-tunnel detail page.

Use a two-column card grid on wide windows. Collapse naturally to one column if minimum width requires it.

Target cards:

Top row:

- left: `Connection`
- right: `Traffic`

Bottom row:

- left: `DNS (Encrypted)` or `DNS` depending on configuration
- right: `Peer`

No old giant `Interface` and `Peer` group boxes.

## Connection card

Show:

- Status
- Tunnel Name
- Addresses / Interface addresses
- Uptime only when reliably available from UI-observed state
- optional listen port / MTU where meaningful

Do **not** display the private key.

Public interface key may be displayed only if useful; it is not required in Overview.

## DNS card

If `config.Interface.DNSOverHTTPS` is configured:

Heading:

`DNS (Encrypted)`

Rows:

- `Mode` → `DNS over HTTPS (DoH)`
- `Resolver` → exact configured URL/host, safely displayed
- `Address Family` → derive from the tunnel/address selection where meaningful (`IPv4`, `IPv6`, `Dual-Stack`) without claiming a runtime choice that is not known
- `Fallback` → `Disabled`

Do not copy the mockup's example fallback resolver text. WireHush is fail-closed after encrypted DNS activation. Do not imply `1.1.1.1` or any bootstrap resolver becomes an active fallback.

If plain DNS is configured, heading is simply `DNS` and show `Plain DNS` plus configured resolver addresses.

## Peer card

For first peer / single peer Overview summary:

- masked/truncated public key with copy action if practical.
- endpoint.
- Allowed IPs summary.
- Persistent Keepalive.
- latest handshake if space allows.

For multiple peers, clearly indicate count and let the `Peer` detail tab show all peers.

Never show preshared-key contents. Only show `Preshared Key: Enabled` when present.

---

# 10. TRAFFIC GRAPH – REQUIRED, REAL DATA, NO FAKE SERIES

The graph in the mockup is part of the accepted design. Implement it.

There is already enough runtime data. `conf.Peer` contains:

- `RxBytes conf.Bytes` (`uint64`)
- `TxBytes conf.Bytes` (`uint64`)

and `ConfView` already calls `tunnel.RuntimeConfig()` once per second while visible.

Do not add a new service/API for traffic statistics.

## New file

Create:

`ui/trafficgraph.go`

## Data model

Use structures equivalent to:

```go
type trafficSample struct {
    at    time.Time
    rxBps float64
    txBps float64
}

type trafficHistory struct {
    samples []trafficSample
    prevAt  time.Time
    prevRx  uint64
    prevTx  uint64
    ready   bool
}
```

Keep at most 300 one-second samples = five minutes.

## Sampling algorithm

On every existing one-second `RuntimeConfig()` refresh for a started tunnel:

1. Sum counters across **all peers**:

```go
var rx, tx uint64
for _, peer := range config.Peers {
    rx += uint64(peer.RxBytes)
    tx += uint64(peer.TxBytes)
}
```

2. On first sample:

- store `prevRx`, `prevTx`, `prevAt`.
- append a zero-rate sample or wait for the second tick.

3. On subsequent sample:

```go
elapsed := now.Sub(prevAt).Seconds()
```

If `elapsed <= 0`, skip.

If counters are lower than previous counters, treat it as a tunnel/runtime reset:

- do not underflow.
- use zero delta for that sample.

Otherwise:

```go
rxBitsPerSecond := float64(rx-prevRx) * 8 / elapsed
txBitsPerSecond := float64(tx-prevTx) * 8 / elapsed
```

4. Append sample.

5. Trim oldest entries until `len(samples) <= 300`.

6. Update previous counters/time.

7. Invalidate the graph widget on the UI thread.

When a different tunnel is selected, clear that graph history rather than showing the previous tunnel's data.

When the same tunnel disconnects:

- keep the existing history visible if desired, but stop adding live transfer deltas or append zero while stopped.
- when counters reset on reconnect, the reset guard above prevents spikes/underflow.

## Totals and current rate

Traffic card bottom area should show:

- Download current rate from latest `rxBps`.
- Upload current rate from latest `txBps`.
- cumulative received bytes = current summed `RxBytes`.
- cumulative sent bytes = current summed `TxBytes`.

Use existing byte-formatting semantics or a small human-readable formatter.

Rate units should automatically format as bps / Kbps / Mbps / Gbps.

## Graph renderer

Use a custom Walk widget:

```go
walk.NewCustomWidgetPixels(parent, 0, paintFunc)
```

Then:

```go
graph.SetPaintMode(walk.PaintBuffered)
graph.SetInvalidatesOnResize(true)
```

Target graph height: roughly 150–180 logical px inside the Traffic card.

Paint behavior:

1. Fill graph background with card background.
2. Draw 4 subtle horizontal grid lines.
3. Compute `maxRate` from visible RX/TX samples.
4. Choose Y-axis ceiling using a friendly rounded scale:
   - minimum ceiling around 1 Mbps when there is traffic below that, or sensible Kbps scale for truly low throughput.
   - otherwise round upward to 1 / 2 / 5 × 10^N style bounds.
5. Plot RX/download line in WireHush cyan/blue.
6. Plot TX/upload line in healthy green.
7. Oldest sample is at left; newest at right.
8. No smoothing that invents traffic. Straight line segments are enough.
9. If no samples exist, show a subtle centered `Waiting for traffic…` message rather than fake points.

The label `Last 5 Minutes` may be static for v1. Do not render a fake dropdown if it has no alternate time ranges.

Do not add a charting dependency.

---

# 11. NETWORK DETAIL PAGE

`Network` tab must use existing configuration/runtime data.

Show clean cards/rows for:

- IPv4 Address(es)
- IPv6 Address(es)
- Listen Port
- MTU
- Endpoint(s)
- routing/table state where already represented by config

Do not show fields that are empty merely to fill space.

No networking behavior changes are needed.

---

# 12. DNS DETAIL PAGE

Show:

- DNS Mode (`DNS over HTTPS (DoH)`, `Plain DNS`, or `Not Configured`)
- configured DoH URL when present
- configured plain DNS server(s) when present
- DNS search suffixes when present
- bootstrap policy summary for DoH: `Family-Aware Bootstrap`
- fallback: `Disabled`

Where available from existing runtime/error callbacks, show DNS error state honestly.

Do not claim a successful live DoH state merely because a URL is configured if the tunnel is stopped or starting.

No TLS behavior, redirect behavior, bootstrap behavior, or fail-closed behavior may be changed by this UI task.

---

# 13. PEER DETAIL PAGE

Render one dark peer card per peer.

Fields:

- Public Key (truncated visually; copy full public key action is okay)
- Endpoint
- Allowed IPs
- Latest Handshake
- Received
- Sent
- Persistent Keepalive
- `Preshared Key: Enabled` only when present; never display the key

Multiple peers must be represented correctly.

---

# 14. ALLOWED IPS PAGE

Render all Allowed IP prefixes, grouped by peer.

For each peer:

- short/truncated peer identifier.
- list its `AllowedIPs`.

This is read-only display. No routing changes.

---

# 15. VISUAL PRIMITIVES / DARK MODE

The rejected screenshot proves that merely calling `SetBackground()` on standard controls is insufficient.

Use these palette targets consistently:

- Canvas: `#0A1219`
- Header: `#0E1B26`
- Rail: `#0C1822`
- Card: `#111F2A`
- Elevated/hover card: approximately `#172B39`
- Border/divider: approximately `#243846`
- Accent: `#00B8E0`
- Bright accent text/icon: `#32CBEF`
- Primary text: `#E8F1F8`
- Muted text: `#A5B9C8`
- Healthy/connected: `#3ADD7E`
- Error/danger: muted red, approximately `#E06C75`, used sparingly

Spacing system:

- outer body padding: 16–18 px.
- card gap: 12–16 px.
- card internal padding: 16–18 px.
- row vertical gap: 8–10 px.

Fonts:

- Segoe UI / Segoe UI Variable where naturally available.
- normal body ~10–11 pt visual size.
- section headings ~12–14 pt Semibold.
- tunnel title ~20–24 pt Semibold.
- product title ~22–24 pt Semibold.

Do not use giant fonts.

## Native controls that remain white

If standard `PushButton`, `TabWidget`, `TableView`, or another native control stubbornly paints bright white/light despite dark theme:

**Do not accept it.**

Replace the visible presentation with owner-drawn/custom Walk widgets while continuing to call the same existing handlers underneath.

The target is a coherent dark product, not technically-dark surrounding panels with white Windows 7-era controls inside them.

---

# 16. REUSABLE DARK ACTION BUTTON

If standard PushButtons cannot match the mockup, implement one reusable owner-drawn action widget rather than hand-coding every button separately.

It needs:

- normal, hover, pressed, disabled states.
- optional icon.
- text.
- optional primary/accent style.
- keyboard activation when focused (`Space` / `Enter`).
- accessible name/role if Walk permits.
- Clicked-style callback/event.

Use it for:

- Add Tunnel
- Import Tunnel(s)
- Delete
- Connect/Disconnect
- Log
- Settings
- Overview/Network/DNS/Peer/Allowed IP navigation if those are owner drawn

Do not duplicate action/business logic. Each custom button should invoke the existing handlers.

---

# 17. SETTINGS / LOG / DIALOG DARKNESS

The main dashboard is the highest priority, but owned secondary UI must not look broken when opened.

Apply dark surfaces/text to:

- Settings page and bootstrap resolver list/buttons.
- Log page.
- Edit tunnel dialog where practical.
- About dialog.
- WireHush-owned warning/error dialogs where practical without replacing standard secure Windows behavior.

Native system file pickers may follow the OS theme and are not required to be custom-drawn.

---

# 18. IMPORTANT CURRENT-CODE BINDINGS

Do not invent new backend sources when the current code already has them.

Current data path in `ui/confview.go`:

- selected `manager.Tunnel`
- `tunnel.State()`
- `tunnel.RuntimeConfig()` once per second while started
- fallback to `tunnel.StoredConfig()`

Keep this refresh architecture.

Use RuntimeConfig for:

- `Peer.RxBytes`
- `Peer.TxBytes`
- `Peer.LastHandshakeTime`
- current peer endpoint/runtime values where populated

Use StoredConfig / existing config for:

- interface addresses
- configured DNS / DoH URL
- Allowed IPs
- Persistent Keepalive
- MTU/listen port
- configuration values not returned by runtime state

Merge/fallback exactly as current code already does; do not lose configured values merely because runtime data omits them.

---

# 19. FUNCTIONAL REGRESSION GUARDRAILS

The UI rewrite must preserve:

- tunnel creation.
- tunnel import.
- edit.
- delete.
- connect/disconnect.
- manager/service IPC.
- tunnel-change callbacks.
- plain DNS behavior.
- DoH behavior.
- fail-closed behavior.
- bootstrap family matching.
- Settings bootstrap resolver editing.
- tray behavior.
- official WireGuard coexistence.
- retained compatibility identifiers from `docs/wirehush-identity-migration.md`.

Do not touch networking code simply to complete this UI unless a concrete integration regression requires it.

---

# 20. DO NOT EXPOSE SECRETS

Never display:

- interface private key.
- preshared key contents.
- secret client IDs/tokens from owner test material beyond what is already intentionally configured/displayed.

Public keys are not secret, but the UI should normally truncate them for presentation and optionally provide a copy action.

---

# 21. ACCEPTANCE SCREEN STATES – MANDATORY OWNER REVIEW

Before asking for merge, build the actual amd64 app/MSI on the Windows build/test VM and provide owner-visible screenshots of the **real running application** for these states:

1. No tunnels / empty state.
2. Tunnel selected but disconnected.
3. Tunnel connected with live runtime data.
4. Connected DoH tunnel showing the DNS card.
5. Traffic graph after at least ~30 seconds of real transfer activity.
6. Settings page in dark mode.
7. Log page in dark mode.

The owner will compare these directly to:

`C:\TunnelMint-Test\UI-Mockup\UI-Mockup.png`

Task019 is not complete until the owner explicitly approves the visuals.

---

# 22. AUTOMATED / BUILD VALIDATION

After visual implementation is functionally complete:

- run existing focused Go tests.
- build `amd64\wirehush.exe` with production overlay.
- run `/update` smoke.
- build amd64 MSI.
- verify existing CI still passes.

Add unit tests for traffic sampling math:

- first sample produces no bogus spike.
- RX/TX delta calculations.
- elapsed-time division.
- counter reset/roll-back does not underflow.
- sample ring caps at 300.
- multi-peer counters are summed.

No network traffic should be generated by the graph itself.

---

# 23. DEFINITION OF DONE

This task is done only when all are true:

- The visible stock `Tunnels / Log / Settings` tab strip is gone.
- The white TableView rectangle is gone.
- White native action buttons are gone from the main dark dashboard.
- The main composition substantially matches the owner mockup.
- Left Connections rail is purpose-built and dark.
- Selected tunnel header includes real state and Connect/Disconnect.
- IPv4, IPv6, endpoint, and handshake summary cards are present.
- Overview / Network / DNS / Peer / Allowed IPs navigation is functional.
- Overview contains Connection, Traffic, DNS, and Peer cards.
- Traffic graph uses real `RuntimeConfig()` RX/TX counters and a five-minute rolling history.
- DoH display accurately reflects fail-closed behavior and does not claim plaintext fallback.
- Empty state is intentional and polished.
- Settings and Log are usable in dark mode.
- No secret key material is exposed.
- Existing tunnel/DNS functionality still works.
- Owner has visually approved the implementation.
- PR #17 remains unmerged until that approval.

If the running UI still looks like the rejected screenshot with a dark background painted around the old WireGuard controls, it is **not done**.
