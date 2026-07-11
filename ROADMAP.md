# Roadmap

This roadmap describes the current direction for spiderw. It is intentionally
high-level and may change as the public API and iwd support mature.

spiderw is developed in vertical slices: each new iwd object or feature should
move through the public API, internal core layer, D-Bus layer, CLI support where
useful, mock integration coverage, and race/stress testing where appropriate.

## Current Focus

- Keep the implemented vertical slices (daemon, adapter, device, station,
  network, known-network, basic service set, and the credentials agent) stable.
- Improve documentation, contribution workflow, CI, and release hygiene.
- Preserve a small, strongly typed public API while avoiding D-Bus details in
  user code.
- Keep shared enum and error mappings centralized as new iwd object types are
  added.

## Implemented Vertical Slices

The following areas are currently implemented and tested end to end:

- Daemon information and status queries.
- Adapter discovery through daemon adapter references.
- Adapter construction by path or name reference through the client.
- Adapter properties such as name, model, vendor, powered state, and supported
  modes.
- Adapter powered state changes.
- Adapter property-change and powered-change subscriptions.
- Adapter unsubscribe behavior.
- Device discovery, construction, properties (name, address, powered state,
  mode, owning adapter), powered/mode changes, and property subscriptions.
- Basic service set (BSS) discovery, construction, and address lookup.
- Network discovery, construction, and properties (SSID, connected state,
  network type, owning device, optional known-network record, and basic service
  set membership via `ExtendedServiceSet`).
- Connecting to open and already-known networks via `Network.Connect`, with
  connected-state subscriptions.
- Credentials agent support (`net.connman.iwd.Agent` / `AgentManager`) via
  `Client.RegisterAgent`, enabling connection to not-yet-known secured (PSK)
  networks. The PSK passphrase path is tested end to end; the 802.1x credential
  callbacks (username/password and private-key passphrase) are wired through every
  layer but are not yet tested against the mock or validated on hardware
  (experimental). A not-yet-known secured network without a registered agent still
  reports a mapped `ErrNoAgent`.
- Station support (`net.connman.iwd.Station`): discovery and enumeration of
  station-mode devices; the `State`, `Scanning`, `ConnectedNetwork`, and
  experimental `ConnectedAccessPoint` / `Affinities` properties with
  state/scanning change subscriptions; scanning (`Scan`, `OrderedNetworks`);
  writing `Affinities` (`SetAffinities`); `Disconnect`; connecting to a hidden
  network (`ConnectHiddenNetwork`, driving the credentials agent for secured
  hidden networks); and listing hidden APs (`GetHiddenAccessPoints`).
- Signal-strength monitoring (`Station.MonitorSignalLevel`) via the
  `SignalLevelAgent` (`RegisterSignalLevelAgent` / `UnregisterSignalLevelAgent`):
  a second exported agent that reports RSSI threshold crossings, with a
  `station <name> monitor-signal` CLI command and end-to-end mock coverage.
- WSC / Wi-Fi Simple Configuration (`net.connman.iwd.SimpleConfiguration`) via
  `Station.SimpleConfiguration`: passphrase-free enrollment to an access point in
  PushButton (PBC) or PIN mode (`PushButton`, `GeneratePin`, `StartPin`,
  `Cancel`), with local PIN normalization/validation, a `station <name> wsc` CLI
  command, and end-to-end mock coverage. The binding is object-path-agnostic (its
  own `simple configuration` error resource), so it will back P2P-peer connection
  when P2P lands.
- Friendly-identifier resolution: `Properties` snapshots and `OrderedNetworks`
  resolve object paths to their human identifiers (network SSID, BSS address,
  device/adapter name) in one batched `GetManagedObjects`, while scalar accessors
  stay raw paths. Stations carry their device `Name`; the CLI references objects
  by name and accepts a BSS MAC for `affinities set` (with `affinities clear`),
  and `scan` takes a `--timeout`.
- Known-network discovery, construction, and properties (name, type, hidden,
  last-connected time, auto-connect), plus toggling auto-connect, forgetting, and
  auto-connect subscriptions.
