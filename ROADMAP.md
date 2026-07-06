# Roadmap

This roadmap describes the current direction for spiderw. It is intentionally
high-level and may change as the public API and iwd support mature.

spiderw is developed in vertical slices: each new iwd object or feature should
move through the public API, internal core layer, D-Bus layer, CLI support where
useful, mock integration coverage, and race/stress testing where appropriate.

## Current Focus

- Keep the daemon, adapter, device, and basic service set vertical slices stable.
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
  hidden networks); and listing hidden APs (`GetHiddenAccessPoints`). Only the
  `SignalLevelAgent` remains as a planned follow-up.
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

- Improve CI for formatting, linting, unit tests, race tests, and integration tests.
- Decide what should be included in the first tagged release.

## Future iwd Coverage

Likely future vertical slices include:

- The station `SignalLevelAgent` (`RegisterSignalLevelAgent` /
  `UnregisterSignalLevelAgent`) — a second exported agent, building on the
  credentials-agent machinery.
- End-to-end test coverage for the 802.1x credential agent callbacks (a mock
  fixture that drives `RequestUserNameAndPassword` / `RequestUserPassword` /
  `RequestPrivateKeyPassphrase`), promoting them from experimental to tested.
- Provisioning new networks, including the `NetworkConfigurationAgent` and full
  802.1x/enterprise configuration (the credentials `Agent` is implemented, but
  *configuring* a brand-new enterprise network is not).
- Additional signal subscriptions for objects beyond adapters and networks.

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
- Keep race and stress tests focused on unique concurrency risks rather than
  duplicating ordinary unit coverage.
- Maintain benchmark coverage for important hot paths without optimizing before
  correctness is clear.
- Add a runnable `example/` application demonstrating an end-to-end flow (such as
  scanning and connecting) once the station and connection slices land. The
  per-method examples in `example_test.go` cover the public API in the meantime.

## Out of Scope for Now

The following are not current goals:

- Replacing iwd.
- Managing non-iwd Wi-Fi backends.
- Providing a full network-manager daemon.
- Exposing raw D-Bus values in the public API.
- Supporting every iwd interface before the core API design is stable.

## Release Direction

Before the first tagged release, spiderw should have:

- A stable public API for the daemon, adapter, device, and basic service set slices.
- Passing local and CI test suites.
- Clear README examples.
- Complete GoDoc for exported public API.
- A documented support and security policy.
- A small changelog or release note describing the initial supported surface.
