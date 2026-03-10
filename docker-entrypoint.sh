#!/bin/sh
set -eu

# Write runtime-config.js so the frontend picks up the API base URL at runtime
# rather than having it baked into the build.
API_BASE_URL="${AIMMOD_HUB_API_BASE_URL:-${VITE_API_BASE_URL:-https://api.aimmod.app}}"
ESCAPED=$(printf '%s' "$API_BASE_URL" | sed 's/\\/\\\\/g; s/"/\\"/g')

cat > "${AIMMOD_HUB_STATIC_DIR:-/app/web/dist}/runtime-config.js" <<EOF
window.__AIMMOD_HUB__ = window.__AIMMOD_HUB__ || {};
window.__AIMMOD_HUB__.apiBaseUrl = "$ESCAPED";
EOF

exec /app/aimmod-hub "$@"
