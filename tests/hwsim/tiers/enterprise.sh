#!/usr/bin/env bash
# Drive the 802.1x (EAP) credential-agent paths against a REAL iwd on
# mac80211_hwsim radios, with hostapd as the EAP authenticator.
#
# Every other hwsim tier uses iwd as BOTH the AP and the station, but iwd's AP
# mode is PSK-only - it cannot be an 802.1x authenticator. So this tier is shaped
# differently: hostapd runs the enterprise AP on one radio, iwd runs the station
# on another, and they meet over the shared hwsim medium. hostapd owns its radio
# exclusively, so that radio is moved into a network namespace where iwd never
# sees its netdev (hwsim radios still hear each other across namespaces - this is
# how iwd's own autotests pair iwd with hostapd).
#
# iwd learns the EAP method + the CA to trust from a .8021x provisioning file the
# tier writes; the CREDENTIAL is deliberately LEFT OUT of that file so iwd asks
# the agent for it, which is the whole point - that is the callback spiderw
# supplies from the command line. Three cases exercise the three agent callbacks:
#
#   1. PEAP, no identity in the file -> spiderw --username + --password
#      (iwd's RequestUserNameAndPassword)
#   2. PEAP, identity in the file    -> spiderw --password
#      (iwd's RequestUserPassword)
#   3. EAP-TLS, encrypted client key -> spiderw --private-key-passphrase
#      (iwd's RequestPrivateKeyPassphrase)
#
# plus a negative (wrong password must fail, then recover). The CLI cannot observe
# WHICH callback iwd chose - that attribution is covered by the mock integration
# tests; this tier proves each path connects end to end against the real daemon.
#
# FIRST-RUN UNKNOWNS (validate on the VM, see the README): the phy->netns move and
# iwd's clean release of it; the exact iwd 3.12 .8021x key names; hostapd coming
# up on the hwsim radio. Generous diagnostics below help pin any of these.
set -uo pipefail

SSID="${SSID:-spiderw-lab-eap}"
EAP_USER="${EAP_USER:-testuser}"
EAP_PASS="${EAP_PASS:-testpass}"
KEY_PASS="${KEY_PASS:-spiderw-key-pass}"
CHANNEL="${CHANNEL:-1}"
SCAN_TRIES="${SCAN_TRIES:-10}"
NETNS="${NETNS:-spiderw-ap}"

fail() { echo "[enterprise] FAIL: $*" >&2; exit 1; }
step() { echo; echo "== $* =="; }
require_cmd() { command -v "$1" >/dev/null 2>&1 || fail "missing required tool: $1"; }

require_cmd hostapd
require_cmd openssl
require_cmd iw
require_cmd ip

# iwd loads client private keys through the kernel's asymmetric-key subsystem,
# which needs the pkcs8_key_parser module (shipped in linux-modules-extra but not
# auto-loaded). Without it the EAP-TLS case fails with iwd's generic "Error
# loading client private key", no matter the key format.
modprobe pkcs8_key_parser 2>/dev/null \
    || echo "[enterprise] WARNING: could not load pkcs8_key_parser; EAP-TLS may fail"

WORK="$(mktemp -d /tmp/spiderw-eap.XXXXXX)" || fail "mktemp failed"
HOSTAPD_LOG="$WORK/hostapd.log"
HOSTAPD_PID=""

# --- teardown: kill hostapd + drop the namespace -----------------------------
# lib.sh's harness_down unloads mac80211_hwsim (which destroys the radios across
# namespaces) and stops iwd, but it does not know about hostapd or our netns, so
# clean those up here. Runs on any exit so a reused VM is left clean.
cleanup_enterprise() {
    local rc=$?
    [ "$HOSTAPD_PID" != "" ] && kill "$HOSTAPD_PID" 2>/dev/null || true
    ip netns pids "$NETNS" 2>/dev/null | xargs -r kill 2>/dev/null || true
    ip netns del "$NETNS" 2>/dev/null || true
    if [ "$rc" -ne 0 ] && [ -f "$HOSTAPD_LOG" ]; then
        echo; echo "===== hostapd log ====="; cat "$HOSTAPD_LOG"
    fi
    rm -rf "$WORK" 2>/dev/null || true
}
trap cleanup_enterprise EXIT

