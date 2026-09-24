#!/bin/sh
# Dispatches between the API server, the GPIO pi agent, or both in one container.
set -eu

MODE="${1:-all}"
PIDS=""

terminate() {
	for p in $PIDS; do
		kill -TERM "$p" 2>/dev/null || true
	done
	wait
	exit 0
}
trap terminate TERM INT

require_config() {
	if [ ! -f "$PIAGENT_CONFIG" ]; then
		echo "piagent: config not found at $PIAGENT_CONFIG" >&2
		echo "mount one, e.g. -v /home/pi/piagent.yaml:$PIAGENT_CONFIG:ro" >&2
		exit 1
	fi
	if [ ! -e /dev/gpiomem ] && [ ! -e /dev/gpiochip0 ]; then
		echo "piagent: no GPIO device visible in this container" >&2
		echo "pass --device /dev/gpiomem --device /dev/gpiochip0" >&2
		exit 1
	fi
}

wait_for_server() {
	i=0
	while [ "$i" -lt 30 ]; do
		if wget -q -O /dev/null "http://127.0.0.1:${PORT}/api/v1/health" 2>/dev/null; then
			return 0
		fi
		i=$((i + 1))
		sleep 1
	done
	echo "piagent: server did not become healthy in 30s, starting anyway" >&2
}

case "$MODE" in
server)
	exec /app/server
	;;
piagent)
	require_config
	exec /app/piagent -config "$PIAGENT_CONFIG"
	;;
all)
	require_config
	/app/server &
	SERVER_PID="$!"
	PIDS="$SERVER_PID"
	wait_for_server
	/app/piagent -config "$PIAGENT_CONFIG" &
	AGENT_PID="$!"
	PIDS="$SERVER_PID $AGENT_PID"
	# Exit as soon as either process dies so Docker's restart policy takes over.
	# busybox ash has no `wait -n`, so poll instead.
	while kill -0 "$SERVER_PID" 2>/dev/null && kill -0 "$AGENT_PID" 2>/dev/null; do
		sleep 2
	done
	echo "entrypoint: a child process exited, shutting down" >&2
	terminate
	;;
*)
	exec "$@"
	;;
esac
