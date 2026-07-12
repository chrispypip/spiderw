# Examples

Small, self-contained programs that show how to use the `spiderw` package for
end-to-end tasks. Each is a normal application: it talks to **real iwd on the
system bus by default**. Pass `-session` to run it against the bundled iwd mock
instead, so you can try everything without a wireless adapter.

For per-method snippets, see the runnable `Example*` functions in the package
docs (pkg.go.dev) — these programs are the full workflows those snippets add up
to.

The station examples (`scan-and-connect`, `monitor`) assume a device that is
already powered and in station mode. [`bring-up`](./bring-up) shows how to get
there from a cold device.

| Example | What it does | Changes state? |
|---|---|---|
| [`status`](./status) | Prints daemon info + adapters, devices, stations, known networks | no |
| [`bring-up`](./bring-up) | Powers on the adapter + device and switches it to station mode, then scans | yes (power + mode) |
| [`scan-and-connect`](./scan-and-connect) | Scans, lists networks by signal, optionally connects to one | only with `-ssid` |
| [`connect-hidden`](./connect-hidden) | Joins a non-broadcasting (hidden) network via a credentials agent | yes |
| [`monitor`](./monitor) | Watches a station and prints state/scanning changes until Ctrl-C | no |
| [`signal-monitor`](./signal-monitor) | Watches connected-network signal strength (RSSI band) until Ctrl-C | no |
| [`wsc-push-button`](./wsc-push-button) | Joins an access point via WSC (WPS) push-button | yes |
| [`wsc-pin`](./wsc-pin) | Joins an access point via WSC (WPS) PIN (generated or supplied) | yes |
| [`known-networks`](./known-networks) | Lists stored networks; can toggle autoconnect or forget one | only with `-forget`/`-autoconnect` |
| [`access-point-start`](./access-point-start) | Prints access-point status; can start a PSK AP (`Start`) or stop it | only with `-ssid`/`-stop` |
| [`access-point-start-profile`](./access-point-start-profile) | Prints access-point status; can start an AP from a stored profile (`StartProfile`) or stop it | only with `-profile`/`-stop` |

## Run against real iwd

```sh
go run ./examples/status
go run ./examples/scan-and-connect -ssid MyWiFi -passphrase 's3cret'
```

The connecting examples change your machine's network state (they can join or
disconnect networks), so they only act when you name a network explicitly.

## Run against the iwd mock

The mock is a pure-Go program that speaks iwd's D-Bus API on the **session** bus,
so nothing touches real hardware. Start it and the example under one session bus:

```sh
dbus-run-session -- bash -c '
  go run ./tools/test-mocks/iwdmock &   # export the mock on the session bus
  sleep 1
  go run ./examples/status -session
  kill %1
'
```

The mock exposes networks named `OpenNet`, `SecuredNet` (passphrase
`mock-secret-passphrase`), `KnownNetwork`, and a hidden `HiddenSecured`
(same passphrase). For example:

```sh
# ... inside the dbus-run-session block, with the mock running:
go run ./examples/scan-and-connect -session -ssid OpenNet
go run ./examples/scan-and-connect -session -ssid SecuredNet -passphrase mock-secret-passphrase
go run ./examples/connect-hidden   -session -ssid HiddenSecured -passphrase mock-secret-passphrase
go run ./examples/known-networks   -session
go run ./examples/wsc-push-button  -session
go run ./examples/wsc-pin          -session
go run ./examples/access-point-start         -session
go run ./examples/access-point-start-profile -session
```

The mock accepts WSC enrollment and returns the PIN `12345670` for `wsc-pin`. Its
AP-mode device (`wlan1`) hosts a running access point named `MockAP`, so the
access-point examples print its status by default. Because that AP is already
running, exercise the start paths by stopping it first — for example
`access-point-start -session -stop` then
`access-point-start -session -ssid Demo -passphrase demopass1`, or
`access-point-start-profile -session -stop` then
`access-point-start-profile -session -profile MockProfile` (the mock's one seeded
profile).

### Running an access point on real iwd (e.g. a Raspberry Pi)

`StartProfile(ssid)` tells iwd to bring up an access point from a provisioning
file it reads off disk, named `<ssid>.ap` in the AP profile directory (the daemon
`StateDirectory` — usually `/var/lib/iwd` — plus `/ap`). The file supplies the
security and (optionally) IP settings; the SSID comes from the file *name*. To
test on real hardware, write a minimal WPA2-PSK profile and start it:

```sh
# 1. Put the wireless device into AP mode (spiderw or iwctl):
spiderw device wlan0 mode ap          # or: iwctl device wlan0 set-property Mode ap

# 2. Write the profile. The file name IS the SSID, so PiTestAP.ap => SSID "PiTestAP".
#    (Confirm your state directory with `spiderw daemon`; /var/lib/iwd is the default.)
sudo mkdir -p /var/lib/iwd/ap
printf '[Security]\nPassphrase=%s\n' 'change-this-passphrase' | sudo tee /var/lib/iwd/ap/PiTestAP.ap >/dev/null
sudo chmod 600 /var/lib/iwd/ap/PiTestAP.ap   # iwd rejects world-readable secret files

# 3. Start it from the profile (this hosts a network — it CHANGES state):
go run ./examples/access-point-start-profile -profile PiTestAP
#    ...or the CLI:  spiderw access-point wlan0 start-profile PiTestAP
```

**Raspberry Pi / brcmfmac caveat — prefer `StartProfile`.** On the Pi's built-in
Wi-Fi (the `brcmfmac` FullMAC driver), iwd's inline `Start(ssid, psk)` — the
`access-point-start` example — often fails even on an idle radio with a generic
`net.connman.iwd.Failed` ("failed starting iwd AccessPoint"). The iwd log shows
the kernel refusing the AP with `START_AP failed: -52`, alongside
`Could not register frame watch … -114` warnings. This is the chip's partial AP
support, not spiderw or your configuration: it reproduces with iwd's own
`iwctl ap wlan0 start …`, and neither enabling network configuration nor
`iw reg set <country>` fixes it. `StartProfile` with a minimal profile takes a
slightly different path into the kernel and *does* work, so it is the reliable way
to run an AP on the built-in chip. For robust AP support (including the inline
`Start`), use a USB adapter with a mac80211 SoftMAC driver instead.

**Giving clients IP addresses.** A `Security`-only profile brings the AP up at
layer 2 — it authenticates, so `Started` is true and the SSID is visible — but
hands out no IP addresses. For iwd to run its DHCP server, enable network
configuration in `/etc/iwd/main.conf` and restart iwd
(`sudo systemctl restart iwd`):

```ini
[General]
EnableNetworkConfiguration=true

[IPv4]
APAddressPool=192.168.80.0/24
```

`APAddressPool` is the subnet iwd hands out to clients (it defaults to
`192.168.0.0/16`; set it explicitly to avoid clashing with your LAN). Then add an
`[IPv4]` block to the `.ap` profile to pin the AP's own address:

```ini
[IPv4]
Address=192.168.80.1
Netmask=255.255.255.0
Gateway=192.168.80.1
DNSList=192.168.80.1
```

Check whether network configuration is currently enabled with `spiderw daemon`
(the `NetworkConfigurationEnabled` field). Note this governs client DHCP only — it
does **not** affect whether the brcmfmac `Start` failure above occurs.