- CLI coverage for daemon, adapter, device, station (`list` / `status` / `scan` /
  `networks` / `disconnect` / `connect-hidden` / `hidden-aps` / `affinities`),
  basic service set, network (including interactive secured connect with
  `--passphrase` / `--passphrase-stdin`), and known-network operations.
- Mock iwd integration tests, including signal firehose coverage.
- Shared adapter mode and network type parsing and formatting across layers.
- Structured error handling using generic error kinds plus resource metadata,
  including matchable sentinels for named iwd errors (for example `ErrNoAgent`,
  `ErrBusy`, `ErrFailed`), across the core and public API.

## Near-Term Goals

- Continue the vertical-slice cadence through the remaining iwd interfaces (see
  Future iwd Coverage), starting with the smaller station-adjacent slices
  (`StationDiagnostic`) before the larger operating-mode and P2P areas.
- Promote the 802.1x credential-agent callbacks from experimental to tested with
  a mock fixture that drives the username/password and private-key paths.
- Formalize real-hardware validation (Raspberry Pi against real iwd) as a release
  step, since mock and CI coverage cannot exercise driver-gated behavior such as
  `SetAffinities`.
- Keep the README, `examples/`, godoc, and release notes current as each slice
  lands.

## Future iwd Coverage

The slices below map the remaining iwd D-Bus surface. spiderw currently
implements the Daemon, Adapter, Device, Station, Network, KnownNetwork,
BasicServiceSet, SimpleConfiguration (WSC), the SignalLevelAgent, and credentials
`Agent` / `AgentManager` interfaces; the intent is to eventually cover the rest
of iwd. The areas are grouped by theme, not strictly ordered — priority is
decided slice by slice.

### Device operating modes

iwd exposes a different interface depending on a device's mode, and spiderw only
covers station mode today. `Device.SetMode` can already switch a device into
these modes, but the mode-specific interfaces are unimplemented:

- **Access Point** (`net.connman.iwd.AccessPoint`) — run an adapter as an AP:
  start/stop a hosted network (open or PSK), scan, and list connected clients,
  plus the companion `AccessPointDiagnostic` interface.
- **Ad-Hoc / IBSS** (`net.connman.iwd.AdHoc`) — start or join an open
  (`StartOpen`) or PSK-secured (`Start`) ad-hoc (IBSS) network and leave it
  (`Stop`), reading the `Started` state and the `ConnectedPeers` MAC list.

### Station-mode features not yet covered

- **Connection diagnostics** (`net.connman.iwd.StationDiagnostic`,
  `GetDiagnostics`) — read live link statistics (RSSI, TX/RX bitrate, frequency,
  security) for the connected station.

### Provisioning and enterprise

- **802.1x / enterprise** — end-to-end coverage for the enterprise credential
  callbacks (`RequestUserNameAndPassword` / `RequestUserPassword` /
  `RequestPrivateKeyPassphrase`) via a mock fixture that promotes them from
  experimental to tested, plus *configuring* a brand-new enterprise network
  (distinct from the already-implemented credentials `Agent`, which only supplies
  secrets for connecting).
- **Network configuration** (`net.connman.iwd.NetworkConfigurationAgent`) —
  provisioning new network profiles and IP/DHCP configuration.
- **DPP / Wi-Fi Easy Connect** (`net.connman.iwd.DeviceProvisioning` plus a
  `SharedCodeAgent`) — QR-code / shared-code provisioning, acting as either
  enrollee or configurator.

### Wi-Fi Direct (P2P)

- **P2P** (`net.connman.iwd.p2p.Device`, `p2p.Peer`, `p2p.ServiceManager`) —
  enable a device for peer-to-peer use and discover peers (`p2p.Device`:
  `Enabled` / `Name`, `RequestDiscovery` / `ReleaseDiscovery`, `GetPeers`, plus
  its own `RegisterSignalLevelAgent`), connect to and disconnect from a discovered
  `p2p.Peer` (with its connected interface / IP), and advertise Wi-Fi Display
  (Miracast) services through `p2p.ServiceManager`. A large, largely
  self-contained area; likely one of the last.

