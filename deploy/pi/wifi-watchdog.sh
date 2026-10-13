#!/bin/sh
# Wi-Fi watchdog for the SodaTracker Pi, run every 2 minutes by
# wifi-watchdog.timer. The Pi is on Wi-Fi only, and nothing else brings it
# back when the link or the brcmfmac driver wedges. After 5 failed checks
# (~10 min) it reloads the driver and reconnects, again every 15 checks while
# still down, and after 15 (~30 min) it reboots, at most once per 6 hours.
#
# Test without acting: DRY_RUN=1 WATCHDOG_URLS=https://example.invalid wifi-watchdog.sh
set -u

CON="${WATCHDOG_CON:-office-guest-wifi}"
URLS="${WATCHDOG_URLS:-https://api.github.com https://slack.com}"
FAILS_FILE="${WATCHDOG_FAILS_FILE:-/run/wifi-watchdog.fails}" # tmpfs: resets on boot
LAST_REBOOT_FILE=/var/lib/wifi-watchdog/last-reboot
RELOAD_AFTER=5
RELOAD_EVERY=15
REBOOT_AFTER=15
REBOOT_MIN_GAP=21600
DRY_RUN="${DRY_RUN:-0}"

run() {
	echo "action: $*"
	[ "$DRY_RUN" = 1 ] || "$@"
}

online() {
	# Any HTTP response counts: it proves DNS, routing and TLS all work.
	for url in $URLS; do
		curl -sS -m 15 -o /dev/null "$url" 2>/dev/null && return 0
	done
	return 1
}

if online; then
	if [ -s "$FAILS_FILE" ]; then
		echo "online again after $(cat "$FAILS_FILE") failed checks"
		rm -f "$FAILS_FILE"
	fi
	exit 0
fi

fails=$(( $(cat "$FAILS_FILE" 2>/dev/null || echo 0) + 1 ))
echo "$fails" > "$FAILS_FILE"
echo "offline: check $fails failed ($URLS)"

if [ "$fails" -ge "$REBOOT_AFTER" ]; then
	now=$(date +%s)
	last=$(cat "$LAST_REBOOT_FILE" 2>/dev/null || echo 0)
	if [ $(( now - last )) -ge "$REBOOT_MIN_GAP" ]; then
		echo "rebooting after $fails failed checks"
		if [ "$DRY_RUN" != 1 ]; then
			mkdir -p "$(dirname "$LAST_REBOOT_FILE")"
			echo "$now" > "$LAST_REBOOT_FILE"
			sync
		fi
		run systemctl reboot
		exit 0
	fi
fi

if [ "$fails" -ge "$RELOAD_AFTER" ] && [ $(( (fails - RELOAD_AFTER) % RELOAD_EVERY )) -eq 0 ]; then
	echo "reloading brcmfmac and reconnecting \"$CON\""
	run modprobe -r brcmfmac_cyw brcmfmac
	run modprobe brcmfmac
	[ "$DRY_RUN" = 1 ] || sleep 15
	run nmcli connection up "$CON"
fi
