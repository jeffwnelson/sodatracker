#!/bin/sh
# Installs the Pi host config: persistent journal and the Wi-Fi watchdog.
# Run on the Pi from the repo clone: sudo deploy/pi/install.sh
set -eu
cd "$(dirname "$0")"

install -D -m 0644 journald-persistent.conf /etc/systemd/journald.conf.d/50-persistent.conf
systemctl restart systemd-journald
journalctl --flush

install -m 0755 wifi-watchdog.sh /usr/local/sbin/wifi-watchdog.sh
install -m 0644 wifi-watchdog.service wifi-watchdog.timer /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now wifi-watchdog.timer
systemctl list-timers wifi-watchdog.timer --no-pager