### Cross-cutting

- **Live object events** — react to `org.freedesktop.DBus.ObjectManager`
  `InterfacesAdded` / `InterfacesRemoved` so adapters, devices, networks, and
  BSSes appearing or disappearing surface as spiderw events, rather than only
  point-in-time enumeration.
- **Broader property subscriptions** — property-change subscriptions for object
  types that do not yet expose them.

### Testing and simulation

- **hwsim** (`net.connman.hwsim` — a sibling service to `net.connman.iwd`, backed
  by the `mac80211_hwsim` kernel module) — manage virtual radios
  (`net.connman.hwsim.Radio`: enumerate / `Destroy`) and traffic rules between
  them (`net.connman.hwsim.Rule`: `SignalStrength`, `Drop`, `Source` /
  `Destination`, `Priority`, ...). This serves a dual purpose: a client binding in
  its own right, and the foundation for a deterministic RF test tier — driving
  `Rule.SignalStrength` lets tests set exact per-frame RSSI and step it over time
  to exercise RSSI-dependent behavior (`SignalLevelAgent` band transitions,
  roaming) against real iwd without physical radios. Requires a privileged
  (kernel-module) test environment; see Testing and Tooling Goals.

Each new slice should follow the established pattern:

1. Add the low-level `internal/iwdbus` implementation.
2. Add shared iwd value parsing in `internal/iwdvalue` when the feature
   introduces a reusable enum or canonical string value.
3. Normalize behavior and errors in `internal/core`.
4. Use `internal/failure` kinds/resources for structured errors rather than
   adding resource-specific error categories.
5. Wire construction through `internal/connect` when needed.
6. Expose a small public API.
7. Add CLI support only when it is useful for manual testing or users.
8. Add unit tests, mock integration tests, and race/stress coverage where the
   feature introduces new concurrency behavior.

## Testing and Tooling Goals

- Keep tests deterministic and isolated from host iwd state.
- Continue using the Go mock for integration coverage that does not require root
  access or system iwd.
- Add a privileged, opt-in `mac80211_hwsim`-based integration tier (see the hwsim
  slice) for deterministically exercising RSSI- and radio-dependent behavior
  (signal-level bands, roaming) against real iwd, kept separate from the
  root-free Go-mock tier above.
- Keep race and stress tests focused on unique concurrency risks rather than
  duplicating ordinary unit coverage.
- Maintain benchmark coverage for important hot paths without optimizing before
  correctness is clear.
- Maintain the runnable programs under `examples/` (status, bring-up,
  scan-and-connect, connect-hidden, monitor, signal-monitor, wsc-push-button,
  wsc-pin, known-networks) alongside the per-method `Example*` functions in
  `example_test.go`, extending both as new slices land.

## Out of Scope for Now

The following are not current goals:

- Replacing iwd.
- Managing non-iwd Wi-Fi backends.
- Providing a full network-manager daemon.
- Exposing raw D-Bus values in the public API.
- Supporting every iwd interface before the core API design is stable.

## Release Direction

spiderw tags releases regularly and is pre-1.0: while the public API is still
maturing, a release that changes exported types or behavior bumps the minor
version (for example the ref-type bundle changes in v0.10.0), and patch releases
are reserved for fixes. A `1.0` release awaits the core slices proving stable
across several releases with no further breaking changes anticipated.

Each tagged release should have:

- Passing local and CI gates: build, vet, gofmt/goimports, golangci-lint,
  codespell, and the unit, race, stress, integration, and cross-compile suites.
- Real-hardware validation for anything driver-dependent, or an explicit note in
  the release notes when a feature could not be exercised on hardware.
- Complete godoc for any new or changed exported API, with the README and
  `examples/` updated to match.
- A signed, annotated tag whose notes summarize the change and call out any
  breaking changes.
- A maintained support and security policy.
