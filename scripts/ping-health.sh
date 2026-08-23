#!/usr/bin/env bash
# Keep a Render (or any HTTP) service awake by sending inbound traffic.
# Run under systemd so it restarts on crash and after reboot.
#
# PING_URL      required, e.g. https://your-api.onrender.com/api/health
# PING_INTERVAL seconds between pings (default 300 = 5 minutes)

set -u

URL="${PING_URL:-}"
INTERVAL="${PING_INTERVAL:-300}"

if [[ -z "$URL" ]]; then
  echo "PING_URL is not set" >&2
  exit 1
fi

echo "pinging ${URL} every ${INTERVAL}s"

while true; do
  if curl -fsS --max-time 30 --retry 2 --retry-delay 3 "$URL" >/dev/null; then
    echo "$(date -Is) ok"
  else
    echo "$(date -Is) fail" >&2
  fi
  sleep "$INTERVAL"
done
