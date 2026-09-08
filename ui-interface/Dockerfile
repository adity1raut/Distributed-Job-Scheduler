# syntax=docker/dockerfile:1.7

# ---------------------------------------------------------------------------
# Build stage
# ---------------------------------------------------------------------------
FROM node:26-alpine AS build

WORKDIR /app

# package-lock.json is copied with package.json so `npm ci` — which installs
# exactly the locked tree and fails if the two disagree — is cached
# independently of source changes.
COPY package.json package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY . .

# Deliberately NOT passing VITE_API_URL here. Vite inlines VITE_* at build
# time; baking the API URL in would mean rebuilding the image for every
# environment. The URL is injected at container start instead — see
# docker-entrypoint.sh and public/config.js.
ARG VERSION=dev
ENV VITE_APP_VERSION=${VERSION}

RUN npm run build

# ---------------------------------------------------------------------------
# Runtime stage
# ---------------------------------------------------------------------------
FROM nginx:1.31-alpine AS runtime

RUN apk add --no-cache wget

COPY --from=build /app/dist /usr/share/nginx/html

# Shipped as a template, not a finished conf, so the API's address can be set
# per environment via the API_UPSTREAM variable. The nginx image's own
# 20-envsubst-on-templates.sh renders /etc/nginx/templates/*.template into
# /etc/nginx/conf.d/ at startup.
COPY default.conf.template /etc/nginx/templates/default.conf.template
COPY security-headers.conf /etc/nginx/snippets/security-headers.conf
COPY docker-entrypoint.sh /docker-entrypoint.d/40-app-config.sh
RUN chmod +x /docker-entrypoint.d/40-app-config.sh

# NGINX_ENVSUBST_FILTER restricts substitution to exactly this one variable.
# Without it envsubst would also try to expand nginx's own runtime variables
# ($uri, $host, $remote_addr) and silently blank them out.
ENV API_UPSTREAM=api:8080 \
    NGINX_ENVSUBST_FILTER="^API_UPSTREAM$" \
    API_URL=""

# The nginx image's entrypoint runs everything in /docker-entrypoint.d before
# starting the server, so both the template render and the config injection
# above happen on every container start without replacing that entrypoint.
#
# Run unprivileged: nginx must be able to write its temp/pid paths, and the
# default config binds :80, which a non-root process cannot. Both are handled
# by switching the listen port to 8080 (see default.conf.template) and handing
# the runtime user ownership of the paths nginx writes to.
RUN sed -i 's|^pid .*|pid /tmp/nginx.pid;|' /etc/nginx/nginx.conf && \
    chown -R nginx:nginx /usr/share/nginx/html /var/cache/nginx /etc/nginx/conf.d

USER nginx

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 -O /dev/null http://127.0.0.1:8080/healthz || exit 1
