#!/bin/sh
# Writes the runtime configuration the SPA reads on boot.
#
# Vite inlines VITE_* variables at build time, so without this the API URL
# would be frozen into the image and every environment would need its own
# build. Instead one image is built once and told where the API lives here,
# at container start.
#
# API_URL defaults to empty, meaning "same origin": nginx reverse-proxies
# /api to the API service, so the browser never makes a cross-origin request
# and CORS never enters the picture. Set API_URL only when the API is served
# from a different host than the dashboard.
set -eu

CONFIG_FILE=/usr/share/nginx/html/config.js

: "${API_URL:=}"

cat > "$CONFIG_FILE" <<EOF
window.__APP_CONFIG__ = {
  API_URL: "${API_URL}",
  VERSION: "${APP_VERSION:-dev}"
};
EOF

echo "[app-config] API_URL=\"${API_URL}\" (empty means same-origin via the /api proxy)"
