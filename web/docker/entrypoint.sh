#!/bin/sh
set -eu

API_BASE_URL="${AIMMOD_HUB_API_BASE_URL:-${VITE_API_BASE_URL:-https://api.aimmod.app}}"
RUNTIME_CONFIG="/usr/share/nginx/html/runtime-config.js"
ESCAPED_API_BASE_URL=$(printf '%s' "$API_BASE_URL" | sed 's/\\/\\\\/g; s/"/\\"/g')

cat > "$RUNTIME_CONFIG" <<EOF
window.__AIMMOD_HUB__ = window.__AIMMOD_HUB__ || {};
window.__AIMMOD_HUB__.apiBaseUrl = "$ESCAPED_API_BASE_URL";
EOF

exec "$@"