# --- certificates (ephemeral, self-signed; the image stays immutable) --------
step "generate test certificates"
gen_certs() {
    local subj_ca="/CN=spiderw-test-ca"
    openssl req -x509 -newkey rsa:2048 -nodes -days 2 \
        -keyout "$WORK/ca.key" -out "$WORK/ca.pem" -subj "$subj_ca" 2>/dev/null \
        || return 1
    # Server cert for the AP's TLS tunnel (PEAP) and EAP-TLS server side.
    openssl req -newkey rsa:2048 -nodes -keyout "$WORK/server.key" \
        -out "$WORK/server.csr" -subj "/CN=spiderw-eap-server" 2>/dev/null || return 1
    openssl x509 -req -in "$WORK/server.csr" -CA "$WORK/ca.pem" -CAkey "$WORK/ca.key" \
        -CAcreateserial -days 2 -out "$WORK/server.pem" 2>/dev/null || return 1
    # Client cert for EAP-TLS, with an ENCRYPTED key whose passphrase iwd asks the
    # agent for (spiderw supplies it via --private-key-passphrase). Generate the key
    # unencrypted, then re-wrap it as PKCS#8 / PBES2 AES-256-CBC explicitly: that is
    # the encrypted-key format iwd's ell TLS reliably decrypts, whereas the default
    # cipher of "openssl req -passout" is openssl-version-dependent and ell rejects
    # some of them (the connect then fails with "Error loading client private key").
    openssl req -newkey rsa:2048 -nodes -keyout "$WORK/client.key.unenc" \
        -out "$WORK/client.csr" -subj "/CN=spiderw-eap-client" 2>/dev/null || return 1
    openssl pkcs8 -topk8 -in "$WORK/client.key.unenc" -out "$WORK/client.key" \
        -passout "pass:$KEY_PASS" -v2 aes-256-cbc 2>/dev/null || return 1
    rm -f "$WORK/client.key.unenc"
    openssl x509 -req -in "$WORK/client.csr" -CA "$WORK/ca.pem" -CAkey "$WORK/ca.key" \
        -CAcreateserial -days 2 -out "$WORK/client.pem" 2>/dev/null || return 1
    # iwd refuses a world-readable client key; keep the secrets tight.
    chmod 600 "$WORK/client.key" "$WORK/server.key" "$WORK/ca.key"
}
gen_certs || fail "certificate generation failed"

# --- hostapd EAP authenticator config ----------------------------------------
# Integrated EAP server (no RADIUS). The eap_user file allows PEAP and TLS on the
# outer identity and holds the phase-2 MSCHAPV2 user for PEAP.
cat >"$WORK/hostapd.eap_user" <<EOF
*	PEAP,TLS
"$EAP_USER"	MSCHAPV2	"$EAP_PASS"	[2]
EOF

# --- pick the two radios -----------------------------------------------------
# iwd currently owns both (both station netdevs). Device 0 becomes the hostapd AP
# (moved out to the netns); device 1 stays under iwd as the station.
mapfile -t DEVICES < <(spiderw device list | cut -f1 | grep -v '^/' | awk 'NF')
[ "${#DEVICES[@]}" -ge 2 ] || fail "need >=2 named devices, saw: ${DEVICES[*]:-none}"
AP_DEV="${DEVICES[0]}"
STA_DEV="${DEVICES[1]}"
echo "[enterprise] AP(hostapd)=$AP_DEV  STA(iwd)=$STA_DEV  SSID=$SSID"

AP_WIPHY="$(iw dev "$AP_DEV" info 2>/dev/null | awk '/wiphy/ {print $2; exit}')"
[ "$AP_WIPHY" != "" ] || fail "could not resolve the wiphy for $AP_DEV"
AP_PHY="phy${AP_WIPHY}"

# hostapd.conf references the interface name it will own inside the netns (the
# netdev keeps its name when the phy moves).
cat >"$WORK/hostapd.conf" <<EOF
interface=$AP_DEV
driver=nl80211
ssid=$SSID
hw_mode=g
channel=$CHANNEL
wpa=2
wpa_key_mgmt=WPA-EAP
rsn_pairwise=CCMP
ieee8021x=1
eap_server=1
eap_user_file=$WORK/hostapd.eap_user
ca_cert=$WORK/ca.pem
server_cert=$WORK/server.pem
private_key=$WORK/server.key
EOF

# --- isolate the AP radio from iwd, then start hostapd -----------------------
step "move $AP_PHY into netns $NETNS (iwd releases it)"
ip netns add "$NETNS" || fail "ip netns add $NETNS failed"
# Modern iw (Ubuntu 24.04 ships iw 6.x) takes 'set netns name'. If a runner's iw
# is too old for the name form, this is the spot to fall back to a PID target.
iw phy "$AP_PHY" set netns name "$NETNS" \
    || fail "could not move $AP_PHY into $NETNS (check iw version / 'set netns name')"
ip netns exec "$NETNS" ip link set "$AP_DEV" up 2>/dev/null || true

step "start hostapd (EAP authenticator) in $NETNS"
ip netns exec "$NETNS" hostapd "$WORK/hostapd.conf" >"$HOSTAPD_LOG" 2>&1 &
HOSTAPD_PID=$!
up=""
for ((i = 0; i < 30; i++)); do
    if grep -q "AP-ENABLED" "$HOSTAPD_LOG" 2>/dev/null; then up=1; break; fi
    kill -0 "$HOSTAPD_PID" 2>/dev/null || fail "hostapd exited early (see log above)"
    sleep 0.3
done
[ "$up" = "1" ] || fail "hostapd never reached AP-ENABLED (see log above)"
echo "[enterprise] hostapd AP is up on $AP_DEV ($AP_PHY) channel $CHANNEL"

# --- station-side helpers (iwd, root netns) ----------------------------------
STA_PATH="$(spiderw device list | awk -F'\t' -v d="$STA_DEV" '$1 == d {print $2}')"
[ "$STA_PATH" != "" ] || fail "iwd no longer lists the station $STA_DEV after the phy move"

