# CLI Reference

The `spiderw` command can query the daemon, adapters, devices, stations, access
points, basic service sets, networks, and known networks through the same public
API used by library callers. It uses the system bus
by default, which is where real iwd runs, so the examples below need no bus flag.
The Go mock registers on the session bus, so pass `--session` when testing
against `iwdmock`.

Global flags may be placed anywhere in the command:

- `--session` uses the session D-Bus bus instead of the default system bus.
- `--json` emits JSON for commands with structured output.
- `--help` prints command help.

Print the CLI's own version (distinct from `daemon version`, which reports iwd's):

```bash
spiderw version
```

Daemon examples:

```bash
spiderw daemon info
spiderw daemon version
spiderw daemon state-dir
spiderw daemon net-conf
```

List adapters, or print a full snapshot for every adapter:

```bash
spiderw adapter list
spiderw adapter status
```

Use the adapter name or path from `adapter list` as the adapter reference:

```bash
spiderw adapter phy0 status
spiderw adapter phy0 powered
spiderw adapter phy0 powered true
spiderw adapter phy0 name
spiderw adapter phy0 model
spiderw adapter phy0 vendor
spiderw adapter phy0 supported-modes
spiderw adapter phy0 supports-mode station
spiderw adapter phy0 supports-station
spiderw adapter phy0 supports-ap
spiderw adapter phy0 supports-ad-hoc
spiderw adapter phy0 monitor powered
```

List devices, or print a full snapshot for every device:

```bash
spiderw device list
spiderw device status
```

Use the device name or path from `device list` as the device reference:

```bash
spiderw device wlan0 status
spiderw device wlan0 powered
spiderw device wlan0 powered false
spiderw device wlan0 mode
spiderw device wlan0 mode ap
spiderw device wlan0 name
spiderw device wlan0 address
spiderw device wlan0 adapter
spiderw device wlan0 monitor powered
spiderw device wlan0 monitor mode
```

Inspect, scan, and control stations (devices in station mode). `status` shows
`State`, `Scanning`, and the connected network/AP; `scan` triggers a scan
(waiting for it to finish, then listing results, unless `--no-wait`; `--timeout`
bounds the wait); `networks` lists the last scan's results by signal; `disconnect`
and `connect-hidden` control the connection (a secured hidden network prompts
for, or takes, a passphrase); `hidden-aps` lists hidden access points; `wsc`
joins an access point without a passphrase via WSC (WPS) push-button or PIN.
Networks render as their SSID and access points as their MAC (object paths are
still available with `--json`). A station is referenced by its device name (e.g.
`wlan0`) or object path:

```bash
spiderw station list
spiderw station status
spiderw station wlan0 status
spiderw station wlan0 scan --timeout=30s
spiderw station wlan0 networks
spiderw station wlan0 disconnect
spiderw station wlan0 connect-hidden MyHidden --passphrase=secret
spiderw station wlan0 hidden-aps
spiderw station wlan0 affinities
spiderw station wlan0 affinities set de:ad:be:ef:ca:fe   # a BSS MAC or object path
spiderw station wlan0 affinities clear
spiderw station wlan0 wsc push-button                    # press the AP's WPS button first
spiderw station wlan0 wsc pin                            # generates and prints a PIN to enter at the AP
spiderw station wlan0 monitor state                      # stream a property until Ctrl-C
spiderw station wlan0 monitor network                    # the connected network's SSID
spiderw station wlan0 monitor access-point               # the associated BSS - this is how a roam is watched
spiderw station wlan0 monitor affinities
spiderw station wlan0 monitor-signal -60 -70 -80         # RSSI thresholds, highest first
```

`monitor access-point` is the only way to observe a **roam**: the station moves
between access points of the same network, so the BSS changes while the state
stays `connected` and the network does not change at all. A reconnect looks
different - the BSS drops to `none` in between.

Inspect and control access points (devices in AP mode). `status` shows `Started`,
`Scanning`, and - while running - the hosted `SSID`, `Frequency`, and ciphers;
`start` brings up a PSK network, `start-profile` one from a stored profile, `stop`
tears it down. `scan` triggers a scan (waiting for it to finish, then listing
results, unless `--no-wait`; `--timeout` bounds the wait), and `networks` lists
the last scan's results by signal. An access point is referenced by its device
name (e.g. `wlan1`) or object path:

```bash
spiderw access-point list
spiderw access-point status
spiderw access-point wlan1 status
spiderw access-point wlan1 start MyAP s3cretpass
spiderw access-point wlan1 start-profile MyProfile
spiderw access-point wlan1 scan --timeout=30s
spiderw access-point wlan1 networks
spiderw access-point wlan1 stop
spiderw access-point wlan1 monitor started               # stream a property until Ctrl-C
spiderw access-point wlan1 monitor scanning
```

List basic service sets (BSSes), or print a full snapshot for every BSS. A
device usually sees many BSSes - one per access point/radio heard during a scan:

```bash
spiderw bss list
spiderw bss status
```

Use the address or path from `bss list` as the BSS reference:

```bash
spiderw bss 11:22:33:44:55:66 status
spiderw bss 11:22:33:44:55:66 address
```

List networks, or print a full snapshot for every network:

```bash
spiderw network list
spiderw network status
```

Use the SSID or path from `network list` as the network reference:

```bash
spiderw network OpenNet status
spiderw network OpenNet connect
spiderw network OpenNet connected
spiderw network OpenNet type
spiderw network OpenNet device
spiderw network OpenNet known-network
spiderw network OpenNet bsses
spiderw network OpenNet monitor connected
spiderw network OpenNet monitor known-network            # fires when the network is saved or forgotten
spiderw network OpenNet monitor bsses
```

`connect` joins open and already-known networks directly. For a not-yet-known
secured (PSK) network it registers a temporary credentials agent and supplies the
passphrase from `--passphrase`, `--passphrase-stdin`, or an interactive no-echo
prompt:

```bash
spiderw network MyWifi connect                       # prompts: Passphrase: ******
spiderw network MyWifi connect --passphrase=hunter2  # non-interactive
echo hunter2 | spiderw network MyWifi connect --passphrase-stdin
```

List known (saved) networks, or print a full snapshot for every one:

```bash
spiderw known-network list
spiderw known-network status
```

Use the name or path from `known-network list` as the reference:

```bash
spiderw known-network KnownNet status
spiderw known-network KnownNet type
spiderw known-network KnownNet hidden
spiderw known-network KnownNet last-connected
spiderw known-network KnownNet autoconnect
spiderw known-network KnownNet autoconnect false
spiderw known-network KnownNet forget
spiderw known-network KnownNet monitor autoconnect
spiderw known-network KnownNet monitor hidden
spiderw known-network KnownNet monitor last-connected    # fires on each successful connect
```

To target the Go mock instead of a real daemon, add `--session`:

```bash
spiderw --session daemon info
```

Monitor commands print the property's current value, then stream each change until
interrupted (Ctrl-C). `<resource> <ref> monitor --help` lists what that resource
can monitor. With `--json`, each change is emitted as its own object, one per line,
so the stream can be piped into a consumer.
