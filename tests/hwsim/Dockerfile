# Real-iwd smoke image for the hwsim test tier.
#
# Multi-stage: build the spiderw CLI with the pinned Go toolchain, build a PINNED
# iwd from source, then drop both into an Ubuntu 24.04 runtime. The container runs
# its OWN system D-Bus and iwd (see entrypoint.sh); it only borrows the host's
# virtual radios, via `docker run --network host`. So iwd owns net.connman.iwd on
# the container's bus, isolated from the host, while managing the mac80211_hwsim
# phys the host created.
#
# We build iwd from the kernel.org release tarball rather than apt, because Ubuntu
# 24.04 ships iwd 2.14 while the target is 3.12. Pinning the version is the whole
# point of this tier: it must meet the SAME daemon the hardware runs, not whatever
# the base image happens to package. The iwd tarball bundles the exact ell it was
# released with (in ./ell), so there is no separate ell version to track.

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY . .
# Pure-Go (godbus), so a static binary runs on any base without glibc surprises.
RUN CGO_ENABLED=0 go build -trimpath -o /out/spiderw ./cmd/spiderw

# --- iwd from source --------------------------------------------------------
# The tarball bundles ell in ./ell; with no --enable-external-ell (which would
# want a system ell via pkg-config), iwd builds that bundled ell in-tree and links
# it statically, so the runtime needs no libell. iwd speaks D-Bus through ell (not
# libdbus), so there is no libdbus-dev here; readline is only for the iwctl client
# we get for free and use when debugging. --with-dbus-datadir sets where the D-Bus
# policy installs (iwd normally reads it from dbus-1's pkg-config, absent here);
# --disable-systemd-service avoids a second, identical pkg-config lookup we do not
# need, since the entrypoint starts iwd directly. --enable-hwsim also builds the
# `hwsim` medium-control tool (net.connman.hwsim) the roam tier drives.
FROM ubuntu:24.04 AS iwd
ARG IWD_VERSION=3.12
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      build-essential pkg-config libreadline-dev wget xz-utils ca-certificates \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /build
RUN wget -qO- "https://mirrors.edge.kernel.org/pub/linux/network/wireless/iwd-${IWD_VERSION}.tar.xz" | tar xJ \
 && cd "iwd-${IWD_VERSION}" \
 && ./configure \
      --prefix=/usr --localstatedir=/var --sysconfdir=/etc --libexecdir=/usr/libexec \
      --disable-manual-pages --disable-systemd-service --enable-hwsim \
      --with-dbus-datadir=/usr/share \
 && make -j"$(nproc)" \
 && make install DESTDIR=/iwd-root

# --- runtime ----------------------------------------------------------------
FROM ubuntu:24.04
# No iwd package: the daemon comes from the source build above. dbus is the system
# bus; iw/iproute2 for inspection; readline for the copied iwctl client.
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      iw iproute2 dbus procps ca-certificates libreadline8t64 \
 && rm -rf /var/lib/apt/lists/*

COPY --from=iwd /iwd-root/ /
COPY --from=build /out/spiderw /usr/local/bin/spiderw
COPY tests/hwsim/entrypoint.sh /usr/local/bin/entrypoint.sh
COPY tests/hwsim/smoke.sh /usr/local/bin/smoke.sh
COPY tests/hwsim/connect.sh /usr/local/bin/connect.sh
COPY tests/hwsim/roam.sh /usr/local/bin/roam.sh
RUN chmod +x /usr/local/bin/entrypoint.sh /usr/local/bin/smoke.sh \
      /usr/local/bin/connect.sh /usr/local/bin/roam.sh

# The entrypoint brings up dbus + iwd, then execs the command. Default: the smoke.
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["/usr/local/bin/smoke.sh"]