net_path() {
    spiderw network list \
      | awk -F'\t' -v ssid="$SSID" -v pfx="$STA_PATH/" \
            '$1 == ssid && index($2, pfx) == 1 { print $2; exit }'
}

scan_for_ssid() {
    local net=""
    for ((i = 0; i < SCAN_TRIES; i++)); do
        spiderw station "$STA_DEV" scan >/dev/null 2>&1
        net="$(net_path)"
        [ "$net" != "" ] && { echo "$net"; return 0; }
        sleep 1
    done
    return 1
}

# write_8021x <<EOF ... EOF  -> provisioning file for SSID, secret omitted.
write_8021x() {
    local f="/var/lib/iwd/${SSID}.8021x"
    cat >"$f" || fail "could not write $f"
    chmod 600 "$f"
}

# reset_known forgets any saved profile so the next case re-reads the .8021x fresh.
reset_known() {
    spiderw station "$STA_DEV" disconnect >/dev/null 2>&1 || true
    spiderw known-network "$SSID" forget >/dev/null 2>&1 || true
}

# connect_ok "<desc>" <flags...>   - expect a successful connect, then disconnect.
connect_ok() {
    local desc="$1"; shift
    local net
    net="$(scan_for_ssid)" || fail "$desc: station never saw $SSID after $SCAN_TRIES scans"
    spiderw network "$net" connect "$@" || fail "$desc: connect failed"
    local connected
    connected="$(spiderw network "$net" connected)"
    [ "$connected" = "true" ] || fail "$desc: connected=$connected (want true)"
    echo "[enterprise] $desc: connected"
    spiderw station "$STA_DEV" disconnect >/dev/null 2>&1 || true
}

# connect_fail "<desc>" <flags...> - expect the connect to FAIL (bad credential).
connect_fail() {
    local desc="$1"; shift
    local net
    net="$(scan_for_ssid)" || fail "$desc: station never saw $SSID after $SCAN_TRIES scans"
    if spiderw network "$net" connect "$@"; then
        fail "$desc: connect unexpectedly succeeded"
    fi
    local connected
    connected="$(spiderw network "$net" connected 2>/dev/null)"
    [ "$connected" = "true" ] && fail "$desc: left the station connected after a bad credential"
    echo "[enterprise] $desc: correctly rejected"
}

# ============================================================================
# Case 1 - PEAP, no identity in the file: spiderw supplies username + password
# (iwd RequestUserNameAndPassword).
# ============================================================================
step "case 1: PEAP, username+password via the agent"
reset_known
write_8021x <<EOF
[Security]
EAP-Method=PEAP
EAP-PEAP-CACert=$WORK/ca.pem
EAP-PEAP-Phase2-Method=MSCHAPV2
EOF
connect_ok "PEAP username+password" --username="$EAP_USER" --password="$EAP_PASS"

# ============================================================================
# Case 2 - PEAP with the inner (phase-2) identity in the file but no password:
# iwd already knows the username, so it asks the agent only for the password
# (iwd RequestUserPassword). The phase-2 identity - not the outer EAP-Identity -
# is the username iwd needs; with it set and the phase-2 password omitted, iwd
# requests just the password at connect time.
# ============================================================================
step "case 2: PEAP, password-only via the agent"
reset_known
write_8021x <<EOF
[Security]
EAP-Method=PEAP
EAP-Identity=anonymous
EAP-PEAP-CACert=$WORK/ca.pem
EAP-PEAP-Phase2-Method=MSCHAPV2
EAP-PEAP-Phase2-Identity=$EAP_USER
EOF
connect_ok "PEAP password-only" --password="$EAP_PASS"

# ============================================================================
# Case 3 - EAP-TLS, encrypted client key: spiderw supplies the key passphrase
# (iwd RequestPrivateKeyPassphrase).
# ============================================================================
step "case 3: EAP-TLS, private-key passphrase via the agent"
reset_known
write_8021x <<EOF
[Security]
EAP-Method=TLS
EAP-Identity=spiderw-eap-client
EAP-TLS-CACert=$WORK/ca.pem
EAP-TLS-ClientCert=$WORK/client.pem
EAP-TLS-ClientKey=$WORK/client.key
EOF
connect_ok "EAP-TLS private-key passphrase" --private-key-passphrase="$KEY_PASS"

# ============================================================================
# Case 4 - negative: a wrong password must FAIL (the mock is most likely to be
# more forgiving than iwd here), then a correct retry must RECOVER.
# ============================================================================
step "case 4: wrong password is rejected, correct one recovers"
reset_known
write_8021x <<EOF
[Security]
EAP-Method=PEAP
EAP-PEAP-CACert=$WORK/ca.pem
EAP-PEAP-Phase2-Method=MSCHAPV2
EOF
connect_fail "PEAP wrong password" --username="$EAP_USER" --password="wrong-$EAP_PASS"
connect_ok   "PEAP recovery"       --username="$EAP_USER" --password="$EAP_PASS"

echo
echo "[enterprise] PASS"
