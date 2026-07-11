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
```

The mock accepts WSC enrollment and returns the PIN `12345670` for `wsc-pin`.
