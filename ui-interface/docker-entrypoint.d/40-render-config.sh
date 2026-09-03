#!/bin/sh
# Runs automatically on container start — nginx's official entrypoint
# executes every executable *.sh file in /docker-entrypoint.d/ before
# starting nginx itself. This renders config.template.js with the
# container's actual VITE_API_URL into the file the app fetches at
# runtime, overwriting the dev-default checked-in public/config.js that
# got baked into the image at build time.
set -eu

: "${VITE_API_URL:=http://localhost:8080}"
export VITE_API_URL

envsubst '${VITE_API_URL}' \
    < /usr/share/nginx/html/config.template.js \
    > /usr/share/nginx/html/config.js
